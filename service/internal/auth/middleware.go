package auth

import (
	"context"
	"net/http"
	"slices"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"github.com/s-frei/rezepte/service/internal/user"
)

// Names of the OpenAPI security schemes this package defines.
const (
	sessionScheme = "session"
	tokenScheme   = "token"
)

// SessionSecurity marks an operation as requiring a logged-in browser
// session. API tokens cannot reach it: an operation that names no token
// scheme is structurally unreachable for them, which is how the session
// mechanics, token management and the household settings write stay
// session-only.
// Operations without Security are public.
var SessionSecurity = []map[string][]string{{sessionScheme: {}}}

// Protected marks an operation reachable by both a browser session and an
// API token carrying every listed scope. At least one scope is required: an
// empty list would make the operation reachable by any valid token, which is
// never what a caller wants (see RequireAuthOrLogin for the one place that
// is intentional, and which does not go through Protected). Protected is
// only ever called while operations are registered, at startup in
// main.go, so panicking here stops the service instead of shipping a hole.
func Protected(scopes ...string) []map[string][]string {
	if len(scopes) == 0 {
		panic("auth.Protected: at least one scope is required")
	}
	return []map[string][]string{{sessionScheme: {}}, {tokenScheme: scopes}}
}

// TokenOnly marks an operation only an API token carrying every listed scope
// may reach, such as the MCP endpoint. A session cookie is not accepted: a
// page on another site can make a logged-in browser send one, but it cannot
// make it send an Authorization header. It panics without a scope, for the
// reason Protected does.
func TokenOnly(scopes ...string) []map[string][]string {
	if len(scopes) == 0 {
		panic("auth.TokenOnly: at least one scope is required")
	}
	return []map[string][]string{{tokenScheme: scopes}}
}

// SecuritySchemes describes both authenticators for the OpenAPI document.
// It is installed through httpserver.WithSecuritySchemes so the scheme names
// stay in this package and httpserver keeps knowing nothing about auth.
func SecuritySchemes() map[string]*huma.SecurityScheme {
	return map[string]*huma.SecurityScheme{
		sessionScheme: {
			Type:        "apiKey",
			In:          "cookie",
			Name:        CookieName,
			Description: "Session cookie set by POST /api/v1/auth/login.",
		},
		tokenScheme: {
			Type:        "http",
			Scheme:      "bearer",
			Description: "API token from Settings, sent as: Authorization: Bearer " + TokenPrefix + "...",
		},
	}
}

// tokenScopes reports the scopes an API token must carry for op, and whether
// tokens may use it at all.
func tokenScopes(op *huma.Operation) ([]string, bool) {
	for _, requirement := range op.Security {
		if scopes, ok := requirement[tokenScheme]; ok {
			return scopes, true
		}
	}
	return nil, false
}

// sessionAllowed reports whether a session cookie may reach op.
func sessionAllowed(op *huma.Operation) bool {
	return slices.ContainsFunc(op.Security, func(requirement map[string][]string) bool {
		_, ok := requirement[sessionScheme]
		return ok
	})
}

// bearerToken extracts a token from an Authorization header value.
func bearerToken(header string) (string, bool) {
	const prefix = "Bearer "
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return "", false
	}
	token := strings.TrimSpace(header[len(prefix):])
	return token, token != ""
}

// missingScopes returns the entries of want that have does not contain.
func missingScopes(have, want []string) []string {
	var missing []string
	for _, scope := range want {
		if !slices.Contains(have, scope) {
			missing = append(missing, scope)
		}
	}
	return missing
}

type userKey struct{}

// UserFrom returns the authenticated user stored by the middleware.
func UserFrom(ctx context.Context) (user.User, bool) {
	u, ok := ctx.Value(userKey{}).(user.User)
	return u, ok
}

type viaTokenKey struct{}

// ViaToken reports whether ctx's request was authenticated by an API token
// rather than a session cookie. Set only by Middleware's bearer branch, so it
// answers truthfully only for huma-registered operations. create-user is its first
// caller: a bearer caller can never see a password it did not choose, so it
// cannot be handed a setup link either, and needs a typed password instead.
func ViaToken(ctx context.Context) bool {
	v, _ := ctx.Value(viaTokenKey{}).(bool)
	return v
}

type scopesKey struct{}

// ScopesFrom returns the scopes of the API token that authenticated ctx's
// request, or nil when it was not authenticated by a token. Operations that
// also accept a session never need them, since a session is not
// scope-limited.
func ScopesFrom(ctx context.Context) []string {
	s, _ := ctx.Value(scopesKey{}).([]string)
	return s
}

// Middleware authenticates every operation that declares Security.
//
// A request carrying an Authorization: Bearer header is decided by the token
// branch alone - there is no fallback to the cookie, so a stale browser
// session cannot silently rescue a revoked token. An operation marked
// TokenOnly refuses a request without one. Every 401 of the token branch
// carries WWW-Authenticate: Bearer per RFC 6750.
//
// On a sliding renewal of a cookie session (Authenticate reports Renewed) it
// re-issues the session cookie with the fresh expiry, so the browser's copy
// never falls behind the one stored in the database.
//
// It is installed via httpserver.WithAPIMiddleware, which applies it before
// any operation can be registered - see httpserver.New. This makes it
// structurally impossible to register a protected operation ahead of the
// middleware, unlike calling api.UseMiddleware directly from Register.
func Middleware(sessions *Service, tokens *TokenService, secureCookies bool) func(api huma.API) func(huma.Context, func(huma.Context)) {
	return func(api huma.API) func(huma.Context, func(huma.Context)) {
		return func(ctx huma.Context, next func(huma.Context)) {
			if len(ctx.Operation().Security) == 0 {
				next(ctx)
				return
			}
			if raw, ok := bearerToken(ctx.Header("Authorization")); ok {
				want, allowed := tokenScopes(ctx.Operation())
				if !allowed {
					_ = huma.WriteErr(api, ctx, http.StatusForbidden, "API tokens cannot use this operation")
					return
				}
				v, err := tokens.Authenticate(ctx.Context(), raw)
				if err != nil {
					ctx.SetHeader("WWW-Authenticate", "Bearer")
					_ = huma.WriteErr(api, ctx, http.StatusUnauthorized, bearerAuthFailureMessage(err))
					return
				}
				if missing := missingScopes(v.Scopes, want); len(missing) > 0 {
					_ = huma.WriteErr(api, ctx, http.StatusForbidden,
						"api token is missing scope "+strings.Join(missing, ", "))
					return
				}
				ctx = huma.WithValue(ctx, userKey{}, v.User)
				ctx = huma.WithValue(ctx, viaTokenKey{}, true)
				next(huma.WithValue(ctx, scopesKey{}, v.Scopes))
				return
			}
			if !sessionAllowed(ctx.Operation()) {
				ctx.SetHeader("WWW-Authenticate", "Bearer")
				_ = huma.WriteErr(api, ctx, http.StatusUnauthorized, "api token required")
				return
			}
			cookie, err := huma.ReadCookie(ctx, CookieName)
			if err != nil {
				_ = huma.WriteErr(api, ctx, http.StatusUnauthorized, "authentication required")
				return
			}
			v, err := sessions.Authenticate(ctx.Context(), cookie.Value)
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
