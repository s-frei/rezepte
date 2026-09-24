package auth

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/s-frei/rezepte/service/internal/user"
)

// UserResponse is the public representation of the current user.
type UserResponse struct {
	ID          string      `json:"id" doc:"User id"`
	Username    string      `json:"username" doc:"Login name"`
	DisplayName string      `json:"displayName" doc:"Name shown wherever the UI names this person"`
	Role        string      `json:"role" enum:"superadmin,admin,user" doc:"Authorization role"`
	Color       string      `json:"color" enum:"amber,clay,rose,plum,sage,olive,teal,slate" doc:"Palette token identifying this person"`
	Locale      user.Locale `json:"locale" doc:"The account holder's interface language"`
}

type loginInput struct {
	Body struct {
		Username string `json:"username" minLength:"1" maxLength:"64"`
		Password string `json:"password" minLength:"1" maxLength:"1024"`
	}
}

type loginOutput struct {
	SetCookie []http.Cookie `header:"Set-Cookie"`
	Body      UserResponse
}

type logoutInput struct {
	Cookie string `cookie:"rezepte_session"`
}

type logoutOutput struct {
	SetCookie []http.Cookie `header:"Set-Cookie"`
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
		DisplayName *string      `json:"displayName,omitempty" maxLength:"64" doc:"Empty falls back to the login name"`
		Color       *string      `json:"color,omitempty" enum:"amber,clay,rose,plum,sage,olive,teal,slate"`
		Locale      *user.Locale `json:"locale,omitempty" doc:"The account holder's interface language"`
	}
}

type updateProfileOutput struct {
	SetCookie []http.Cookie `header:"Set-Cookie"`
	Body      UserResponse
}

type colorUsageOutput struct {
	Body struct {
		Items []user.ColorCount `json:"items" doc:"Every palette color and how many accounts hold it, in palette order"`
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
		Errors:      []int{401, 429, 503},
	}, func(ctx context.Context, in *loginInput) (*loginOutput, error) {
		sess, err := svc.Login(ctx, in.Body.Username, in.Body.Password)
		if errors.Is(err, user.ErrInvalidCredentials) {
			return nil, huma.Error401Unauthorized("invalid username or password")
		}
		var throttled *ThrottledError
		if errors.As(err, &throttled) {
			return nil, throttledError(throttled)
		}
		if mapped := BusyError(err); mapped != nil {
			return nil, mapped
		}
		if err != nil {
			return nil, err
		}
		return &loginOutput{
			SetCookie: []http.Cookie{
				sessionCookie(sess.Token, sess.ExpiresAt, secureCookies),
				localeCookie(sess.User.Locale, secureCookies),
			},
			Body: toResponse(sess.User),
		}, nil
	})
	DeclareRetryAfter(api, http.MethodPost, "/api/v1/auth/login", http.StatusTooManyRequests, http.StatusServiceUnavailable)

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
		return &logoutOutput{
			SetCookie: []http.Cookie{
				expiredSessionCookie(secureCookies),
				expiredLocaleCookie(secureCookies),
			},
		}, nil
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
		Errors:        []int{401, 422, 503},
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
		if mapped := BusyError(err); mapped != nil {
			return nil, mapped
		}
		if err != nil {
			return nil, err
		}
		if err := svc.DeleteUserSessionsExcept(ctx, u.ID, in.Cookie); err != nil {
			return nil, err
		}
		return &changePasswordOutput{}, nil
	})
	DeclareRetryAfter(api, http.MethodPatch, "/api/v1/auth/me", http.StatusServiceUnavailable)

	huma.Register(api, huma.Operation{
		OperationID: "update-own-profile",
		Method:      http.MethodPatch,
		Path:        "/api/v1/auth/me/profile",
		Summary:     "Change the current user's display name, color and interface language",
		Description: "Its own path rather than PATCH /api/v1/auth/me, which is the password change and demands the current password - a rename has nothing to do with it.",
		Tags:        []string{"auth"},
		Security:    SessionSecurity,
		Errors:      []int{401, 422},
	}, func(ctx context.Context, in *updateProfileInput) (*updateProfileOutput, error) {
		u, ok := UserFrom(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("authentication required")
		}
		if in.Body.DisplayName == nil && in.Body.Color == nil && in.Body.Locale == nil {
			return nil, huma.Error422UnprocessableEntity("nothing to change")
		}
		update := user.ProfileUpdate{DisplayName: in.Body.DisplayName}
		if in.Body.Color != nil {
			c := user.Color(*in.Body.Color)
			update.Color = &c
		}
		update.Locale = in.Body.Locale
		updated, err := svc.users.SetProfile(ctx, u.ID, update)
		if mapped := ProfileError(err); mapped != nil {
			return nil, mapped
		}
		if err != nil {
			return nil, err
		}
		return &updateProfileOutput{
			SetCookie: []http.Cookie{localeCookie(updated.Locale, secureCookies)},
			Body:      toResponse(updated),
		}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list-color-usage",
		Method:      http.MethodGet,
		Path:        "/api/v1/auth/me/colors",
		Summary:     "List the color palette and how many accounts hold each color",
		Description: "Counts, not names: the picker needs to mark a color as taken, and a plain member may not list users. It sits under /auth/me because it describes what the caller may choose for themselves; everything under /users is admin-only.",
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

// DeclareRetryAfter adds the Retry-After header, in whole seconds, to the
// listed error responses of the operation registered at method and path.
// huma writes an operation's error responses from its Errors list and
// declares no headers on them, so this runs after huma.Register.
func DeclareRetryAfter(api huma.API, method, path string, statuses ...int) {
	item := api.OpenAPI().Paths[path]
	op := map[string]*huma.Operation{
		http.MethodPost:  item.Post,
		http.MethodPatch: item.Patch,
		http.MethodPut:   item.Put,
	}[method]
	for _, status := range statuses {
		resp := op.Responses[strconv.Itoa(status)]
		if resp.Headers == nil {
			resp.Headers = map[string]*huma.Header{}
		}
		resp.Headers["Retry-After"] = &huma.Header{
			Description: "Seconds to wait before trying again",
			Schema:      &huma.Schema{Type: huma.TypeInteger},
		}
	}
}

// BusyError maps a full argon2 queue (user.ErrBusy) to a 503 that asks the
// client to retry in a second, and returns nil for anything else. Every
// operation that hashes or checks a password can meet it.
func BusyError(err error) error {
	if !errors.Is(err, user.ErrBusy) {
		return nil
	}
	return huma.ErrorWithHeaders(
		huma.Error503ServiceUnavailable("too many password checks at once, try again"),
		http.Header{"Retry-After": {"1"}},
	)
}

// ProfileError maps the profile validation failures to the field they belong
// to, and returns nil for anything else. An unknown color or locale cannot
// reach it through the API - huma refuses a value outside the enum with its
// own 422 before the handler runs - but ErrInvalidColor and ErrInvalidLocale
// are mapped anyway, because the service may be called from somewhere with
// no enum in front of it.
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
			Location: "body.color", Message: "unknown color",
		})
	case errors.Is(err, user.ErrInvalidLocale):
		return huma.Error422UnprocessableEntity("validation failed", &huma.ErrorDetail{
			Location: "body.locale", Message: "unknown interface language",
		})
	}
	return nil
}

