package settings

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"

	"github.com/danielgtaylor/huma/v2"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/user"
)

// settings is Settings plus whether mail is on, which the settings service
// does not own (package mail does). huma names a schema after its Go type
// with the first letter raised, so this lowercase name keeps the API
// schema called Settings.
type settings struct {
	Settings
	MailEnabled bool `json:"mailEnabled" doc:"Whether Rezepte can send mail, which switches the Add account dialog"`
}

type settingsOutput struct {
	Body settings
}

// nullableDay is a PATCH field for a lifetime in days that tells apart three
// states a plain *int cannot: the key absent (leave the stored value alone),
// the key sent as null (make it permanent), and the key sent as a day count.
// encoding/json leaves a *int nil for both of the first two, which is wrong
// here - PublicShareDefaultDays and PublicShareMaxDays are settings that are
// genuinely nullable, not merely optional, so "send null to make it
// permanent again" has to be expressible once a maximum has been set.
type nullableDay struct {
	sent  bool
	value *int
}

// UnmarshalJSON records that the key was present in the body at all, on top
// of decoding its value.
func (n *nullableDay) UnmarshalJSON(b []byte) error {
	n.sent = true
	if bytes.Equal(b, []byte("null")) {
		n.value = nil
		return nil
	}
	var v int
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	n.value = &v
	return nil
}

// Schema documents nullableDay as the nullable integer it behaves like over
// the wire - the shape a plain *int field would get - so the field's own
// `enum` tag still applies on top of it.
func (n nullableDay) Schema(r huma.Registry) *huma.Schema {
	s := r.Schema(reflect.TypeOf(0), true, "")
	s.Nullable = true
	return s
}

// updateSettingsInput changes the settings it names and leaves the rest.
type updateSettingsInput struct {
	Body struct {
		RecipesLockedByDefault *bool       `json:"recipesLockedByDefault,omitempty" required:"false" doc:"Lock every recipe on the default policy to its author and admins"`
		LinkPreviews           *bool       `json:"linkPreviews,omitempty" required:"false" doc:"Let a recipe link shared from the app show the recipe's title, description and cover in link previews"`
		LinkPreviewMinutes     *int        `json:"linkPreviewMinutes,omitempty" required:"false" enum:"15,60,1440" doc:"How many minutes such a link shows the recipe"`
		PublicShares           *bool       `json:"publicShares,omitempty" required:"false" doc:"Let members turn a recipe into a public, read-only link"`
		PublicShareDefaultDays nullableDay `json:"publicShareDefaultDays,omitzero" required:"false" enum:"1,7,30,365" doc:"Lifetime preselected when a member creates a public link; omitted leaves it unchanged, null makes it permanent"`
		PublicShareMaxDays     nullableDay `json:"publicShareMaxDays,omitzero" required:"false" enum:"1,7,30,365" doc:"Longest lifetime a public link may have; omitted leaves it unchanged, null removes the maximum"`
		PublicShareAttribution *bool       `json:"publicShareAttribution,omitempty" required:"false" doc:"Name Rezepte, with a link to the project, at the foot of a public share page and of a recipe card"`
	}
}

// Register installs get-settings (any session or a recipes:read token) and
// update-settings (the owner's session only).
func Register(api huma.API, svc *Service, mailEnabled func(context.Context) bool) {
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
		return &settingsOutput{Body: settings{Settings: s, MailEnabled: mailEnabled(ctx)}}, nil
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
		if errors.Is(err, ErrUnknownLifetime) || errors.Is(err, ErrUnknownShareLifetime) {
			return nil, huma.Error422UnprocessableEntity(err.Error())
		}
		if err != nil {
			return nil, err
		}
		return &settingsOutput{Body: settings{Settings: s, MailEnabled: mailEnabled(ctx)}}, nil
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
	if in.Body.PublicShares != nil {
		if _, err := svc.SetPublicShares(ctx, u, *in.Body.PublicShares); err != nil {
			return Settings{}, err
		}
	}
	if in.Body.PublicShareAttribution != nil {
		if _, err := svc.SetPublicShareAttribution(ctx, u, *in.Body.PublicShareAttribution); err != nil {
			return Settings{}, err
		}
	}
	if in.Body.PublicShareDefaultDays.sent || in.Body.PublicShareMaxDays.sent {
		// A PATCH naming only one lifetime field keeps the other at its
		// stored value: SetShareLifetimes always writes both columns, so
		// the field left alone has to be read first. "Named" means the key
		// was present at all (sent), not that its value is non-nil - a null
		// is how the field is set back to permanent.
		current, err := svc.Get(ctx)
		if err != nil {
			return Settings{}, err
		}
		defaultDays, maxDays := current.PublicShareDefaultDays, current.PublicShareMaxDays
		if in.Body.PublicShareDefaultDays.sent {
			defaultDays = in.Body.PublicShareDefaultDays.value
		}
		if in.Body.PublicShareMaxDays.sent {
			maxDays = in.Body.PublicShareMaxDays.value
		}
		if _, err := svc.SetShareLifetimes(ctx, u, defaultDays, maxDays); err != nil {
			return Settings{}, err
		}
	}
	return svc.Get(ctx)
}
