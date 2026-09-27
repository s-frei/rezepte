package share

import (
	"context"
	"errors"
	"net/http"
	"reflect"

	"github.com/danielgtaylor/huma/v2"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/user"
)

// shareList is the response body of list-shares (the schema ShareList).
type shareList struct {
	Items []Share `json:"items" nullable:"false"`
}

// ExistingShareError is the 409 create-public-share answers when the caller
// already has a public link for the recipe: a problem document that carries
// that link, so a second tab (or a double click) shows it instead of an
// error.
type ExistingShareError struct {
	huma.ErrorModel
	Share Share `json:"share" doc:"The caller's existing public link for this recipe"`
}

// RefusedError is the 403 create-public-share answers when the caller may
// not open a link: its reason says why in a word a client can switch on, so
// the detail stays free to change wording.
type RefusedError struct {
	huma.ErrorModel
	Reason string `json:"reason" enum:"sharing-off,not-allowed" doc:"sharing-off: the owner has switched public sharing off for the instance; not-allowed: an admin has withdrawn the right from this user"`
}

// The reasons a RefusedError carries.
const (
	ReasonSharingOff = "sharing-off"
	ReasonNotAllowed = "not-allowed"
)

func refused(err error, reason string) *RefusedError {
	return &RefusedError{
		ErrorModel: huma.ErrorModel{
			Title:  http.StatusText(http.StatusForbidden),
			Status: http.StatusForbidden,
			Detail: err.Error(),
		},
		Reason: reason,
	}
}

type shareOutput struct {
	Body Share
}

type listOutput struct {
	Body shareList
}

type createShareInput struct {
	ID   string `path:"id" doc:"Recipe id"`
	Body struct {
		Days *int `json:"days" enum:"1,7,30,365" nullable:"true" doc:"How many days the link lasts; null makes it permanent. Must not exceed publicShareMaxDays (null exceeds any maximum)."`
	}
}

type myShareInput struct {
	RecipeID string `path:"recipeId" doc:"Recipe id"`
}

// nullableShare is Share as get-my-public-share's body carries it: a nil
// *nullableShare marshals as JSON null on its own - the answer for "no live
// public link", which is a normal 200, not a 404 the recipe page would have
// to treat as an error on every view. huma only lets a `nullable` tag apply
// to a scalar field; a struct field that resolves to a $ref cannot carry it
// (huma panics), so this implements Schema itself and inlines Share's own
// object schema with Nullable set - the trick settings' nullableDay uses for
// a scalar, applied to an object instead.
type nullableShare Share

// Schema returns Share's own schema, cloned so the shared "Share" component
// other operations $ref is untouched, with Nullable set on the clone.
func (n nullableShare) Schema(r huma.Registry) *huma.Schema {
	base := r.Schema(reflect.TypeFor[Share](), false, "Share")
	s := *base
	s.Nullable = true
	return &s
}

// myShareBody is get-my-public-share's response body.
type myShareBody struct {
	Share *nullableShare `json:"share" doc:"The caller's public link for this recipe; null when there is none, or the recipe is unknown"`
}

type myShareOutput struct {
	Body myShareBody
}

type listSharesInput struct {
	All bool `query:"all" doc:"Everyone's links with their creators instead of the caller's own; admins only"`
}

type revokeShareInput struct {
	ID string `path:"id" doc:"Share id"`
}

type noContent struct{}

// currentUser is the session's user, as the auth middleware loaded it from
// the database for this very request - so a withdrawn right or a changed
// role shows in the next request's answer, not only after signing in again.
func currentUser(ctx context.Context) (user.User, error) {
	u, ok := auth.UserFrom(ctx)
	if !ok {
		return user.User{}, huma.Error401Unauthorized("authentication required")
	}
	return u, nil
}

