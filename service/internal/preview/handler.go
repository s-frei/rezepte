package preview

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/httpserver"
	"github.com/s-frei/rezepte/service/internal/image"
)

type shareLinkInput struct {
	ID string `path:"id" doc:"Recipe id"`
}

type shareLinkOutput struct {
	Body Link
}

type coverInput struct {
	RecipeID string `path:"recipeId"`
	ImageID  string `path:"imageId"`
	Share    string `query:"share" doc:"The token of the address create-share-link returned"`
}

// Register installs create-share-link for anyone who may read recipes -
// sharing hands out nothing the household setting does not already allow -
// and the cover route the address's link preview points at.
// create-share-link is a POST because every call signs a new token, and
// because a GET under /recipes/{id}/ would collide with GET
// /recipes/by-slug/{slug} in the mux.
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

	huma.Register(api, huma.Operation{
		OperationID: "get-link-preview-cover",
		Method:      http.MethodGet,
		Path:        "/link-preview/{recipeId}/{imageId}",
		Summary:     "Read the cover a link preview shows",
		Description: "No session needed: the crawler building a link preview has none. Answers the thumb of the recipe's cover while link previews are on and the token from create-share-link is valid. A photo that is not the cover, an invalid token and a recipe that does not exist answer 404 alike.",
		Tags:        []string{"public"},
		Errors:      []int{404},
		Responses:   image.JPEGResponses,
	}, func(ctx context.Context, in *coverInput) (*huma.StreamResponse, error) {
		if !svc.showsCover(ctx, in.RecipeID, in.ImageID, in.Share) {
			return nil, httpserver.PublicNotFound("not found")
		}
		f, err := svc.images.Open(in.RecipeID, in.ImageID, "thumb")
		if err != nil {
			return nil, httpserver.PublicNotFound("not found")
		}
		// A crawler fetches it once; nobody else should keep it around.
		return image.JPEG(f, "private, max-age=3600"), nil
	})
}
