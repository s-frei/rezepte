package auth_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/config"
	"github.com/s-frei/rezepte/service/internal/db/dbtest"
	"github.com/s-frei/rezepte/service/internal/httpserver"
	"github.com/s-frei/rezepte/service/internal/user"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

// requireEnv is the shared setup for the TokenOnly and RequireAuthOrLogin
// tests: an admin, both services, and a session cookie value.
type requireEnv struct {
	sessions     *auth.Service
	tokens       *auth.TokenService
	sessionToken string
	ownerID      string
}

// newRequireEnv seeds an admin ("sam"), logs them in for a session cookie
// value, and returns the token service alongside it.
func newRequireEnv(t *testing.T) *requireEnv {
	t.Helper()
	ctx := context.Background()
	conn := dbtest.Open(t)
	users := user.NewService(conn, "")
	sam, err := users.Create(ctx, user.CreateParams{Username: "sam", Password: "pw", Role: user.RoleAdmin})
	if err != nil {
		t.Fatal(err)
	}
	sessions := auth.NewService(conn, users)
	tokens := auth.NewTokenService(conn, users)
	sess, err := sessions.Login(ctx, "sam", "pw")
	if err != nil {
		t.Fatal(err)
	}
	return &requireEnv{sessions: sessions, tokens: tokens, sessionToken: sess.Token, ownerID: sam.ID}
}

// issue creates a token carrying scopes for env's owner and returns its raw
// value.
func (e *requireEnv) issue(t *testing.T, scopes ...string) string {
	t.Helper()
	raw, _, err := e.tokens.Create(context.Background(), e.ownerID, "t", scopes, nil)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestRequireSessionRejectsMissingCookie(t *testing.T) {
	conn := dbtest.Open(t)
	users := user.NewService(conn, "")
	sessions := auth.NewService(conn, users)
	tokens := auth.NewTokenService(conn, users)
	h := auth.RequireAuthOrLogin(sessions, tokens, false)(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/images/x", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/problem+json") {
		t.Fatalf("content type %q", ct)
	}
	if !strings.Contains(rec.Body.String(), `"status":401`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestRequireSessionAcceptsValidCookieAndStoresUser(t *testing.T) {
	conn := dbtest.Open(t)
	users := user.NewService(conn, "")
	if _, err := users.Create(context.Background(), user.CreateParams{Username: "sam", Password: "pw", Role: user.RoleAdmin}); err != nil {
		t.Fatal(err)
	}
	sessions := auth.NewService(conn, users)
	tokens := auth.NewTokenService(conn, users)
	sess, err := sessions.Login(context.Background(), "sam", "pw")
	if err != nil {
		t.Fatal(err)
	}

	var gotUser user.User
	var gotOK bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser, gotOK = auth.UserFrom(r.Context())
		w.WriteHeader(http.StatusOK)
	})
	h := auth.RequireAuthOrLogin(sessions, tokens, false)(next)

	req := httptest.NewRequest(http.MethodGet, "/images/x", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: sess.Token})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if !gotOK || gotUser.Username != "sam" {
		t.Fatalf("UserFrom = %+v, %v", gotUser, gotOK)
	}
}

func TestRequireSessionReissuesCookieOnRenewal(t *testing.T) {
	conn := dbtest.Open(t)
	users := user.NewService(conn, "")
	if _, err := users.Create(context.Background(), user.CreateParams{Username: "sam", Password: "pw", Role: user.RoleAdmin}); err != nil {
		t.Fatal(err)
	}
	sessions := auth.NewService(conn, users)
	tokens := auth.NewTokenService(conn, users)

	clock := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	sessions.SetClock(func() time.Time { return clock })

	sess, err := sessions.Login(context.Background(), "sam", "pw")
	if err != nil {
		t.Fatal(err)
	}

	// Past renewAfter (24h), so Authenticate reports Renewed.
	clock = clock.Add(10 * 24 * time.Hour)
	h := auth.RequireAuthOrLogin(sessions, tokens, false)(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/images/x", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: sess.Token})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	found := false
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.CookieName {
			found = true
		}
	}
	if !found {
		t.Fatal("renewed session did not re-issue the cookie")
	}
}

// browserAccept is what a browser sends when the address bar navigates. The
// redirect keys off exactly this.
const browserAccept = "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"

