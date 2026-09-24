// Package userapi exposes user management (admin only) over HTTP. It is a
// separate package because it needs auth.UserFrom and auth.SessionSecurity,
// and package auth already imports package user.
package userapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/user"
)

// UserAccount is a user as the API shows it (no secrets).
type UserAccount struct {
	ID          string      `json:"id" doc:"User id"`
	Username    string      `json:"username" doc:"Login name"`
	DisplayName string      `json:"displayName" doc:"Name shown wherever the UI names this person"`
	Role        string      `json:"role" enum:"superadmin,admin,user" doc:"Authorization role"`
	Color       string      `json:"color" enum:"amber,clay,rose,plum,sage,olive,teal,slate" doc:"Palette token identifying this person"`
	Locale      user.Locale `json:"locale" doc:"The account holder's interface language"`
	CreatedAt   time.Time   `json:"createdAt" doc:"When the account was created"`
}

// UserAccountList is the response body of list-users.
type UserAccountList struct {
	Items []UserAccount `json:"items"`
}

type listOutput struct {
	Body UserAccountList
}

type createInput struct {
	Body struct {
		Username    string       `json:"username" minLength:"1" maxLength:"64"`
		Password    string       `json:"password" minLength:"8" maxLength:"128"`
		Role        string       `json:"role" enum:"admin,user"`
		DisplayName *string      `json:"displayName,omitempty" maxLength:"64" doc:"Empty falls back to the login name"`
		Color       *string      `json:"color,omitempty" enum:"amber,clay,rose,plum,sage,olive,teal,slate" doc:"Omitted picks the least-used color"`
		Locale      *user.Locale `json:"locale,omitempty" doc:"Interface language; defaults to REZEPTE_LOCALE"`
	}
}

type userOutput struct {
	Body UserAccount
}

type updateInput struct {
	ID   string `path:"id"`
	Body struct {
		Password    *string `json:"password,omitempty" minLength:"8" maxLength:"128" doc:"New password; ends all of the user's sessions"`
		Role        *string `json:"role,omitempty" enum:"admin,user"`
		DisplayName *string `json:"displayName,omitempty" maxLength:"64" doc:"Owner only"`
		Color       *string `json:"color,omitempty" enum:"amber,clay,rose,plum,sage,olive,teal,slate" doc:"Owner only"`
	}
}

type deleteInput struct {
	ID string `path:"id"`
}

type deleteOutput struct{}

// requireAdmin returns the calling user or a 401/403 huma error. Every
// operation in this package calls it first; there is no permission table,
// the rank of the session's user against the rank of the target is the whole
// authorization model. IsAdmin rather than a comparison against RoleAdmin,
// because the superadmin is an admin.
func requireAdmin(ctx context.Context) (user.User, error) {
	u, ok := auth.UserFrom(ctx)
	if !ok {
		return user.User{}, huma.Error401Unauthorized("authentication required")
	}
	if !u.Role.IsAdmin() {
		return user.User{}, huma.Error403Forbidden("admin role required")
	}
	return u, nil
}

// rankError maps the two authorization errors the user service returns.
// ErrSuperadminProtected states a fact about the instance that holds for every
// caller, which is the register ErrSelfDelete already uses; ErrSuperadminRequired
// is about who is asking. Returns nil for anything else.
func rankError(err error) error {
	switch {
	case errors.Is(err, user.ErrSuperadminProtected):
		return huma.Error409Conflict("the superadmin cannot be modified")
	case errors.Is(err, user.ErrSuperadminRequired):
		return huma.Error403Forbidden("superadmin role required")
	}
	return nil
}

func toResponse(u user.User) UserAccount {
	return UserAccount{
		ID:          u.ID,
		Username:    u.Username,
		DisplayName: u.DisplayName,
		Role:        string(u.Role),
		Color:       string(u.Color),
		Locale:      u.Locale,
		CreatedAt:   u.CreatedAt,
	}
}

