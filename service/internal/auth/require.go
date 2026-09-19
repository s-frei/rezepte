package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

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
	return requireSession(svc, secure, func(w http.ResponseWriter, _ *http.Request, detail string) {
		writeUnauthorized(w, detail)
	})
}

// RequireSessionOrLogin is RequireSession for a route a person opens in the
// address bar: instead of a 401 the browser would render as raw JSON, it
// sends them to the login form with a `next` back to where they were going.
//
// Only a browser navigation is redirected - a GET whose Accept asks for
// HTML. Everything else keeps the 401, and that distinction is load-bearing
// rather than cosmetic: docs/user/scripts/fetch-openapi.ts decides by
// res.ok, so a redirect it followed to a 200 login page would look like
// success and put the login page into openapi.json. Scalar's own fetch of
// the document asks for JSON and so keeps the 401 too.
func RequireSessionOrLogin(svc *Service, secure bool) func(http.Handler) http.Handler {
	return requireSession(svc, secure, func(w http.ResponseWriter, r *http.Request, detail string) {
		if !navigatingBrowser(r) {
			writeUnauthorized(w, detail)
			return
		}
		next := url.QueryEscape(r.URL.RequestURI())
		http.Redirect(w, r, "/login?next="+next, http.StatusSeeOther)
	})
}

// requireSession holds the authentication both variants share; deny is what
// they disagree about.
func requireSession(svc *Service, secure bool, deny func(http.ResponseWriter, *http.Request, string)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(CookieName)
			if err != nil {
				deny(w, r, "authentication required")
				return
			}
			v, err := svc.Authenticate(r.Context(), cookie.Value)
			if err != nil {
				deny(w, r, "session invalid or expired")
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

// navigatingBrowser reports whether r looks like someone typing the address
// rather than code calling the API: a GET that explicitly asks for HTML.
// A bare */* does not count - that is what fetch and curl send.
func navigatingBrowser(r *http.Request) bool {
	return r.Method == http.MethodGet &&
		strings.Contains(r.Header.Get("Accept"), "text/html")
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