// throttledError is the 429 for a locked username. Retry-After is rounded up
// to whole seconds, so a client that waits exactly that long is let through.
func throttledError(e *ThrottledError) error {
	seconds := int64((e.RetryAfter + time.Second - 1) / time.Second)
	return huma.ErrorWithHeaders(
		huma.Error429TooManyRequests("too many failed login attempts, try again later"),
		http.Header{"Retry-After": {strconv.FormatInt(seconds, 10)}},
	)
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

// LocaleCookieName is the cookie Paraglide's cookie strategy reads. The name
// is Paraglide's own default, configured in frontend/vite.config.ts; the two
// have to agree, and this is the writing end.
const LocaleCookieName = "PARAGLIDE_LOCALE"

// localeCookie carries the account's interface language to the SPA, which
// resolves its locale before the first render and cannot wait for
// GET /auth/me. The users row stays the source of truth: only the service
// writes this cookie, and only from that column.
//
// Deliberately not HttpOnly - Paraglide reads it from JavaScript. It holds a
// display preference and nothing a session could be hijacked with.
func localeCookie(l user.Locale, secure bool) http.Cookie {
	return http.Cookie{ //nolint:gosec // G124: not HttpOnly by design (Paraglide reads it in the browser); Secure follows the secureCookies config flag and SameSite is always set.
		Name:     LocaleCookieName,
		Value:    string(l),
		Path:     "/",
		MaxAge:   int((365 * 24 * time.Hour).Seconds()),
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	}
}

// expiredLocaleCookie clears the locale cookie on logout. The cookie names an
// account's language, and once nobody is signed in that preference no longer
// applies; the SPA's login screen falls back to Paraglide's preferredLanguage
// strategy (the browser's own Accept-Language) instead of staying stuck on
// whichever account last logged out - which matters on a shared machine.
func expiredLocaleCookie(secure bool) http.Cookie {
	return http.Cookie{ //nolint:gosec // G124: not HttpOnly by design (Paraglide reads it in the browser); Secure follows the secureCookies config flag and SameSite is always set.
		Name:     LocaleCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
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
		Locale:      u.Locale,
	}
}
