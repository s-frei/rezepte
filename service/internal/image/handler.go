package image

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/recipe"
)

// uploadForm is the multipart body of the upload operation: exactly one
// part named "file". huma validates the part's declared Content-Type
// against the contentType list (422 on mismatch) before the handler runs;
// the bytes themselves are sniffed by decode (415 when they are not JPEG,
// PNG or WebP).
type uploadForm struct {
	File huma.FormFile `form:"file" contentType:"image/jpeg,image/png,image/webp" required:"true"`
}

type uploadInput struct {
	ID      string `path:"id"`
	RawBody huma.MultipartFormFiles[uploadForm]
}

type imageOutput struct {
	Body recipe.Image
}

type deleteImageInput struct {
	ID      string `path:"id"`
	ImageID string `path:"imageId"`
}

type deleteImageOutput struct{}

// OrderInput is the body of the reorder operation.
type OrderInput struct {
	ImageIDs []string `json:"imageIds" maxItems:"20" doc:"Every image id of the recipe, in the new order"`
}

type orderInput struct {
	ID   string `path:"id"`
	Body OrderInput
}

// OrderResult is the body of the reorder response. It is not named
// ImageList because that stutters as image.ImageList, nor List because a
// generic name like that risks colliding with another feature package's
// OpenAPI schema name (huma panics on a duplicate).
type OrderResult struct {
	Items []recipe.Image `json:"items"`
}

type orderOutput struct {
	Body OrderResult
}

// CoverInput is the body of the set-cover operation.
type CoverInput struct {
	ImageID string `json:"imageId" minLength:"1"`
}

type coverInput struct {
	ID   string `path:"id"`
	Body CoverInput
}

type coverOutput struct{}

// Register installs the image operations onto api: upload (multipart),
// delete, reorder and set-cover. Every operation requires a session or a
// scoped API token (Security: auth.Protected(...)); the file route is not a
// huma operation, see FileHandler.
func Register(api huma.API, svc *Service) {
	huma.Register(api, huma.Operation{
		OperationID:   "upload-image",
		Method:        http.MethodPost,
		Path:          "/api/v1/recipes/{id}/images",
		Summary:       "Upload an image (JPEG, PNG or WebP, at most 10 MiB)",
		Tags:          []string{"images"},
		Security:      auth.Protected(auth.ScopeRecipesWrite),
		DefaultStatus: http.StatusCreated,
		MaxBodyBytes:  maxUploadBytes,
		Middlewares:   huma.Middlewares{limitUpload(api)},
		Errors:        []int{404, 413, 415, 422},
	}, func(ctx context.Context, in *uploadInput) (*imageOutput, error) {
		u, ok := auth.UserFrom(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("authentication required")
		}
		file := in.RawBody.Data().File
		defer file.Close()
		img, err := svc.Upload(ctx, in.ID, u.ID, file)
		switch {
		case errors.Is(err, ErrNotFound):
			return nil, huma.Error404NotFound(err.Error())
		case errors.Is(err, ErrUnsupported):
			return nil, huma.Error415UnsupportedMediaType(err.Error())
		case errors.Is(err, ErrInvalid), errors.Is(err, ErrTooLarge), errors.Is(err, ErrTooMany):
			return nil, huma.Error422UnprocessableEntity(err.Error())
		case err != nil:
			return nil, err
		}
		return &imageOutput{Body: img}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "delete-image",
		Method:        http.MethodDelete,
		Path:          "/api/v1/recipes/{id}/images/{imageId}",
		Summary:       "Delete an image",
		Tags:          []string{"images"},
		Security:      auth.Protected(auth.ScopeRecipesWrite),
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{404},
	}, func(ctx context.Context, in *deleteImageInput) (*deleteImageOutput, error) {
		u, ok := auth.UserFrom(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("authentication required")
		}
		if err := svc.Delete(ctx, in.ID, in.ImageID, u.ID); err != nil {
			if errors.Is(err, ErrNotFound) {
				return nil, huma.Error404NotFound(err.Error())
			}
			return nil, err
		}
		return &deleteImageOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "reorder-images",
		Method:      http.MethodPut,
		Path:        "/api/v1/recipes/{id}/images/order",
		Summary:     "Reorder a recipe's images",
		Tags:        []string{"images"},
		Security:    auth.Protected(auth.ScopeRecipesWrite),
		Errors:      []int{404, 422},
	}, func(ctx context.Context, in *orderInput) (*orderOutput, error) {
		u, ok := auth.UserFrom(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("authentication required")
		}
		items, err := svc.Reorder(ctx, in.ID, u.ID, in.Body.ImageIDs)
		switch {
		case errors.Is(err, ErrNotFound):
			return nil, huma.Error404NotFound(err.Error())
		case errors.Is(err, ErrBadOrder):
			return nil, huma.Error422UnprocessableEntity(err.Error())
		case err != nil:
			return nil, err
		}
		if items == nil {
			items = []recipe.Image{}
		}
		return &orderOutput{Body: OrderResult{Items: items}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "set-cover",
		Method:        http.MethodPut,
		Path:          "/api/v1/recipes/{id}/cover",
		Summary:       "Choose the cover image",
		Tags:          []string{"images"},
		Security:      auth.Protected(auth.ScopeRecipesWrite),
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{404},
	}, func(ctx context.Context, in *coverInput) (*coverOutput, error) {
		u, ok := auth.UserFrom(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("authentication required")
		}
		if err := svc.SetCover(ctx, in.ID, in.Body.ImageID, u.ID); err != nil {
			if errors.Is(err, ErrNotFound) {
				return nil, huma.Error404NotFound(err.Error())
			}
			return nil, err
		}
		return &coverOutput{}, nil
	})
}

// limitUpload enforces the upload cap on the wire. huma's multipart path
// does not apply Operation.MaxBodyBytes (verified in v2.39.1: humago's
// GetMultipartForm calls ParseMultipartForm with no limit reader, and
// readBody, which honors MaxBodyBytes, is only used for non-multipart
// bodies). A declared Content-Length above the cap is answered 413 before
// anything is read; http.MaxBytesReader stops chunked or lying clients at
// the cap, which huma then reports as a 422 on "body".
//
// The declared length is read from Request.ContentLength rather than the
// raw header: net/http parses the header into it for every server request
// (-1 when the length is unknown, e.g. chunked), and httptest.NewRequest
// populates it without writing the header at all.
func limitUpload(api huma.API) func(ctx huma.Context, next func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		r, w := humago.Unwrap(ctx)
		if r.ContentLength > maxUploadBytes {
			_ = huma.WriteErr(api, ctx, http.StatusRequestEntityTooLarge, "upload larger than 10 MiB")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
		next(ctx)
	}
}
