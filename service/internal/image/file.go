package image

import (
	"context"
	"net/http"
	"os"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"

	"github.com/s-frei/rezepte/service/internal/auth"
)

// immutableCache suits the files: an image id is never reused and a file
// never changes after it is written, so a browser may cache it for a year.
// "private" keeps shared caches out - the route is behind a session.
const immutableCache = "private, max-age=31536000, immutable"

// JPEGResponses is the 200 of every operation that answers with a JPEG. The
// operation returns a StreamResponse, which carries no schema of its own.
var JPEGResponses = map[string]*huma.Response{"200": {
	Description: "The JPEG, with Range and conditional-request support",
	Content:     map[string]*huma.MediaType{"image/jpeg": {Schema: &huma.Schema{Type: "string", Format: "binary"}}},
}}

type fileInput struct {
	RecipeID string `path:"recipeId"`
	ImageID  string `path:"imageId"`
	File     string `path:"file" doc:"thumb.jpg, detail.jpg or original.jpg; anything else is a 404"`
}

// registerFile adds GET /images/{recipeId}/{imageId}/{file}. It lives
// outside /api/v1 because the SPA puts these addresses into <img> tags.
func registerFile(api huma.API, svc *Service) {
	huma.Register(api, huma.Operation{
		OperationID: "get-image-file",
		Method:      http.MethodGet,
		Path:        "/images/{recipeId}/{imageId}/{file}",
		Summary:     "Read an image in one of its rendered sizes",
		Tags:        []string{"images"},
		Security:    auth.Protected(auth.ScopeRecipesRead),
		Errors:      []int{403, 404},
		Responses:   JPEGResponses,
	}, func(_ context.Context, in *fileInput) (*huma.StreamResponse, error) {
		variant, ok := strings.CutSuffix(in.File, ".jpg")
		if !ok {
			return nil, huma.Error404NotFound("image not found")
		}
		f, err := svc.Open(in.RecipeID, in.ImageID, variant)
		if err != nil {
			return nil, huma.Error404NotFound("image not found")
		}
		return JPEG(f, immutableCache), nil
	})
}

// JPEG answers with the open image file f and closes it, with Range and
// conditional-request support from http.ServeContent. cacheControl is set
// unless empty.
func JPEG(f *os.File, cacheControl string) *huma.StreamResponse {
	return &huma.StreamResponse{Body: func(ctx huma.Context) {
		defer f.Close()
		r, w := humago.Unwrap(ctx)
		info, err := f.Stat()
		if err != nil {
			http.Error(w, "image unreadable", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "image/jpeg")
		if cacheControl != "" {
			w.Header().Set("Cache-Control", cacheControl)
		}
		http.ServeContent(w, r, "", info.ModTime(), f)
	}}
}
