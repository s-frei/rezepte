package recipeimport

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"reflect"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/recipe"
)

type draftInput struct {
	Body struct {
		URL  string `json:"url,omitempty" required:"false" maxLength:"2000" doc:"Address of a recipe page; give this or text"`
		Text string `json:"text,omitempty" required:"false" maxLength:"65536" doc:"A recipe copied as plain text; give this or url"`
	}
}

type photoRef struct {
	Href string `json:"href" doc:"Fetch within two minutes; answers the photo or 404"`
}

// orNull is an object or null. huma refuses nullable on an object reference,
// so Schema spells the choice out as a oneOf.
type orNull[T any] struct{ V *T }

// MarshalJSON writes null for no value, else the object.
func (n orNull[T]) MarshalJSON() ([]byte, error) { return json.Marshal(n.V) }

// Schema documents the field as the object or null.
func (orNull[T]) Schema(r huma.Registry) *huma.Schema {
	return &huma.Schema{OneOf: []*huma.Schema{r.Schema(reflect.TypeFor[T](), true, ""), {Type: "null"}}}
}

type draftOutput struct {
	Body struct {
		Recipe        recipe.Input               `json:"recipe" doc:"Valid for POST /api/v1/recipes once a title is set"`
		Review        []DraftReview              `json:"review" nullable:"false" doc:"Ingredients the parser was unsure about, by index into recipe.ingredientGroups"`
		SuggestedTags []string                   `json:"suggestedTags" nullable:"false" doc:"Keywords that are not tags in this collection yet"`
		Photo         orNull[photoRef]           `json:"photo" doc:"Null when the page has no photo"`
		Duplicate     orNull[recipe.SourceMatch] `json:"duplicate" doc:"A recipe already imported from this page; null when there is none"`
		Truncated     bool                       `json:"truncated" doc:"Something was cut to the recipe limits"`
	}
}

type photoInput struct {
	Src   string `query:"src" required:"true"`
	Token string `query:"token" required:"true"`
}

// Register adds the draft operations. Call after recipe.Register.
func Register(api huma.API, svc *Service) {
	huma.Register(api, huma.Operation{
		OperationID: "create-recipe-draft",
		Method:      http.MethodPost,
		Path:        "/api/v1/recipe-drafts",
		Summary:     "Read a recipe page or pasted text into an unsaved draft",
		Tags:        []string{"recipes"},
		Security:    auth.Protected(auth.ScopeRecipesWrite),
		Errors:      []int{401, 403, 422},
	}, func(ctx context.Context, in *draftInput) (*draftOutput, error) {
		hasURL, hasText := in.Body.URL != "", in.Body.Text != ""
		if hasURL == hasText {
			return nil, huma.Error422UnprocessableEntity("validation failed",
				&huma.ErrorDetail{Location: "body", Message: "give exactly one of url and text"})
		}
		var res Result
		var err error
		loc := "body.text"
		if hasURL {
			loc = "body.url"
			res, err = svc.FromURL(ctx, in.Body.URL)
		} else {
			res, err = svc.FromText(ctx, in.Body.Text)
		}
		switch {
		case errors.Is(err, ErrUnreachable):
			slog.InfoContext(ctx, "recipe import fetch failed", "err", err)
			return nil, huma.Error422UnprocessableEntity("import failed", &huma.ErrorDetail{Location: loc, Message: "unreachable"})
		case errors.Is(err, ErrNoRecipe), errors.Is(err, ErrNoRecipeInText):
			return nil, huma.Error422UnprocessableEntity("import failed", &huma.ErrorDetail{Location: loc, Message: err.Error()})
		case err != nil:
			return nil, err
		}
		out := &draftOutput{}
		out.Body.Recipe = res.Draft.Recipe
		out.Body.Review = append([]DraftReview{}, res.Draft.Review...)
		out.Body.SuggestedTags = append([]string{}, res.Draft.SuggestedTags...)
		out.Body.Duplicate = orNull[recipe.SourceMatch]{res.Duplicate}
		out.Body.Truncated = res.Draft.Truncated
		if res.Draft.PhotoURL != "" {
			out.Body.Photo = orNull[photoRef]{&photoRef{Href: svc.PhotoHref(res.Draft.PhotoURL)}}
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-recipe-draft-photo",
		Method:      http.MethodGet,
		Path:        "/api/v1/recipe-drafts/photo",
		Summary:     "The photo a draft found, while its token lasts",
		Tags:        []string{"recipes"},
		Security:    auth.Protected(auth.ScopeRecipesWrite),
		Errors:      []int{401, 403, 404},
		Responses: map[string]*huma.Response{"200": {
			Description: "The photo as the source served it",
			Content: map[string]*huma.MediaType{
				"image/jpeg": {Schema: &huma.Schema{Type: "string", Format: "binary"}},
				"image/png":  {Schema: &huma.Schema{Type: "string", Format: "binary"}},
				"image/webp": {Schema: &huma.Schema{Type: "string", Format: "binary"}},
			},
		}},
	}, func(ctx context.Context, in *photoInput) (*huma.StreamResponse, error) {
		body, ct, err := svc.Photo(ctx, in.Src, in.Token)
		if err != nil {
			slog.InfoContext(ctx, "recipe import photo refused", "err", err)
			return nil, huma.Error404NotFound("not found")
		}
		return &huma.StreamResponse{Body: func(hctx huma.Context) {
			_, w := humago.Unwrap(hctx)
			w.Header().Set("Content-Type", ct)
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			_, _ = w.Write(body)
		}}, nil
	})
}
