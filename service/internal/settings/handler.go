package settings

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/s-frei/rezepte/service/internal/auth"
)

type settingsOutput struct {
	Body Settings
}

type updateSettingsInput struct {
	Body struct {
		RecipesLockedByDefault bool `json:"recipesLockedByDefault" doc:"Lock every recipe on the default policy to its author and admins"`
	}
}

// Register installs get-settings (any session or a recipes:read token) and
// update-settings (the owner's session only).
func Register(api huma.API, svc *Service) {
	huma.Register(api, huma.Operation{
		OperationID: "get-settings",
		Method:      http.MethodGet,
		Path:        "/api/v1/settings",
		Summary:     "Get the household settings",
		Tags:        []string{"settings"},
		Security:    auth.Protected(auth.ScopeRecipesRead),
	}, func(ctx context.Context, _ *struct{}) (*settingsOutput, error) {
		s, err := svc.Get(ctx)
		if err != nil {
			return nil, err
		}
		return &settingsOutput{Body: s}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-settings",
		Method:      http.MethodPatch,
		Path:        "/api/v1/settings",
		Summary:     "Change the household settings",
		Tags:        []string{"settings"},
		Security:    auth.SessionSecurity,
		Errors:      []int{403},
	}, func(ctx context.Context, in *updateSettingsInput) (*settingsOutput, error) {
		u, ok := auth.UserFrom(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("authentication required")
		}
		s, err := svc.SetRecipesLockedByDefault(ctx, u, in.Body.RecipesLockedByDefault)
		if errors.Is(err, ErrOwnerRequired) {
			return nil, huma.Error403Forbidden("superadmin role required")
		}
		if err != nil {
			return nil, err
		}
		return &settingsOutput{Body: s}, nil
	})
}
