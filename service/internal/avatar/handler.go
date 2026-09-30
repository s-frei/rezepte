package avatar

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/image"
	"github.com/s-frei/rezepte/service/internal/user"
)

// uploadForm is the multipart body: exactly one part named "file". huma
// checks the declared type; package image judges the bytes.
type uploadForm struct {
	File huma.FormFile `form:"file" contentType:"image/jpeg,image/png,image/webp" required:"true"`
}

const cropDoc = "The square to keep, as x,y,size: x and y place its top-left corner as fractions of the upright image's width and height, size is its side as a fraction of the shorter side. Absent keeps the centered square."

type ownUploadInput struct {
	Crop    string `query:"crop" doc:"The square to keep, as x,y,size; see the operation description"`
	RawBody huma.MultipartFormFiles[uploadForm]
}

type userUploadInput struct {
	ID      string `path:"id"`
	Crop    string `query:"crop" doc:"The square to keep, as x,y,size; see the operation description"`
	RawBody huma.MultipartFormFiles[uploadForm]
}

type userInput struct {
	ID string `path:"id"`
}

// avatarResult is the body of an upload: the id of the new picture. huma
// names the schema after the type, as AvatarResult, which says what it is
// in the API document where a bare Result would not.
type avatarResult struct {
	AvatarID string `json:"avatarId" doc:"The new picture, served at /avatars/{userId}/{avatarId}.jpg"`
}

type uploadOutput struct {
	Body avatarResult
}

type noContent struct{}

// Register installs the four picture operations: an account's own, which
// any session may use, and another account's, which only the owner may,
// the same rule user.CanEditProfile applies to the rest of a profile. The
// file route is not a huma operation, see FileHandler.
func Register(api huma.API, svc *Service) {
	upload := func(ctx context.Context, userID, crop string, file io.ReadCloser) (*uploadOutput, error) {
		defer file.Close()
		c, err := image.ParseCrop(crop)
		if err != nil {
			return nil, avatarErr(err)
		}
		id, err := svc.Set(ctx, userID, file, c)
		if err != nil {
			return nil, avatarErr(err)
		}
		return &uploadOutput{Body: avatarResult{AvatarID: id}}, nil
	}

	huma.Register(api, huma.Operation{
		OperationID:  "set-own-avatar",
		Method:       http.MethodPut,
		Path:         "/api/v1/auth/me/avatar",
		Summary:      "Set the own picture (JPEG, PNG or WebP, at most 10 MiB)",
		Description:  cropDoc,
		Tags:         []string{"auth"},
		Security:     auth.SessionSecurity,
		MaxBodyBytes: image.MaxUploadBytes,
		Middlewares:  huma.Middlewares{image.LimitUpload(api)},
		Errors:       []int{401, 413, 415, 422},
	}, func(ctx context.Context, in *ownUploadInput) (*uploadOutput, error) {
		u, ok := auth.UserFrom(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("authentication required")
		}
		return upload(ctx, u.ID, in.Crop, in.RawBody.Data().File)
	})

	huma.Register(api, huma.Operation{
		OperationID:   "delete-own-avatar",
		Method:        http.MethodDelete,
		Path:          "/api/v1/auth/me/avatar",
		Summary:       "Remove the own picture",
		Tags:          []string{"auth"},
		Security:      auth.SessionSecurity,
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{401},
	}, func(ctx context.Context, _ *struct{}) (*noContent, error) {
		u, ok := auth.UserFrom(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("authentication required")
		}
		if err := svc.Remove(ctx, u.ID); err != nil {
			return nil, avatarErr(err)
		}
		return &noContent{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:  "set-user-avatar",
		Method:       http.MethodPut,
		Path:         "/api/v1/users/{id}/avatar",
		Summary:      "Set another account's picture (owner only)",
		Description:  cropDoc,
		Tags:         []string{"users"},
		Security:     auth.Protected(auth.ScopeUsersWrite),
		MaxBodyBytes: image.MaxUploadBytes,
		Middlewares:  huma.Middlewares{image.LimitUpload(api)},
		Errors:       []int{401, 403, 404, 413, 415, 422},
	}, func(ctx context.Context, in *userUploadInput) (*uploadOutput, error) {
		if err := requireOwner(ctx); err != nil {
			return nil, err
		}
		return upload(ctx, in.ID, in.Crop, in.RawBody.Data().File)
	})

	huma.Register(api, huma.Operation{
		OperationID:   "delete-user-avatar",
		Method:        http.MethodDelete,
		Path:          "/api/v1/users/{id}/avatar",
		Summary:       "Remove another account's picture (owner only)",
		Tags:          []string{"users"},
		Security:      auth.Protected(auth.ScopeUsersWrite),
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{401, 403, 404},
	}, func(ctx context.Context, in *userInput) (*noContent, error) {
		if err := requireOwner(ctx); err != nil {
			return nil, err
		}
		if err := svc.Remove(ctx, in.ID); err != nil {
			return nil, avatarErr(err)
		}
		return &noContent{}, nil
	})
}

// requireOwner is user.CanEditProfile at the edge: only the instance owner
// writes another account's profile, picture included.
func requireOwner(ctx context.Context) error {
	actor, ok := auth.UserFrom(ctx)
	if !ok {
		return huma.Error401Unauthorized("authentication required")
	}
	if err := user.CanEditProfile(actor.Role); err != nil {
		return huma.Error403Forbidden("superadmin role required")
	}
	return nil
}

// avatarErr maps domain errors to their status; anything else is a 500.
func avatarErr(err error) error {
	switch {
	case errors.Is(err, ErrNotFound):
		return huma.Error404NotFound(err.Error())
	case errors.Is(err, image.ErrUnsupported):
		return huma.Error415UnsupportedMediaType(err.Error())
	case errors.Is(err, image.ErrInvalid), errors.Is(err, image.ErrTooLarge), errors.Is(err, image.ErrBadCrop):
		return huma.Error422UnprocessableEntity(err.Error())
	}
	return err
}

// immutableCache suits the files: an avatar id is never reused and its file
// never changes. "private" keeps shared caches out; the route is behind a
// session.
const immutableCache = "private, max-age=31536000, immutable"

// FileHandler serves GET /avatars/{userId}/{file}, file being
// "<avatar id>.jpg". Register it wrapped in auth.RequireAuth with
// auth.ScopeUsersRead. Anything but an existing picture is a 404.
func FileHandler(svc *Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := strings.CutSuffix(r.PathValue("file"), ".jpg")
		if !ok {
			http.NotFound(w, r)
			return
		}
		f, err := svc.Open(r.PathValue("userId"), id)
		if err == nil {
			err = image.ServeJPEG(w, r, f, immutableCache)
		}
		if err != nil {
			http.NotFound(w, r)
		}
	})
}
