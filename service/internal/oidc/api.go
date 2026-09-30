package oidc

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/user"
)

type configOutput struct {
	Body struct {
		Enabled bool   `json:"enabled" doc:"Whether this instance offers sign-in through an identity provider"`
		Name    string `json:"name,omitempty" doc:"The provider's name for the button; absent when disabled"`
	}
}

type identityOutput struct {
	Body struct {
		LinkedAt time.Time `json:"linkedAt" doc:"When the account was connected to the provider"`
	}
}

// Register installs get-oidc, the two steps of the flow and the own-identity
// operations. A nil l (OIDC not configured) registers the same operations
// answering enabled:false and 404, so the OpenAPI document does not depend on
// configuration.
func Register(api huma.API, l *Login) {
	huma.Register(api, huma.Operation{
		OperationID: "get-oidc",
		Method:      http.MethodGet,
		Path:        "/api/v1/auth/oidc",
		Summary:     "Tell whether sign-in through an identity provider is offered",
		Description: "Public: the login page asks before anyone is signed in. The flow itself runs through POST /api/v1/auth/oidc/start, a form post the browser follows to the provider.",
		Tags:        []string{"auth"},
	}, func(context.Context, *struct{}) (*configOutput, error) {
		out := &configOutput{}
		if l != nil {
			out.Body.Enabled, out.Body.Name = true, l.cfg.Name
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-own-identity",
		Method:      http.MethodGet,
		Path:        "/api/v1/auth/me/identity",
		Summary:     "Tell whether the current account is connected to the identity provider",
		Tags:        []string{"auth"},
		Security:    auth.SessionSecurity,
		Errors:      []int{401, 404},
	}, func(ctx context.Context, _ *struct{}) (*identityOutput, error) {
		u, ok := auth.UserFrom(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("authentication required")
		}
		if l == nil {
			return nil, huma.Error404NotFound("no identity connected")
		}
		at, err := l.users.IdentityLinkedAt(ctx, u.ID, l.cfg.Issuer)
		if errors.Is(err, user.ErrNotFound) {
			return nil, huma.Error404NotFound("no identity connected")
		}
		if err != nil {
			return nil, err
		}
		out := &identityOutput{}
		out.Body.LinkedAt = at
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "unlink-own-identity",
		Method:        http.MethodDelete,
		Path:          "/api/v1/auth/me/identity",
		Summary:       "Disconnect the current account from the identity provider",
		Description:   "Refused with 409 while the account has no password: it would have no way left to sign in.",
		Tags:          []string{"auth"},
		Security:      auth.SessionSecurity,
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{401, 404, 409},
	}, func(ctx context.Context, _ *struct{}) (*struct{}, error) {
		u, ok := auth.UserFrom(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("authentication required")
		}
		if l == nil {
			return nil, huma.Error404NotFound("no identity connected")
		}
		err := l.users.UnlinkIdentity(ctx, u.ID, l.cfg.Issuer)
		switch {
		case errors.Is(err, user.ErrLastCredential):
			return nil, huma.Error409Conflict("set a password before disconnecting")
		case errors.Is(err, user.ErrNotFound):
			return nil, huma.Error404NotFound("no identity connected")
		case err != nil:
			return nil, err
		}
		return nil, nil
	})

	// Both steps answer a browser navigation with a 303, never JSON, so they
	// stream: the handler writes the redirect and its cookies itself.
	step := func(serve func(*Login, http.ResponseWriter, *http.Request)) func(context.Context, *struct{}) (*huma.StreamResponse, error) {
		return func(context.Context, *struct{}) (*huma.StreamResponse, error) {
			if l == nil {
				return nil, huma.Error404NotFound("sign-in through an identity provider is not configured")
			}
			return &huma.StreamResponse{Body: func(ctx huma.Context) {
				r, w := humago.Unwrap(ctx)
				serve(l, w, r)
			}}, nil
		}
	}
	huma.Register(api, huma.Operation{
		OperationID:   "start-oidc",
		Method:        http.MethodPost,
		Path:          "/api/v1/auth/oidc/start",
		Summary:       "Start signing in through the identity provider",
		Description:   "A form post (application/x-www-form-urlencoded) the browser follows: intent is login, link (connect the signed-in account) or setup (finish an account through its setup link, passed as setup); next is the SPA path to return to. Answers 303 to the provider, or back to the SPA with ?oidc=failed.",
		Tags:          []string{"auth"},
		DefaultStatus: http.StatusSeeOther,
		Errors:        []int{404},
	}, step((*Login).start))
	huma.Register(api, huma.Operation{
		OperationID:   "finish-oidc",
		Method:        http.MethodGet,
		Path:          "/api/v1/auth/oidc/callback",
		Summary:       "Finish signing in through the identity provider",
		Description:   "The redirect URI the provider sends the browser back to with code and state. Answers 303 into the SPA: signed in, connected, or with ?oidc=unlinked|taken|linked|failed.",
		Tags:          []string{"auth"},
		DefaultStatus: http.StatusSeeOther,
		Errors:        []int{404},
	}, step((*Login).callback))
}
