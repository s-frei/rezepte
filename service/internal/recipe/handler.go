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
	Query          string   `query:"q" maxLength:"100" doc:"Full-text search query"`
	Tags           []string `query:"tags" doc:"Restrict to recipes carrying all of these tags"`
	MaxMinutes     int      `query:"maxMinutes" minimum:"0" maximum:"1440" doc:"Only recipes whose total time is at most this many minutes"`
	FavouritesOnly bool     `query:"favourites" doc:"Restrict to the caller's own favourites"`
	Sort           string   `query:"sort" enum:"updated,created,title" default:"updated" doc:"Result order"`
	Page           int      `query:"page" minimum:"1" default:"1" doc:"1-based page number"`
	Limit          int      `query:"limit" minimum:"1" maximum:"100" default:"24" doc:"Page size"`
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

type favouriteInput struct {
	ID string `path:"id"`
}

type favouriteOutput struct{}

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
		// UserID is filled from the context on every request, not only when
		// a favourites filter is set - otherwise Card.Favourite would be
		// wrong on every card for every caller. An unauthenticated context
		// (not possible here, since this operation requires a session, but
		// defended anyway) leaves it "", which disables the favourite
		// lookup rather than matching rows - see ListParams.UserID.
		u, _ := auth.UserFrom(ctx)
		page, err := svc.List(ctx, ListParams{
			Query:          in.Query,
			Tags:           NormalizeTagQuery(in.Tags),
			MaxMinutes:     in.MaxMinutes,
			FavouritesOnly: in.FavouritesOnly,
			Sort:           in.Sort,
			Page:           in.Page,
			Limit:          in.Limit,
			UserID:         u.ID,
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
		if err := fillFavourite(ctx, svc, &r); err != nil {
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
		if err := fillFavourite(ctx, svc, &r); err != nil {
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
		OperationID:   "set-favourite",
		Method:        http.MethodPut,
		Path:          "/api/v1/recipes/{id}/favourite",
		Summary:       "Mark a recipe as a favourite",
		Tags:          []string{"recipes"},
		Security:      auth.SessionSecurity,
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{404},
	}, func(ctx context.Context, in *favouriteInput) (*favouriteOutput, error) {
		u, ok := auth.UserFrom(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("authentication required")
		}
		if err := svc.SetFavourite(ctx, u.ID, in.ID, true); err != nil {
			if errors.Is(err, ErrNotFound) {
				return nil, huma.Error404NotFound("recipe not found")
			}
			return nil, err
		}
		return &favouriteOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "delete-favourite",
		Method:        http.MethodDelete,
		Path:          "/api/v1/recipes/{id}/favourite",
		Summary:       "Clear a recipe's favourite mark",
		Tags:          []string{"recipes"},
		Security:      auth.SessionSecurity,
		DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *favouriteInput) (*favouriteOutput, error) {
		u, ok := auth.UserFrom(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("authentication required")
		}
		if err := svc.SetFavourite(ctx, u.ID, in.ID, false); err != nil {
			return nil, err
		}
		return &favouriteOutput{}, nil
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

// fillFavourite sets r.Favourite from the caller's own favourite state,
// deriving the caller strictly from ctx (auth.UserFrom), never from r or
// any path/query/body value - a caller must not be able to name whose
// favourites are being read. An unauthenticated context (defended against
// even though both call sites require a session) leaves r.Favourite false,
// the same "empty user id disables the lookup" rule IsFavourite applies.
func fillFavourite(ctx context.Context, svc *Service, r *Recipe) error {
	u, ok := auth.UserFrom(ctx)
	if !ok {
		return nil
	}
	fav, err := svc.IsFavourite(ctx, u.ID, r.ID)
	if err != nil {
		return err
	}
	r.Favourite = fav
	return nil
}

// maxTagFilters caps the number of tags NormalizeTagQuery keeps - the same
// ceiling Input.Tags carries as maxItems:"20".
const maxTagFilters = 20

// NormalizeTagQuery trims and lower-cases each tag query parameter so
// "?tags=Fleisch" or "?tags= fleisch " match the lower-cased tag names
// Service.List and NormalizeTags store and expect. Blanks are dropped,
// duplicates collapsed, and the result capped at maxTagFilters - a caller
// passing 500 tags would otherwise build a 500-way subquery.
func NormalizeTagQuery(tags []string) []string {
	out := make([]string, 0, len(tags))
	seen := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		name := strings.ToLower(strings.TrimSpace(tag))
		if name == "" {
			continue
		}
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
		if len(out) == maxTagFilters {
			break
		}
	}
	return out
}
