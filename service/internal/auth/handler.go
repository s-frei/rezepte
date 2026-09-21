package auth

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/s-frei/rezepte/service/internal/user"
)

// UserResponse is the public representation of the current user.
type UserResponse struct {
	ID          string `json:"id" doc:"User id"`
	Username    string `json:"username" doc:"Login name"`
	DisplayName string `json:"displayName" doc:"Name shown wherever the UI names this person"`
	Role        string `json:"role" enum:"superadmin,admin,user" doc:"Authorization role"`
	Color       string `json:"color" enum:"amber,clay,rose,plum,sage,olive,teal,slate" doc:"Palette token identifying this person"`
}

type loginInput struct {
	Body struct {
		Username string `json:"username" minLength:"1" maxLength:"64"`
		Password string `json:"password" minLength:"1" maxLength:"1024"`
	}
}

type loginOutput struct {
	SetCookie http.Cookie `header:"Set-Cookie"`
	Body      UserResponse
}

type logoutInput struct {
	Cookie string `cookie:"rezepte_session"`
}

type logoutOutput struct {
	SetCookie http.Cookie `header:"Set-Cookie"`
}

type meOutput struct {
	Body UserResponse
}

type changePasswordInput struct {
	Cookie string `cookie:"rezepte_session"`
	Body   struct {
		CurrentPassword string `json:"currentPassword" minLength:"1" maxLength:"1024" doc:"The password in use now"`
		Password        string `json:"password" minLength:"8" maxLength:"128" doc:"The new password"`
	}
}

type changePasswordOutput struct{}

type updateProfileInput struct {
	Body struct {
		DisplayName *string `json:"displayName,omitempty" maxLength:"64" doc:"Empty falls back to the login name"`
		Color       *string `json:"color,omitempty" enum:"amber,clay,rose,plum,sage,olive,teal,slate"`
	}
}

type updateProfileOutput struct {
	Body UserResponse
}

type colorUsageOutput struct {
	Body struct {
		Items []user.ColorCount `json:"items" doc:"Every palette colour and how many accounts hold it, in palette order"`
	}
}

