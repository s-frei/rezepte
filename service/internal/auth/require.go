package auth

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/s-frei/rezepte/service/internal/httpserver"
)

// RequireAuthOrLogin is mux middleware for the contract routes - the OpenAPI
// document, the JSON schemas and the Scalar docs page - which huma hangs
// straight off the mux rather than registering as operations, so
// Middleware never sees them. It accepts either a session cookie or any
// valid API token, requiring no scope, re-issues the cookie on sliding
// renewal (mirroring Middleware) and stores the resolved user in the request
// context.
//
// A person opening the page in the address bar is sent to the login form
// with a `next` back to where they were going, instead of a 401 the browser
// would render as raw JSON. Only a browser navigation is redirected - a GET
// whose Accept asks for HTML and which carries no bearer token. Everything
// else keeps the 401, and that distinction is load-bearing rather than
// cosmetic: docs/user/scripts/fetch-openapi.ts decides by res.ok, so a
// redirect it followed to a 200 login page would look like success and put
// the login page into openapi.json. Scalar's own fetch of the document asks
// for JSON and so keeps the 401 too. A bearer request is never redirected
// either: a login page is meaningless to a client that authenticates with a
// header.
func RequireAuthOrLogin(sessions *Service, tokens *TokenService, secure bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if raw, ok := bearerToken(r.Header.Get("Authorization")); ok {
				v, err := tokens.Authenticate(r.Context(), raw)
				if err != nil {
					w.Header().Set("WWW-Authenticate", "Bearer")
					writeUnauthorized(w, bearerAuthFailureMessage(err))
					return
				}
				next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey{}, v.User)))
				return
			}
			deny := func(detail string) {
				if !navigatingBrowser(r) {
					writeUnauthorized(w, detail)
					return
				}
				http.Redirect(w, r, "/login?next="+url.QueryEscape(r.URL.RequestURI()), http.StatusSeeOther)
			}
			cookie, err := r.Cookie(CookieName)
			if err != nil {
				deny("authentication required")
				return
			}
			v, err := sessions.Authenticate(r.Context(), cookie.Value)
			if err != nil {
				deny("session invalid or expired")
				return
			}
			if v.Renewed {
				fresh := SessionCookie(cookie.Value, v.ExpiresAt, secure)
				http.SetCookie(w, &fresh)
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey{}, v.User)))
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
// the shape huma.WriteErr produces for protected huma operations.
func writeUnauthorized(w http.ResponseWriter, detail string) {
	httpserver.WriteProblem(w, http.StatusUnauthorized, detail)
}
