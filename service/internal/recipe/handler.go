package recipe

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"github.com/s-frei/rezepte/service/internal/auth"
)

type listInput struct {
	Query string `query:"q" maxLength:"100" doc:"Full-text search query"`
	Tag   string `query:"tag" maxLength:"40" doc:"Restrict to recipes carrying this tag"`
	Page  int    `query:"page" minimum:"1" default:"1" doc:"1-based page number"`
	Limit int    `query:"limit" minimum:"1" maximum:"100" default:"24" doc:"Page size"`
}

type listOutput struct {
	Body Page
}

type getRecipeInput struct {
	ID string `path:"id"`
}

type getRecipeBySlugInput struct {
	Slug string `path:"slug"`
}

type recipeOutput struct {
	Body Recipe
}

type createRecipeInput struct {
	Body Input
}

type updateRecipeInput struct {
	ID   string `path:"id"`
	Body Input
}

type deleteRecipeInput struct {
	ID string `path:"id"`
}

type deleteRecipeOutput struct{}

// TagList is the response body of the list-tags operation.
type TagList struct {
	Items []TagCount `json:"items"`
}

type tagListOutput struct {
	Body TagList
}

// Register installs the recipe and tag operations onto api: listing
// (with search and tag filtering), lookup by id or slug, create, update,
// delete, and the tag list with usage counts. Every operation requires a
// session (Security: auth.SessionSecurity); the caller is responsible for
// installing the matching auth.Middleware via httpserver.WithAPIMiddleware
// before Register runs.
func Register(api huma.API, svc *Service) {
	huma.Register(api, huma.Operation{
		OperationID: "list-recipes",
		Method:      http.MethodGet,
		Path:        "/api/v1/recipes",
		Summary:     "List and search recipes",
		Tags:        []string{"recipes"},
		Security:    auth.SessionSecurity,
	}, func(ctx context.Context, in *listInput) (*listOutput, error) {
		page, err := svc.List(ctx, ListParams{
			Query: in.Query,
			Tag:   normalizeTagQuery(in.Tag),
			Page:  in.Page,
			Limit: in.Limit,
		})
		if err != nil {
			return nil, err
		}
		return &listOutput{Body: page}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-recipe",
		Method:      http.MethodGet,
		Path:        "/api/v1/recipes/{id}",
		Summary:     "Get a recipe by id",
		Tags:        []string{"recipes"},
		Security:    auth.SessionSecurity,
		Errors:      []int{404},
	}, func(ctx context.Context, in *getRecipeInput) (*recipeOutput, error) {
		r, err := svc.ByID(ctx, in.ID)
		if errors.Is(err, ErrNotFound) {
			return nil, huma.Error404NotFound("recipe not found")
		}
		if err != nil {
			return nil, err
		}
		return &recipeOutput{Body: r}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-recipe-by-slug",
		Method:      http.MethodGet,
		Path:        "/api/v1/recipes/by-slug/{slug}",
		Summary:     "Get a recipe by slug",
		Tags:        []string{"recipes"},
		Security:    auth.SessionSecurity,
		Errors:      []int{404},
	}, func(ctx context.Context, in *getRecipeBySlugInput) (*recipeOutput, error) {
		r, err := svc.BySlug(ctx, in.Slug)
		if errors.Is(err, ErrNotFound) {
			return nil, huma.Error404NotFound("recipe not found")
		}
		if err != nil {
			return nil, err
		}
		return &recipeOutput{Body: r}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "create-recipe",
		Method:        http.MethodPost,
		Path:          "/api/v1/recipes",
		Summary:       "Create a recipe",
		Tags:          []string{"recipes"},
		Security:      auth.SessionSecurity,
		DefaultStatus: http.StatusCreated,
	}, func(ctx context.Context, in *createRecipeInput) (*recipeOutput, error) {
		u, ok := auth.UserFrom(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("authentication required")
		}
		r, err := svc.Create(ctx, u.ID, in.Body)
		if err != nil {
			return nil, err
		}
		return &recipeOutput{Body: r}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-recipe",
		Method:      http.MethodPut,
		Path:        "/api/v1/recipes/{id}",
		Summary:     "Update a recipe",
		Tags:        []string{"recipes"},
		Security:    auth.SessionSecurity,
		Errors:      []int{404},
	}, func(ctx context.Context, in *updateRecipeInput) (*recipeOutput, error) {
		r, err := svc.Update(ctx, in.ID, in.Body)
		if errors.Is(err, ErrNotFound) {
			return nil, huma.Error404NotFound("recipe not found")
		}
		if err != nil {
			return nil, err
		}
		return &recipeOutput{Body: r}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "delete-recipe",
		Method:        http.MethodDelete,
		Path:          "/api/v1/recipes/{id}",
		Summary:       "Delete a recipe",
		Tags:          []string{"recipes"},
		Security:      auth.SessionSecurity,
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{404},
	}, func(ctx context.Context, in *deleteRecipeInput) (*deleteRecipeOutput, error) {
		if err := svc.Delete(ctx, in.ID); err != nil {
			if errors.Is(err, ErrNotFound) {
				return nil, huma.Error404NotFound("recipe not found")
			}
			return nil, err
		}
		return &deleteRecipeOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list-tags",
		Method:      http.MethodGet,
		Path:        "/api/v1/tags",
		Summary:     "List tags with usage counts",
		Tags:        []string{"tags"},
		Security:    auth.SessionSecurity,
	}, func(ctx context.Context, _ *struct{}) (*tagListOutput, error) {
		tags, err := svc.Tags(ctx)
		if err != nil {
			return nil, err
		}
		if tags == nil {
			tags = []TagCount{}
		}
		return &tagListOutput{Body: TagList{Items: tags}}, nil
	})
}

// normalizeTagQuery trims and lower-cases the tag query parameter so
// "?tag=Fleisch" or "?tag= fleisch " match the lower-cased tag names
// Service.List and NormalizeTags store and expect.
func normalizeTagQuery(tag string) string {
	return strings.ToLower(strings.TrimSpace(tag))
}
