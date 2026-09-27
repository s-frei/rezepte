package preview

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/s-frei/rezepte/service/internal/auth"
)

type shareLinkInput struct {
	ID string `path:"id" doc:"Recipe id"`
}

type shareLinkOutput struct {
	Body Link
}

// Register installs create-share-link for anyone who may read recipes:
// sharing hands out nothing the household setting does not already allow.
// It is a POST because every call signs a new token, and because a GET
// under /recipes/{id}/ would collide with GET /recipes/by-slug/{slug} in
// the mux.
func Register(api huma.API, svc *Service) {
	huma.Register(api, huma.Operation{
		OperationID: "create-share-link",
		Method:      http.MethodPost,
		Path:        "/api/v1/recipes/{id}/share-link",
		Summary:     "Create the address to share a recipe by",
		Description: "The recipe's path. While the owner has link previews on, it carries a token that shows the recipe's title, description and cover in link previews for linkPreviewMinutes; otherwise it is the plain path.",
		Tags:        []string{"recipes"},
		Security:    auth.Protected(auth.ScopeRecipesRead),
		Errors:      []int{404},
	}, func(ctx context.Context, in *shareLinkInput) (*shareLinkOutput, error) {
		link, err := svc.ShareLink(ctx, in.ID)
		if errors.Is(err, ErrNotFound) {
			return nil, huma.Error404NotFound("recipe not found")
		}
		if err != nil {
			return nil, err
		}
		return &shareLinkOutput{Body: link}, nil
	})
}