// Register installs the login, logout, me and change-password operations.
//
// The session security scheme itself is not declared here: it comes from
// SecuritySchemes, installed through httpserver.WithSecuritySchemes, which
// is the single place both schemes are described.
//
// The auth middleware itself is not installed here: it is passed to
// httpserver.New via httpserver.WithAPIMiddleware(auth.Middleware(...)),
// which applies it before this package (or any other) can register an
// operation. That ordering is structural, not a calling-convention
// requirement of this function.
func Register(api huma.API, svc *Service, secureCookies bool) {
	huma.Register(api, huma.Operation{
		OperationID: "login",
		Method:      http.MethodPost,
		Path:        "/api/v1/auth/login",
		Summary:     "Log in with username and password",
		Tags:        []string{"auth"},
		Errors:      []int{401},
	}, func(ctx context.Context, in *loginInput) (*loginOutput, error) {
		sess, err := svc.Login(ctx, in.Body.Username, in.Body.Password)
		if errors.Is(err, user.ErrInvalidCredentials) {
			return nil, huma.Error401Unauthorized("invalid username or password")
		}
		if err != nil {
			return nil, err
		}
		return &loginOutput{
			SetCookie: sessionCookie(sess.Token, sess.ExpiresAt, secureCookies),
			Body:      toResponse(sess.User),
		}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "logout",
		Method:        http.MethodPost,
		Path:          "/api/v1/auth/logout",
		Summary:       "End the current session",
		Tags:          []string{"auth"},
		Security:      SessionSecurity,
		DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *logoutInput) (*logoutOutput, error) {
		if err := svc.Logout(ctx, in.Cookie); err != nil {
			return nil, err
		}
		return &logoutOutput{SetCookie: expiredSessionCookie(secureCookies)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "me",
		Method:      http.MethodGet,
		Path:        "/api/v1/auth/me",
		Summary:     "Return the current user",
		Tags:        []string{"auth"},
		Security:    SessionSecurity,
		Errors:      []int{401},
	}, func(ctx context.Context, _ *struct{}) (*meOutput, error) {
		u, ok := UserFrom(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("authentication required")
		}
		return &meOutput{Body: toResponse(u)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "change-own-password",
		Method:        http.MethodPatch,
		Path:          "/api/v1/auth/me",
		Summary:       "Change the current user's password",
		Description:   "Verifies the current password, stores the new one and ends every other session of the user; the session making the call stays valid.",
		Tags:          []string{"auth"},
		Security:      SessionSecurity,
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{401, 422},
	}, func(ctx context.Context, in *changePasswordInput) (*changePasswordOutput, error) {
		u, ok := UserFrom(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("authentication required")
		}
		err := svc.users.ChangePassword(ctx, u.ID, in.Body.CurrentPassword, in.Body.Password)
		if errors.Is(err, user.ErrWrongPassword) {
			return nil, huma.Error422UnprocessableEntity("validation failed", &huma.ErrorDetail{
				Location: "body.currentPassword",
				Message:  "current password is wrong",
			})
		}
		if err != nil {
			return nil, err
		}
		if err := svc.DeleteUserSessionsExcept(ctx, u.ID, in.Cookie); err != nil {
			return nil, err
		}
		return &changePasswordOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-own-profile",
		Method:      http.MethodPatch,
		Path:        "/api/v1/auth/me/profile",
		Summary:     "Change the current user's display name and colour",
		Description: "Its own path rather than PATCH /api/v1/auth/me, which is the password change and demands the current password - a rename has nothing to do with it.",
		Tags:        []string{"auth"},
		Security:    SessionSecurity,
		Errors:      []int{401, 422},
	}, func(ctx context.Context, in *updateProfileInput) (*updateProfileOutput, error) {
		u, ok := UserFrom(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("authentication required")
		}
		if in.Body.DisplayName == nil && in.Body.Color == nil {
			return nil, huma.Error422UnprocessableEntity("nothing to change")
		}
		update := user.ProfileUpdate{DisplayName: in.Body.DisplayName}
		if in.Body.Color != nil {
			c := user.Color(*in.Body.Color)
			update.Color = &c
		}
		updated, err := svc.users.SetProfile(ctx, u.ID, update)
		if mapped := ProfileError(err); mapped != nil {
			return nil, mapped
		}
		if err != nil {
			return nil, err
		}
		return &updateProfileOutput{Body: toResponse(updated)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list-color-usage",
		Method:      http.MethodGet,
		Path:        "/api/v1/auth/me/colors",
		Summary:     "List the colour palette and how many accounts hold each colour",
		Description: "Counts, not names: the picker needs to mark a colour as taken, and a plain member may not list users. It sits under /auth/me because it describes what the caller may choose for themselves; everything under /users is admin-only.",
		Tags:        []string{"auth"},
		Security:    SessionSecurity,
		Errors:      []int{401},
	}, func(ctx context.Context, _ *struct{}) (*colorUsageOutput, error) {
		if _, ok := UserFrom(ctx); !ok {
			return nil, huma.Error401Unauthorized("authentication required")
		}
		usage, err := svc.users.ColorUsage(ctx)
		if err != nil {
			return nil, err
		}
		out := &colorUsageOutput{}
		out.Body.Items = usage
		return out, nil
	})
}

// ProfileError maps the profile validation failures to the field they belong
// to, and returns nil for anything else. An unknown colour cannot reach it
// through the API - huma refuses a value outside the enum with its own 422
// before the handler runs - but ErrInvalidColor is mapped anyway, because the
// service may be called from somewhere with no enum in front of it.
func ProfileError(err error) error {
	switch {
	case errors.Is(err, user.ErrDisplayNameTooLong):
		return huma.Error422UnprocessableEntity("validation failed", &huma.ErrorDetail{
			Location: "body.displayName", Message: "display name is too long",
		})
	case errors.Is(err, user.ErrInvalidDisplayName):
		return huma.Error422UnprocessableEntity("validation failed", &huma.ErrorDetail{
			Location: "body.displayName", Message: "display name must not contain control characters",
		})
	case errors.Is(err, user.ErrInvalidColor):
		return huma.Error422UnprocessableEntity("validation failed", &huma.ErrorDetail{
			Location: "body.color", Message: "unknown colour",
		})
	}
	return nil
}

func sessionCookie(token string, expires time.Time, secure bool) http.Cookie {
	return http.Cookie{ //nolint:gosec // G124: Secure follows the secureCookies config flag (false only for local http dev); HttpOnly and SameSite are always set.
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	}
}

// expiredSessionCookie is a Set-Cookie value that tells the browser to
// delete the session cookie immediately.
func expiredSessionCookie(secure bool) http.Cookie {
	return http.Cookie{ //nolint:gosec // G124: Secure follows the secureCookies config flag (false only for local http dev); HttpOnly and SameSite are always set.
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	}
}

func toResponse(u user.User) UserResponse {
	return UserResponse{
		ID:          u.ID,
		Username:    u.Username,
		DisplayName: u.DisplayName,
		Role:        string(u.Role),
		Color:       string(u.Color),
	}
}
