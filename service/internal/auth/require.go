package auth

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

// RequireSession is mux middleware for routes that are not registered as
// huma operations (such as Phase 4's /images/) but still need session
// authentication equivalent to declaring Security: auth.SessionSecurity on
// a huma operation. It reads the session cookie, authenticates it via svc,
// re-issues the cookie on sliding renewal (mirroring Middleware) and stores
// the resolved user in the request context so auth.UserFrom(ctx) works for
// downstream handlers. A missing or invalid session gets a 401
// problem+json body instead of calling next.
func RequireSession(svc *Service, secure bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(CookieName)
			if err != nil {
				writeUnauthorized(w, "authentication required")
				return
			}
			v, err := svc.Authenticate(r.Context(), cookie.Value)
			if err != nil {
				writeUnauthorized(w, "session invalid or expired")
				return
			}
			if v.Renewed {
				fresh := sessionCookie(cookie.Value, v.ExpiresAt, secure)
				http.SetCookie(w, &fresh)
			}
			ctx := context.WithValue(r.Context(), userKey{}, v.User)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// writeUnauthorized answers with an RFC 9457 problem+json 401 body, matching
// the shape huma.WriteErr produces for protected huma operations (see
// httpserver.checkOrigin's writeForbiddenOrigin for the same pattern).
func writeUnauthorized(w http.ResponseWriter, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(huma.ErrorModel{
		Title:  http.StatusText(http.StatusUnauthorized),
		Status: http.StatusUnauthorized,
		Detail: detail,
	})
}
