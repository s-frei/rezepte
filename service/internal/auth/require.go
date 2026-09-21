package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"github.com/s-frei/rezepte/service/internal/user"
)

// RequireAuth is mux middleware for routes that are not registered as huma
// operations (the image files) but still need the same authentication as an
// operation declaring auth.Protected(scopes...). It accepts either a session
// cookie or an API token carrying every listed scope, re-issues the cookie on
// sliding renewal (mirroring Middleware) and stores the resolved user in the
// request context so auth.UserFrom(ctx) works downstream. Failures get a 401
// or 403 problem+json body instead of calling next.
func RequireAuth(sessions *Service, tokens *TokenService, secure bool, scopes ...string) func(http.Handler) http.Handler {
	return requireAuth(sessions, tokens, secure, scopes, func(w http.ResponseWriter, _ *http.Request, detail string) {
		writeUnauthorized(w, detail)
	})
}

// RequireAuthOrLogin is RequireAuth for a route a person opens in the address
// bar: instead of a 401 the browser would render as raw JSON, it sends them to
// the login form with a `next` back to where they were going. It requires no
// scope - any valid token opens it.
//
// Only a browser navigation is redirected - a GET whose Accept asks for HTML
// and which carries no bearer token. Everything else keeps the 401, and that
// distinction is load-bearing rather than cosmetic:
// docs/user/scripts/fetch-openapi.ts decides by res.ok, so a redirect it
// followed to a 200 login page would look like success and put the login page
// into openapi.json. Scalar's own fetch of the document asks for JSON and so
// keeps the 401 too.
func RequireAuthOrLogin(sessions *Service, tokens *TokenService, secure bool) func(http.Handler) http.Handler {
	return requireAuth(sessions, tokens, secure, nil, func(w http.ResponseWriter, r *http.Request, detail string) {
		if !navigatingBrowser(r) {
			writeUnauthorized(w, detail)
			return
		}
		next := url.QueryEscape(r.URL.RequestURI())
		http.Redirect(w, r, "/login?next="+next, http.StatusSeeOther)
	})
}

// requireAuth holds the authentication both variants share; deny is what they
// disagree about. A bearer request never reaches deny: it is answered with a
// plain 401 or 403, because a redirect to the login page is meaningless to a
// client that authenticates with a header.
func requireAuth(sessions *Service, tokens *TokenService, secure bool, scopes []string, deny func(http.ResponseWriter, *http.Request, string)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if raw, ok := bearerToken(r.Header.Get("Authorization")); ok {
				v, err := tokens.Authenticate(r.Context(), raw)
				if err != nil {
					writeUnauthorized(w, bearerAuthFailureMessage(err))
					return
				}
				if missing := missingScopes(v.Scopes, scopes); len(missing) > 0 {
					writeForbiddenScope(w, missing)
					return
				}
				serveAs(w, r, next, v.User)
				return
			}
			cookie, err := r.Cookie(CookieName)
			if err != nil {
				deny(w, r, "authentication required")
				return
			}
			v, err := sessions.Authenticate(r.Context(), cookie.Value)
			if err != nil {
				deny(w, r, "session invalid or expired")
				return
			}
			if v.Renewed {
				fresh := sessionCookie(cookie.Value, v.ExpiresAt, secure)
				http.SetCookie(w, &fresh)
			}
			serveAs(w, r, next, v.User)
		})
	}
}

// serveAs runs next with u stored in the request context.
func serveAs(w http.ResponseWriter, r *http.Request, next http.Handler, u user.User) {
	next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey{}, u)))
}

// writeForbiddenScope answers with an RFC 9457 problem+json 403 naming the
// scopes the token lacks, so the operator knows which token to re-issue.
func writeForbiddenScope(w http.ResponseWriter, missing []string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(huma.ErrorModel{
		Title:  http.StatusText(http.StatusForbidden),
		Status: http.StatusForbidden,
		Detail: "api token is missing scope " + strings.Join(missing, ", "),
	})
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