// Register installs list, create, update and delete for users. All four
// require an admin session.
func Register(api huma.API, users *user.Service, sessions *auth.Service) {
	huma.Register(api, huma.Operation{
		OperationID: "list-users",
		Method:      http.MethodGet,
		Path:        "/api/v1/users",
		Summary:     "List users",
		Tags:        []string{"users"},
		Security:    auth.Protected(auth.ScopeUsersRead),
		Errors:      []int{401, 403},
	}, func(ctx context.Context, _ *struct{}) (*listOutput, error) {
		if _, err := requireAdmin(ctx); err != nil {
			return nil, err
		}
		list, err := users.List(ctx)
		if err != nil {
			return nil, err
		}
		items := make([]UserAccount, 0, len(list))
		for _, u := range list {
			items = append(items, toResponse(u))
		}
		return &listOutput{Body: UserAccountList{Items: items}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "create-user",
		Method:        http.MethodPost,
		Path:          "/api/v1/users",
		Summary:       "Create a user",
		Tags:          []string{"users"},
		Security:      auth.Protected(auth.ScopeUsersWrite),
		DefaultStatus: http.StatusCreated,
		Errors:        []int{401, 403, 409, 422, 503},
	}, func(ctx context.Context, in *createInput) (*userOutput, error) {
		actor, err := requireAdmin(ctx)
		if err != nil {
			return nil, err
		}
		// Create carries no authorization check of its own - that is what
		// lets the bootstrap path write the one superadmin row - so the rank
		// rule for a brand new account is checked here, where there is a
		// requested role but no target row to read.
		if err := user.CanAssignRole(actor.Role, user.Role(in.Body.Role)); err != nil {
			if mapped := rankError(err); mapped != nil {
				return nil, mapped
			}
			return nil, err
		}
		params := user.CreateParams{
			Username: in.Body.Username,
			Password: in.Body.Password,
			Role:     user.Role(in.Body.Role),
		}
		if in.Body.DisplayName != nil {
			params.DisplayName = *in.Body.DisplayName
		}
		if in.Body.Color != nil {
			params.Color = user.Color(*in.Body.Color)
		}
		if in.Body.Locale != nil {
			params.Locale = *in.Body.Locale
		}
		u, err := users.Create(ctx, params)
		if errors.Is(err, user.ErrUsernameTaken) {
			return nil, huma.Error409Conflict("username already taken")
		}
		if errors.Is(err, user.ErrInvalidUsername) {
			return nil, huma.Error422UnprocessableEntity("validation failed", &huma.ErrorDetail{
				Location: "body.username",
				Message:  "username must not be empty",
			})
		}
		if mapped := auth.ProfileError(err); mapped != nil {
			return nil, mapped
		}
		if mapped := auth.BusyError(err); mapped != nil {
			return nil, mapped
		}
		if err != nil {
			return nil, err
		}
		return &userOutput{Body: toResponse(u)}, nil
	})
	auth.DeclareRetryAfter(api, http.MethodPost, "/api/v1/users", http.StatusServiceUnavailable)

	huma.Register(api, huma.Operation{
		OperationID: "update-user",
		Method:      http.MethodPatch,
		Path:        "/api/v1/users/{id}",
		Summary:     "Change a user's role or profile, and/or reset the password",
		Tags:        []string{"users"},
		Security:    auth.Protected(auth.ScopeUsersWrite),
		Errors:      []int{401, 403, 404, 409, 422, 503},
	}, func(ctx context.Context, in *updateInput) (*userOutput, error) {
		actor, err := requireAdmin(ctx)
		if err != nil {
			return nil, err
		}
		hasProfile := in.Body.DisplayName != nil || in.Body.Color != nil
		if in.Body.Password == nil && in.Body.Role == nil && !hasProfile {
			return nil, huma.Error422UnprocessableEntity("nothing to change")
		}
		// Renaming somebody is not administration, so it is the owner's alone -
		// a different rule from guardTarget, which still decides the role
		// change and the password reset below.
		if hasProfile {
			if err := user.CanEditProfile(actor.Role); err != nil {
				if mapped := rankError(err); mapped != nil {
					return nil, mapped
				}
				if mapped := auth.BusyError(err); mapped != nil {
					return nil, mapped
				}
				return nil, err
			}
		}
		var u user.User
		if in.Body.Role != nil {
			u, err = users.SetRole(ctx, actor, in.ID, user.Role(*in.Body.Role))
		} else {
			u, err = users.ByID(ctx, in.ID)
		}
		if mapped := rankError(err); mapped != nil {
			return nil, mapped
		}
		if errors.Is(err, user.ErrNotFound) {
			return nil, huma.Error404NotFound("user not found")
		}
		if err != nil {
			return nil, err
		}
		if hasProfile {
			update := user.ProfileUpdate{DisplayName: in.Body.DisplayName}
			if in.Body.Color != nil {
				c := user.Color(*in.Body.Color)
				update.Color = &c
			}
			u, err = users.SetProfile(ctx, in.ID, update)
			if mapped := auth.ProfileError(err); mapped != nil {
				return nil, mapped
			}
			if errors.Is(err, user.ErrNotFound) {
				return nil, huma.Error404NotFound("user not found")
			}
			if err != nil {
				return nil, err
			}
		}
		if in.Body.Password != nil {
			if err := users.SetPassword(ctx, actor, in.ID, *in.Body.Password); err != nil {
				if mapped := rankError(err); mapped != nil {
					return nil, mapped
				}
				return nil, err
			}
			// A reset means the old credential is compromised or forgotten:
			// every device logged in with it goes.
			if err := sessions.DeleteUserSessionsExcept(ctx, in.ID, ""); err != nil {
				return nil, err
			}
		}
		return &userOutput{Body: toResponse(u)}, nil
	})
	auth.DeclareRetryAfter(api, http.MethodPatch, "/api/v1/users/{id}", http.StatusServiceUnavailable)

	huma.Register(api, huma.Operation{
		OperationID:   "delete-user",
		Method:        http.MethodDelete,
		Path:          "/api/v1/users/{id}",
		Summary:       "Delete a user",
		Tags:          []string{"users"},
		Security:      auth.Protected(auth.ScopeUsersWrite),
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{401, 403, 404, 409},
	}, func(ctx context.Context, in *deleteInput) (*deleteOutput, error) {
		actor, err := requireAdmin(ctx)
		if err != nil {
			return nil, err
		}
		err = users.Delete(ctx, actor, in.ID)
		if mapped := rankError(err); mapped != nil {
			return nil, mapped
		}
		switch {
		case errors.Is(err, user.ErrSelfDelete):
			return nil, huma.Error409Conflict("cannot delete yourself")
		case errors.Is(err, user.ErrNotFound):
			return nil, huma.Error404NotFound("user not found")
		case err != nil:
			return nil, err
		}
		return &deleteOutput{}, nil
	})
}
