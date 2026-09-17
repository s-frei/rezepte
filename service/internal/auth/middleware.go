package auth

import (
	"context"

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

// middleware authenticates every operation that declares SessionSecurity.
func (s *Service) middleware(api huma.API) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		if len(ctx.Operation().Security) == 0 {
			next(ctx)
			return
		}
		cookie, err := huma.ReadCookie(ctx, CookieName)
		if err != nil {
			_ = huma.WriteErr(api, ctx, 401, "authentication required")
			return
		}
		u, err := s.Authenticate(ctx.Context(), cookie.Value)
		if err != nil {
			_ = huma.WriteErr(api, ctx, 401, "session invalid or expired")
			return
		}
		next(huma.WithValue(ctx, userKey{}, u))
	}
}
