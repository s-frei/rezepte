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
	ID       string `json:"id" doc:"User id"`
	Username string `json:"username" doc:"Login name"`
	Role     string `json:"role" enum:"admin,user" doc:"Authorization role"`
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

// Register declares the OpenAPI cookie security scheme and installs the
// login, logout and me operations.
//
// The auth middleware itself is not installed here: it is passed to
// httpserver.New via httpserver.WithAPIMiddleware(auth.Middleware(...)),
// which applies it before this package (or any other) can register an
// operation. That ordering is structural, not a calling-convention
// requirement of this function.
func Register(api huma.API, svc *Service, secureCookies bool) {
	oapi := api.OpenAPI()
	if oapi.Components == nil {
		oapi.Components = &huma.Components{}
	}
	if oapi.Components.SecuritySchemes == nil {
		oapi.Components.SecuritySchemes = map[string]*huma.SecurityScheme{}
	}
	oapi.Components.SecuritySchemes[securityScheme] = &huma.SecurityScheme{
		Type: "apiKey",
		In:   "cookie",
		Name: CookieName,
	}

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
	return UserResponse{ID: u.ID, Username: u.Username, Role: string(u.Role)}
}