func TestRequireAuthOrLoginRedirectsABrowser(t *testing.T) {
	conn := dbtest.Open(t)
	users := user.NewService(conn, "")
	sessions := auth.NewService(conn, users)
	tokens := auth.NewTokenService(conn, users)
	h := auth.RequireAuthOrLogin(sessions, tokens, false)(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/docs", nil)
	req.Header.Set("Accept", browserAccept)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/login?next=%2Fapi%2Fv1%2Fdocs" {
		t.Fatalf("Location = %q", loc)
	}
}

// TestRequireAuthOrLoginKeeps401ForEverythingElse is what keeps
// fetch-openapi.ts honest: it checks res.ok, so a redirect followed to a 200
// login page would pass that check and write the login page into
// openapi.json. Anything that is not a browser navigating stays a 401.
func TestRequireAuthOrLoginKeeps401ForEverythingElse(t *testing.T) {
	conn := dbtest.Open(t)
	users := user.NewService(conn, "")
	sessions := auth.NewService(conn, users)
	tokens := auth.NewTokenService(conn, users)
	h := auth.RequireAuthOrLogin(sessions, tokens, false)(okHandler())

	cases := []struct {
		name   string
		method string
		accept string
	}{
		{"a script or XHR", http.MethodGet, "*/*"},
		{"scalar fetching the document", http.MethodGet, "application/json"},
		{"no Accept header at all", http.MethodGet, ""},
		{"a non-GET method", http.MethodPost, browserAccept},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(c.method, "/api/v1/openapi.json", nil)
			if c.accept != "" {
				req.Header.Set("Accept", c.accept)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status %d, want %d", rec.Code, http.StatusUnauthorized)
			}
			if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/problem+json") {
				t.Fatalf("content type %q", ct)
			}
		})
	}
}

func TestRequireAuthOrLoginServesAValidSession(t *testing.T) {
	conn := dbtest.Open(t)
	users := user.NewService(conn, "")
	if _, err := users.Create(context.Background(), user.CreateParams{Username: "sam", Password: "pw", Role: user.RoleAdmin}); err != nil {
		t.Fatal(err)
	}
	sessions := auth.NewService(conn, users)
	tokens := auth.NewTokenService(conn, users)
	sess, err := sessions.Login(context.Background(), "sam", "pw")
	if err != nil {
		t.Fatal(err)
	}
	h := auth.RequireAuthOrLogin(sessions, tokens, false)(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/docs", nil)
	req.Header.Set("Accept", browserAccept)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: sess.Token})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
}

// TestRequireAuthOrLoginAcceptsAnyValidToken: the contract routes require
// no scope, so a token carrying none of the recipe scopes opens them too.
func TestRequireAuthOrLoginAcceptsAnyValidToken(t *testing.T) {
	env := newRequireEnv(t)
	var seen string
	guarded := auth.RequireAuthOrLogin(env.sessions, env.tokens, false)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u, _ := auth.UserFrom(r.Context())
			seen = u.Username
			w.WriteHeader(http.StatusOK)
		}))
	do := func(token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/openapi.json", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		guarded.ServeHTTP(rec, req)
		return rec
	}
	if rec := do(env.issue(t, auth.ScopeUsersRead)); rec.Code != http.StatusOK || seen != "sam" {
		t.Fatalf("status = %d, user = %q, want 200 and sam", rec.Code, seen)
	}
	rec := do(auth.TokenPrefix + "nope")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 for an unknown token", rec.Code)
	}
	if got := rec.Header().Get("WWW-Authenticate"); got != "Bearer" {
		t.Fatalf("WWW-Authenticate = %q, want Bearer", got)
	}
}

// TestRequireAuthOrLoginValueWithoutPrefixGetsAComprehensibleMessage mirrors
// TestBearerValueWithoutPrefixGetsAComprehensibleMessage in bearer_test.go
// for the other bearer path, RequireAuthOrLogin: a session
// cookie pasted as a bearer token must get the same message there too.
func TestRequireAuthOrLoginValueWithoutPrefixGetsAComprehensibleMessage(t *testing.T) {
	conn := dbtest.Open(t)
	users := user.NewService(conn, "")
	sessions := auth.NewService(conn, users)
	tokens := auth.NewTokenService(conn, users)
	h := auth.RequireAuthOrLogin(sessions, tokens, false)(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/images/x", nil)
	req.Header.Set("Authorization", "Bearer not-a-token-at-all")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401: %s", rec.Code, rec.Body.String())
	}
	want := `this is not an API token; API tokens begin with \"` + auth.TokenPrefix + `\"`
	if !strings.Contains(rec.Body.String(), want) {
		t.Fatalf("body = %s, want detail containing %q", rec.Body.String(), want)
	}
}

