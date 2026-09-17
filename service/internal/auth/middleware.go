package auth

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/s-frei/rezepte/service/internal/user"
)

// securityScheme is the OpenAPI name of the cookie session scheme.
const securityScheme = "session"

// SessionSecurity marks an operation as requiring a logged-in user.
// Operations without Security are public.
var SessionSecurity = []map[string][]string{{securityScheme: {}}}

type userKey struct{}

// UserFrom returns the authenticated user stored by the middleware.
func UserFrom(ctx context.Context) (user.User, bool) {
	u, ok := ctx.Value(userKey{}).(user.User)
	return u, ok
}

// Middleware authenticates every operation that declares Security. On a
// sliding renewal (Authenticate reports Renewed) it re-issues the session
// cookie with the fresh expiry, so the browser's copy never falls behind the
// one stored in the database.
//
// It is installed via httpserver.WithAPIMiddleware, which applies it before
// any operation can be registered - see httpserver.New. This makes it
// structurally impossible to register a protected operation ahead of the
// middleware, unlike calling api.UseMiddleware directly from Register.
func Middleware(svc *Service, secureCookies bool) func(api huma.API) func(huma.Context, func(huma.Context)) {
	return func(api huma.API) func(huma.Context, func(huma.Context)) {
		return func(ctx huma.Context, next func(huma.Context)) {
			if len(ctx.Operation().Security) == 0 {
				next(ctx)
				return
			}
			cookie, err := huma.ReadCookie(ctx, CookieName)
			if err != nil {
				_ = huma.WriteErr(api, ctx, http.StatusUnauthorized, "authentication required")
				return
			}
			v, err := svc.Authenticate(ctx.Context(), cookie.Value)
			if err != nil {
				_ = huma.WriteErr(api, ctx, http.StatusUnauthorized, "session invalid or expired")
				return
			}
			if v.Renewed {
				fresh := sessionCookie(cookie.Value, v.ExpiresAt, secureCookies)
				ctx.AppendHeader("Set-Cookie", fresh.String())
			}
			next(huma.WithValue(ctx, userKey{}, v.User))
		}
	}
}
