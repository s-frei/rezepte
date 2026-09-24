package auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/db/dbtest"
	"github.com/s-frei/rezepte/service/internal/user"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

// requireEnv is the shared setup for RequireToken tests: an admin, the token
// service, and a session cookie value to prove a session is not accepted.
type requireEnv struct {
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
	users := user.NewService(conn)
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
	return &requireEnv{tokens: tokens, sessionToken: sess.Token, ownerID: sam.ID}
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
	users := user.NewService(conn)
	sessions := auth.NewService(conn, users)
	tokens := auth.NewTokenService(conn, users)
	h := auth.RequireAuth(sessions, tokens, false)(okHandler())

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
	users := user.NewService(conn)
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
	h := auth.RequireAuth(sessions, tokens, false)(next)

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
	users := user.NewService(conn)
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
	h := auth.RequireAuth(sessions, tokens, false)(okHandler())

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
	users := user.NewService(conn)
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
	users := user.NewService(conn)
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
	users := user.NewService(conn)
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

func TestRequireAuthAcceptsABearerTokenWithTheScope(t *testing.T) {
	ctx := context.Background()
	conn := dbtest.Open(t)
	users := user.NewService(conn)
	sam, err := users.Create(ctx, user.CreateParams{Username: "sam", Password: "pw", Role: user.RoleAdmin})
	if err != nil {
		t.Fatal(err)
	}
	sessions := auth.NewService(conn, users)
	tokens := auth.NewTokenService(conn, users)
	reader, _, err := tokens.Create(ctx, sam.ID, "reader", []string{auth.ScopeRecipesRead}, nil)
	if err != nil {
		t.Fatal(err)
	}
	writerOnly, _, err := tokens.Create(ctx, sam.ID, "writer", []string{auth.ScopeUsersRead}, nil)
	if err != nil {
		t.Fatal(err)
	}

	var seen string
	guarded := auth.RequireAuth(sessions, tokens, false, auth.ScopeRecipesRead)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u, _ := auth.UserFrom(r.Context())
			seen = u.Username
			w.WriteHeader(http.StatusOK)
		}))

	do := func(token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/images/a/b/c.jpg", nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		guarded.ServeHTTP(rec, req)
		return rec
	}

	if rec := do(reader); rec.Code != http.StatusOK || seen != "sam" {
		t.Fatalf("status = %d, user = %q, want 200 and sam", rec.Code, seen)
	}
	if rec := do(writerOnly); rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 for a token without recipes:read", rec.Code)
	}
	if rec := do(auth.TokenPrefix + "nope"); rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 for an unknown token", rec.Code)
	}
}

// TestRequireAuthValueWithoutPrefixGetsAComprehensibleMessage mirrors
// TestBearerValueWithoutPrefixGetsAComprehensibleMessage in bearer_test.go
// for the other bearer path (requireAuth, behind RequireAuth): a session
// cookie pasted as a bearer token must get the same message there too.
func TestRequireAuthValueWithoutPrefixGetsAComprehensibleMessage(t *testing.T) {
	conn := dbtest.Open(t)
	users := user.NewService(conn)
	sessions := auth.NewService(conn, users)
	tokens := auth.NewTokenService(conn, users)
	h := auth.RequireAuth(sessions, tokens, false, auth.ScopeRecipesRead)(okHandler())

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
	users := user.NewService(conn)
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

// TestRequireTokenRefusesSession covers why RequireToken exists alongside
// RequireAuth: a page on another site can make a logged-in browser send a
// cookie, but it cannot make it send an Authorization header, so a route
// only an API token may call must not accept one.
func TestRequireTokenRefusesSession(t *testing.T) {
	env := newRequireEnv(t)
	h := auth.RequireToken(env.tokens, auth.ScopeRecipesRead)(okHandler())
	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: env.sessionToken})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if got := rec.Header().Get("WWW-Authenticate"); got != "Bearer" {
		t.Fatalf("WWW-Authenticate = %q, want Bearer", got)
	}
}

func TestRequireTokenStoresScopes(t *testing.T) {
	env := newRequireEnv(t)
	raw := env.issue(t, auth.ScopeRecipesRead, auth.ScopeRecipesWrite)
	var got []string
	h := auth.RequireToken(env.tokens, auth.ScopeRecipesRead)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = auth.ScopesFrom(r.Context())
		if _, ok := auth.UserFrom(r.Context()); !ok {
			t.Error("user missing from context")
		}
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	h.ServeHTTP(httptest.NewRecorder(), req)
	if !slices.Equal(got, []string{auth.ScopeRecipesRead, auth.ScopeRecipesWrite}) {
		t.Fatalf("scopes = %v", got)
	}
}

func TestRequireTokenMissingScope(t *testing.T) {
	env := newRequireEnv(t)
	raw := env.issue(t, auth.ScopeUsersRead)
	h := auth.RequireToken(env.tokens, auth.ScopeRecipesRead)(okHandler())
	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}
