package settings

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/user"
)

type settingsOutput struct {
	Body Settings
}

// updateSettingsInput changes the settings it names and leaves the rest.
type updateSettingsInput struct {
	Body struct {
		RecipesLockedByDefault *bool `json:"recipesLockedByDefault,omitempty" required:"false" doc:"Lock every recipe on the default policy to its author and admins"`
		LinkPreviews           *bool `json:"linkPreviews,omitempty" required:"false" doc:"Let a recipe link shared from the app show the recipe's title, description and cover in link previews"`
		LinkPreviewMinutes     *int  `json:"linkPreviewMinutes,omitempty" required:"false" enum:"15,60,1440" doc:"How many minutes such a link shows the recipe"`
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
		s, err := update(ctx, svc, u, in)
		if errors.Is(err, ErrOwnerRequired) {
			return nil, huma.Error403Forbidden("superadmin role required")
		}
		if errors.Is(err, ErrUnknownLifetime) {
			return nil, huma.Error422UnprocessableEntity(err.Error())
		}
		if err != nil {
			return nil, err
		}
		return &settingsOutput{Body: s}, nil
	})
}

// update applies the fields the request names, one after the other; a body
// naming neither returns the settings unchanged, after the same owner check.
func update(ctx context.Context, svc *Service, u user.User, in *updateSettingsInput) (Settings, error) {
	if !u.Role.IsSuperadmin() {
		return Settings{}, ErrOwnerRequired
	}
	if in.Body.RecipesLockedByDefault != nil {
		if _, err := svc.SetRecipesLockedByDefault(ctx, u, *in.Body.RecipesLockedByDefault); err != nil {
			return Settings{}, err
		}
	}
	if in.Body.LinkPreviews != nil {
		if _, err := svc.SetLinkPreviews(ctx, u, *in.Body.LinkPreviews); err != nil {
			return Settings{}, err
		}
	}
	if in.Body.LinkPreviewMinutes != nil {
		if _, err := svc.SetLinkPreviewMinutes(ctx, u, *in.Body.LinkPreviewMinutes); err != nil {
			return Settings{}, err
		}
	}
	return svc.Get(ctx)
}
