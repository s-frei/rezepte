package oidc

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/user"
)

// SignInSettings is the sign-in provider as the owner sees it; the client
// secret never leaves the service, only whether one is set.
type SignInSettings struct {
	Source                   Source `json:"source" enum:"env,settings,none" doc:"env: REZEPTE_OIDC_* pin it and the UI is read-only; settings: configured here; none: no provider"`
	Issuer                   string `json:"issuer"`
	ClientID                 string `json:"clientId"`
	SecretSet                bool   `json:"secretSet"`
	Name                     string `json:"name" doc:"The provider's name for the button as configured; empty means single sign-on"`
	RedirectURI              string `json:"redirectUri" doc:"The redirect URI to register at the provider; empty while REZEPTE_PUBLIC_URL is unset"`
	PublicURLMissing         bool   `json:"publicUrlMissing" doc:"REZEPTE_PUBLIC_URL is unset, so there is no redirect URI and sign-in cannot be turned on"`
	Connected                int    `json:"connected" doc:"Accounts connected at the issuer in force"`
	ConnectedWithoutPassword int    `json:"connectedWithoutPassword" doc:"Of those, the accounts without a password, who lose their way in when the issuer changes or sign-in is turned off"`
}

// SignInConfig is a configuration as the edit dialog sends it.
type SignInConfig struct {
	Issuer       string  `json:"issuer" minLength:"1"`
	ClientID     string  `json:"clientId" minLength:"1"`
	ClientSecret *string `json:"clientSecret,omitempty" doc:"Omitted keeps the stored secret, empty removes it"`
	Name         string  `json:"name,omitempty"`
}

type signInOutput struct{ Body SignInSettings }
type signInInput struct{ Body SignInConfig }

func signInOwner(ctx context.Context) (user.User, error) {
	u, ok := auth.UserFrom(ctx)
	if !ok {
		return user.User{}, huma.Error401Unauthorized("authentication required")
	}
	if !u.Role.IsSuperadmin() {
		return user.User{}, huma.Error403Forbidden("superadmin role required")
	}
	return u, nil
}

// mapSettingsErr answers the settings' errors with their status and leaves
// anything else to be the 500 it is.
func mapSettingsErr(err error) error {
	switch {
	case errors.Is(err, ErrOwnerRequired):
		return huma.Error403Forbidden("superadmin role required")
	case errors.Is(err, ErrEnvLocked):
		return huma.Error409Conflict("sign-in is set by the server environment")
	case errors.Is(err, ErrPublicURLMissing):
		return huma.Error409Conflict("REZEPTE_PUBLIC_URL is not set")
	case errors.Is(err, ErrDiscovery):
		// The cause (refused, timed out, another issuer) helps whoever
		// reads the log; the owner gets the field error.
		slog.Info("oidc discovery failed", "err", err)
		return huma.Error422UnprocessableEntity("validation failed", &huma.ErrorDetail{Location: "body.issuer", Message: "no OpenID provider answers at this address"})
	case errors.Is(err, ErrSecretRequired):
		return huma.Error422UnprocessableEntity("validation failed", &huma.ErrorDetail{Location: "body.clientSecret", Message: "enter the secret for a different provider"})
	case errors.Is(err, ErrInvalidConfig):
		return huma.Error422UnprocessableEntity(err.Error())
	}
	return err
}

func signInBody(ctx context.Context, s *Settings) (SignInSettings, error) {
	c, src, err := s.raw(ctx)
	if err != nil {
		return SignInSettings{}, err
	}
	total, without, err := s.Connected(ctx)
	if err != nil {
		return SignInSettings{}, err
	}
	b := SignInSettings{Source: src, Issuer: c.Issuer, ClientID: c.ClientID, SecretSet: c.ClientSecret != "", Name: c.Name,
		PublicURLMissing: c.PublicURL == "", Connected: total, ConnectedWithoutPassword: without}
	if c.PublicURL != "" {
		b.RedirectURI = c.PublicURL + "/api/v1/auth/oidc/callback"
	}
	return b, nil
}

// RegisterSettings installs the owner's sign-in settings operations.
func RegisterSettings(api huma.API, s *Settings) {
	huma.Register(api, huma.Operation{
		OperationID: "get-sign-in-settings", Method: http.MethodGet, Path: "/api/v1/settings/sign-in",
		Summary: "Get the sign-in provider", Tags: []string{"settings"},
		Security: auth.SessionSecurity, Errors: []int{401, 403},
	}, func(ctx context.Context, _ *struct{}) (*signInOutput, error) {
		if _, err := signInOwner(ctx); err != nil {
			return nil, err
		}
		b, err := signInBody(ctx, s)
		return &signInOutput{Body: b}, err
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-sign-in-settings", Method: http.MethodPut, Path: "/api/v1/settings/sign-in",
		Summary: "Configure the sign-in provider", Tags: []string{"settings"},
		Description: "Runs OpenID discovery at the issuer first and stores nothing when no provider answers (422 on body.issuer). Takes effect on the next request.",
		Security:    auth.SessionSecurity, Errors: []int{401, 403, 409, 422},
	}, func(ctx context.Context, in *signInInput) (*signInOutput, error) {
		u, err := signInOwner(ctx)
		if err != nil {
			return nil, err
		}
		c := Config{Issuer: in.Body.Issuer, ClientID: in.Body.ClientID, Name: in.Body.Name}
		if err := s.Save(ctx, u, c, in.Body.ClientSecret); err != nil {
			return nil, mapSettingsErr(err)
		}
		b, err := signInBody(ctx, s)
		return &signInOutput{Body: b}, err
	})

	huma.Register(api, huma.Operation{
		OperationID: "delete-sign-in-settings", Method: http.MethodDelete, Path: "/api/v1/settings/sign-in",
		Summary: "Turn sign-in through a provider off", Tags: []string{"settings"},
		Description: "Connected identities stay and count again when the same provider is configured.",
		Security:    auth.SessionSecurity, DefaultStatus: http.StatusNoContent, Errors: []int{401, 403, 409},
	}, func(ctx context.Context, _ *struct{}) (*struct{}, error) {
		u, err := signInOwner(ctx)
		if err != nil {
			return nil, err
		}
		if err := s.Clear(ctx, u); err != nil {
			return nil, mapSettingsErr(err)
		}
		return &struct{}{}, nil
	})
}