func TestRequireAuthOrLoginNeverRedirectsABearerRequest(t *testing.T) {
	ctx := context.Background()
	conn := dbtest.Open(t)
	users := user.NewService(conn, "")
	if _, err := users.Create(ctx, user.CreateParams{Username: "sam", Password: "pw", Role: user.RoleAdmin}); err != nil {
		t.Fatal(err)
	}
	sessions := auth.NewService(conn, users)
	tokens := auth.NewTokenService(conn, users)

	guarded := auth.RequireAuthOrLogin(sessions, tokens, false)(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))

	// A bad token plus an HTML Accept header: the browser redirect must not
	// apply, because docs/user/scripts/fetch-openapi.ts decides by res.ok and
	// a followed redirect to a 200 login page would look like success.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/openapi.json", nil)
	req.Header.Set("Accept", "text/html")
	req.Header.Set("Authorization", "Bearer "+auth.TokenPrefix+"nope")
	rec := httptest.NewRecorder()
	guarded.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 rather than a redirect", rec.Code)
	}
}

// tokenOnlyStack serves one operation marked auth.TokenOnly with
// recipes:read behind the real middleware, recording the scopes it sees.
func tokenOnlyStack(t *testing.T, env *requireEnv, scopes *[]string) http.Handler {
	t.Helper()
	cfg, _ := config.LoadFrom(map[string]string{})
	srv := httpserver.New(cfg, slog.New(slog.DiscardHandler), fstest.MapFS{},
		httpserver.WithAPIMiddleware(auth.Middleware(env.sessions, env.tokens, false)),
		httpserver.WithSecuritySchemes(auth.SecuritySchemes()))
	huma.Register(srv.API(), huma.Operation{
		OperationID: "token-only",
		Method:      http.MethodPost,
		Path:        "/token-only",
		Security:    auth.TokenOnly(auth.ScopeRecipesRead),
	}, func(ctx context.Context, _ *struct{}) (*struct{}, error) {
		if _, ok := auth.UserFrom(ctx); !ok {
			t.Error("user missing from context")
		}
		*scopes = auth.ScopesFrom(ctx)
		return nil, nil
	})
	return srv.Handler()
}

func tokenOnlyReq(h http.Handler, authz string, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/token-only", nil)
	if authz != "" {
		req.Header.Set("Authorization", authz)
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// TestTokenOnlyRefusesSession covers why TokenOnly exists alongside
// Protected: a page on another site can make a logged-in browser send a
// cookie, but it cannot make it send an Authorization header, so an
// operation only an API token may call must not accept one.
func TestTokenOnlyRefusesSession(t *testing.T) {
	env := newRequireEnv(t)
	var scopes []string
	h := tokenOnlyStack(t, env, &scopes)
	rec := tokenOnlyReq(h, "", &http.Cookie{Name: auth.CookieName, Value: env.sessionToken})
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), "api token required") {
		t.Fatalf("status = %d %s, want 401 api token required", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("WWW-Authenticate"); got != "Bearer" {
		t.Fatalf("WWW-Authenticate = %q, want Bearer", got)
	}
}

func TestTokenOnlyStoresScopes(t *testing.T) {
	env := newRequireEnv(t)
	var scopes []string
	h := tokenOnlyStack(t, env, &scopes)
	raw := env.issue(t, auth.ScopeRecipesRead, auth.ScopeRecipesWrite)
	if rec := tokenOnlyReq(h, "Bearer "+raw, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d %s", rec.Code, rec.Body.String())
	}
	if !slices.Equal(scopes, []string{auth.ScopeRecipesRead, auth.ScopeRecipesWrite}) {
		t.Fatalf("scopes = %v", scopes)
	}
}

func TestTokenOnlyMissingScope(t *testing.T) {
	env := newRequireEnv(t)
	var scopes []string
	h := tokenOnlyStack(t, env, &scopes)
	rec := tokenOnlyReq(h, "Bearer "+env.issue(t, auth.ScopeUsersRead), nil)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "api token is missing scope recipes:read") {
		t.Fatalf("status = %d %s, want 403 naming recipes:read", rec.Code, rec.Body.String())
	}
}

func TestTokenOnlyPanicsWithNoScopes(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("TokenOnly() with no scopes did not panic")
		}
	}()
	auth.TokenOnly()
}