// Register installs the five operations members manage their public links
// with. All of them are session-only: an API token cannot hand out public
// links to the household's recipes, nor list or revoke them.
//
// get-my-public-share lives at /shares/by-recipe/{recipeId} rather than
// under /recipes/{id}/: a GET there would conflict in the mux with
// GET /recipes/by-slug/{slug} (the path /recipes/by-slug/public-share
// matches both, and neither is more specific).
func Register(api huma.API, svc *Service) {
	registry := api.OpenAPI().Components.Schemas
	huma.Register(api, huma.Operation{
		OperationID:   "create-public-share",
		Method:        http.MethodPost,
		Path:          "/api/v1/recipes/{id}/public-share",
		Summary:       "Create a public link to a recipe",
		Description:   "Opens a link anyone can read the recipe by without signing in, until it is revoked or expires. A caller who already has one for the recipe gets it back with a 409.",
		Tags:          []string{"shares"},
		Security:      auth.SessionSecurity,
		DefaultStatus: http.StatusCreated,
		Errors:        []int{401, 404, 422},
		Responses: map[string]*huma.Response{
			"403": {
				Description: "Public sharing is off, or the caller may not share publicly; reason says which",
				Content: map[string]*huma.MediaType{
					"application/problem+json": {Schema: registry.Schema(reflect.TypeFor[RefusedError](), true, "")},
				},
			},
			"409": {
				Description: "The caller already has a public link for this recipe; the problem document carries it as share",
				Content: map[string]*huma.MediaType{
					"application/problem+json": {Schema: registry.Schema(reflect.TypeFor[ExistingShareError](), true, "")},
				},
			},
		},
	}, func(ctx context.Context, in *createShareInput) (*shareOutput, error) {
		u, err := currentUser(ctx)
		if err != nil {
			return nil, err
		}
		sh, err := svc.Create(ctx, u, in.ID, in.Body.Days)
		switch {
		case errors.Is(err, ErrExists):
			return nil, &ExistingShareError{
				ErrorModel: huma.ErrorModel{
					Title:  http.StatusText(http.StatusConflict),
					Status: http.StatusConflict,
					Detail: ErrExists.Error(),
				},
				Share: sh,
			}
		case errors.Is(err, ErrSharingOff):
			return nil, refused(err, ReasonSharingOff)
		case errors.Is(err, ErrNotAllowed):
			return nil, refused(err, ReasonNotAllowed)
		case errors.Is(err, ErrLifetime):
			return nil, huma.Error422UnprocessableEntity("validation failed", &huma.ErrorDetail{
				Location: "body.days",
				Message:  "lifetime exceeds the maximum",
				Value:    in.Body.Days,
			})
		case errors.Is(err, ErrRecipeNotFound):
			return nil, huma.Error404NotFound("recipe not found")
		case err != nil:
			return nil, err
		}
		return &shareOutput{Body: sh}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-my-public-share",
		Method:      http.MethodGet,
		Path:        "/api/v1/shares/by-recipe/{recipeId}",
		Summary:     "Get the caller's public link to a recipe",
		Description: "200 with share: null when the caller has no live public link for this recipe, or the recipe is unknown - not an error, so a page can call this on every view.",
		Tags:        []string{"shares"},
		Security:    auth.SessionSecurity,
		Errors:      []int{401},
	}, func(ctx context.Context, in *myShareInput) (*myShareOutput, error) {
		u, err := currentUser(ctx)
		if err != nil {
			return nil, err
		}
		sh, err := svc.Mine(ctx, u, in.RecipeID)
		if errors.Is(err, ErrNotFound) {
			return &myShareOutput{}, nil
		}
		if err != nil {
			return nil, err
		}
		ns := nullableShare(sh)
		return &myShareOutput{Body: myShareBody{Share: &ns}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list-shares",
		Method:      http.MethodGet,
		Path:        "/api/v1/shares",
		Summary:     "List public links",
		Description: "The caller's own public links, newest first. With all=true, an admin gets everyone's, each with its creator.",
		Tags:        []string{"shares"},
		Security:    auth.SessionSecurity,
		Errors:      []int{401, 403},
	}, func(ctx context.Context, in *listSharesInput) (*listOutput, error) {
		u, err := currentUser(ctx)
		if err != nil {
			return nil, err
		}
		items, err := svc.List(ctx, u, in.All)
		if errors.Is(err, ErrAdminRequired) {
			return nil, huma.Error403Forbidden("admin role required")
		}
		if err != nil {
			return nil, err
		}
		return &listOutput{Body: shareList{Items: items}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "revoke-share",
		Method:        http.MethodDelete,
		Path:          "/api/v1/shares/{id}",
		Summary:       "Revoke a public link",
		Description:   "The caller's own link, or anyone's as an admin. The link stops working at once.",
		Tags:          []string{"shares"},
		Security:      auth.SessionSecurity,
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{401, 404},
	}, func(ctx context.Context, in *revokeShareInput) (*noContent, error) {
		u, err := currentUser(ctx)
		if err != nil {
			return nil, err
		}
		err = svc.Revoke(ctx, u, in.ID)
		if errors.Is(err, ErrNotFound) {
			return nil, huma.Error404NotFound("share not found")
		}
		if err != nil {
			return nil, err
		}
		return &noContent{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "revoke-all-shares",
		Method:        http.MethodDelete,
		Path:          "/api/v1/shares",
		Summary:       "Revoke every public link",
		Description:   "Every public link in the household, whoever created it. Admins only.",
		Tags:          []string{"shares"},
		Security:      auth.SessionSecurity,
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{401, 403},
	}, func(ctx context.Context, _ *struct{}) (*noContent, error) {
		u, err := currentUser(ctx)
		if err != nil {
			return nil, err
		}
		err = svc.RevokeAll(ctx, u)
		if errors.Is(err, ErrAdminRequired) {
			return nil, huma.Error403Forbidden("admin role required")
		}
		if err != nil {
			return nil, err
		}
		return &noContent{}, nil
	})
}
