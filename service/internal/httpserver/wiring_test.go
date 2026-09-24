package httpserver_test

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/config"
	"github.com/s-frei/rezepte/service/internal/db/dbtest"
	"github.com/s-frei/rezepte/service/internal/httpserver"
	"github.com/s-frei/rezepte/service/internal/image"
	"github.com/s-frei/rezepte/service/internal/mcpserver"
	"github.com/s-frei/rezepte/service/internal/recipe"
	"github.com/s-frei/rezepte/service/internal/settings"
	"github.com/s-frei/rezepte/service/internal/tokenapi"
	"github.com/s-frei/rezepte/service/internal/user"
	"github.com/s-frei/rezepte/service/internal/userapi"
)

// fullApp is the whole API wired the way main.go does it, plus the services
// a test needs to hand itself a session.
//
// It lives in package httpserver_test (not httpserver) because the feature
// packages import httpserver in their own tests; an external test package
// keeps that from becoming an import cycle.
type fullApp struct {
	srv      *httpserver.Server
	users    *user.Service
	sessions *auth.Service
	tokens   *auth.TokenService
}

func newFullApp(t *testing.T) fullApp {
	t.Helper()
	cfg, err := config.LoadFrom(map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	conn := dbtest.Open(t)
	users := user.NewService(conn)
	sessions := auth.NewService(conn, users)
	tokens := auth.NewTokenService(conn, users)

	srv := httpserver.New(cfg, slog.New(slog.DiscardHandler), fstest.MapFS{"index.html": {Data: []byte("app")}},
		httpserver.WithAPIMiddleware(auth.Middleware(sessions, tokens, cfg.SecureCookies)),
		httpserver.WithSecuritySchemes(auth.SecuritySchemes()),
		httpserver.WithSpecGuard(auth.RequireAuthOrLogin(sessions, tokens, cfg.SecureCookies)))
	auth.Register(srv.API(), sessions, cfg.SecureCookies)
	imageDir := filepath.Join(t.TempDir(), "images")
	recipes := recipe.NewService(conn, recipe.WithImageDir(imageDir))
	recipe.Register(srv.API(), recipes)
	mcpHandler, err := mcpserver.Handler(recipes, srv.API(), "test")
	if err != nil {
		t.Fatal(err)
	}
	srv.Handle("/mcp", auth.RequireToken(tokens, auth.ScopeRecipesRead)(mcpHandler))
	images := image.NewService(conn, imageDir)
	image.Register(srv.API(), images)
	settings.Register(srv.API(), settings.NewService(conn))
	srv.Handle("GET /images/{recipeId}/{imageId}/{file}",
		auth.RequireAuth(sessions, tokens, cfg.SecureCookies, auth.ScopeRecipesRead)(image.FileHandler(images)))
	userapi.Register(srv.API(), users, sessions)
	tokenapi.Register(srv.API(), tokens)
	return fullApp{srv: srv, users: users, sessions: sessions, tokens: tokens}
}

// issueToken creates an admin and returns the raw value of an API token of
// theirs holding scopes.
func (a fullApp) issueToken(t *testing.T, scopes ...string) string {
	t.Helper()
	ctx := t.Context()
	u, err := a.users.Create(ctx, user.CreateParams{Username: "agent", Password: "secret123", Role: user.RoleAdmin})
	if err != nil {
		t.Fatal(err)
	}
	raw, _, err := a.tokens.Create(ctx, u.ID, "t", scopes, nil)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// login creates a plain user and returns the cookie a browser would hold
// after signing in. Role user, not admin: reading the contract is not an
// administrative act, and the test would pass either way if it were.
func (a fullApp) login(t *testing.T) *http.Cookie {
	t.Helper()
	ctx := t.Context()
	if _, err := a.users.Create(ctx, user.CreateParams{Username: "reader", Password: "secret123", Role: user.RoleUser}); err != nil {
		t.Fatal(err)
	}
	s, err := a.sessions.Login(ctx, "reader", "secret123")
	if err != nil {
		t.Fatal(err)
	}
	return &http.Cookie{Name: auth.CookieName, Value: s.Token}
}

func (a fullApp) get(path string, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	a.srv.Handler().ServeHTTP(rec, req)
	return rec
}

// specRoutes is every route huma registers for the contract itself: the
// OpenAPI document in all four flavors it emits (3.1 plus the 3.0
// downgrades, each as JSON and YAML), the schema registry and the Scalar
// docs page. None of them is a huma operation, so auth.Middleware never
// sees them - httpserver.WithSpecGuard is what covers them.
var specRoutes = []string{
	"/api/v1/docs",
	"/api/v1/openapi.json",
	"/api/v1/openapi.yaml",
	"/api/v1/openapi-3.0.json",
	"/api/v1/openapi-3.0.yaml",
	"/api/v1/schemas/Card.json",
}

// TestSpecRoutesNeedASession pins that an instance reachable from the
// internet hands a stranger nothing to read. The document describes the
// deployed version's surface, which is more than the released spec the user
// docs publish - see docs/memory/content/features/users-and-auth.mdx.
func TestSpecRoutesNeedASession(t *testing.T) {
	app := newFullApp(t)
	for _, route := range specRoutes {
		if got := app.get(route, nil).Code; got != http.StatusUnauthorized {
			t.Errorf("GET %s without a cookie: status %d, want %d", route, got, http.StatusUnauthorized)
		}
	}
}

// TestSpecRoutesServeASession is the other half: the guard must not lock out
// the people who are supposed to read the contract.
func TestSpecRoutesServeASession(t *testing.T) {
	app := newFullApp(t)
	cookie := app.login(t)
	for _, route := range specRoutes {
		if got := app.get(route, cookie).Code; got != http.StatusOK {
			t.Errorf("GET %s with a session: status %d, want %d", route, got, http.StatusOK)
		}
	}
}

// TestBrowserIsSentToTheLoginForm is the assembled version of what
// auth.RequireAuthOrLogin promises: on the whole server, a person opening
// the docs page lands on the login form, while the document a script fetches
// from the very same guard still refuses with a 401.
func TestBrowserIsSentToTheLoginForm(t *testing.T) {
	app := newFullApp(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/docs", nil)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	rec := httptest.NewRecorder()
	app.srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Errorf("browser on /api/v1/docs: status %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/login?next=%2Fapi%2Fv1%2Fdocs" {
		t.Errorf("Location = %q", loc)
	}

	if got := app.get("/api/v1/openapi.json", nil).Code; got != http.StatusUnauthorized {
		t.Errorf("script on /api/v1/openapi.json: status %d, want %d", got, http.StatusUnauthorized)
	}
}

// TestPublicRoutesStayPublic guards the other direction: the path matching
// behind the guard is a prefix test, so it is the kind of thing that
// silently swallows more than it should.
func TestPublicRoutesStayPublic(t *testing.T) {
	app := newFullApp(t)
	for _, route := range []string{"/healthz", "/recipes/x"} {
		if got := app.get(route, nil).Code; got != http.StatusOK {
			t.Errorf("GET %s without a cookie: status %d, want %d", route, got, http.StatusOK)
		}
	}
}

// TestWellKnownIsNotTheSPA pins that nothing under /.well-known/ falls
// through to index.html: an MCP client that gets a 401 from /mcp looks there
// for OAuth metadata, and a 200 with the app shell would read as a broken
// document instead of "this server has none".
func TestWellKnownIsNotTheSPA(t *testing.T) {
	app := newFullApp(t)
	for _, route := range []string{
		"/.well-known/oauth-protected-resource",
		"/.well-known/oauth-protected-resource/mcp",
		"/.well-known/oauth-authorization-server",
		"/.well-known/openid-configuration",
	} {
		rec := app.get(route, nil)
		if rec.Code != http.StatusNotFound {
			t.Errorf("GET %s: status %d, want 404", route, rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/problem+json") {
			t.Errorf("GET %s: Content-Type = %q, want problem+json", route, ct)
		}
	}
}

// TestEveryFeatureRegisters renders the OpenAPI document. huma panics when
// two feature packages give different Go types the same schema name (e.g.
// two types called List), and it does so while generating the schema - so
// without this test such a clash only surfaces when the binary serves
// /api/v1/openapi.json, which no per-package handler test reaches because
// each registers a single feature.
func TestEveryFeatureRegisters(t *testing.T) {
	app := newFullApp(t)
	rec := app.get("/api/v1/openapi.json", app.login(t))
	if rec.Code != http.StatusOK {
		t.Fatalf("openapi.json: status %d, body %s", rec.Code, rec.Body.String())
	}

	var doc struct {
		Paths map[string]map[string]struct {
			OperationID string `json:"operationId"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("openapi.json is not JSON: %v", err)
	}
	ids := make(map[string]bool)
	for _, methods := range doc.Paths {
		for _, op := range methods {
			ids[op.OperationID] = true
		}
	}
	// One operation per feature package: proof that all six registrations
	// made it into the same document.
	for _, id := range []string{"login", "list-recipes", "upload-image", "get-settings", "list-users", "list-api-tokens"} {
		if !ids[id] {
			t.Errorf("operation %q missing from the OpenAPI document", id)
		}
	}
}

// TestEveryTagIsDeclared holds the tag names in the operations and the tag
// declarations of the document to each other. An operation's Tags field does
// not create a declaration, so a new endpoint under a new name emits a
// document that groups operations under a tag it never defines - which the
// Scalar page renders as a bare slug and the generated reference in
// docs/user/ skips entirely, since it builds one page per declared tag.
// A declaration nothing uses is the same fault the other way round: it
// generates an empty documentation page.
func TestEveryTagIsDeclared(t *testing.T) {
	app := newFullApp(t)
	rec := app.get("/api/v1/openapi.json", app.login(t))
	if rec.Code != http.StatusOK {
		t.Fatalf("openapi.json: status %d, body %s", rec.Code, rec.Body.String())
	}

	var doc struct {
		Tags  []struct{ Name string } `json:"tags"`
		Paths map[string]map[string]struct {
			Tags []string `json:"tags"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("openapi.json is not JSON: %v", err)
	}

	declared := make(map[string]bool, len(doc.Tags))
	for _, tag := range doc.Tags {
		declared[tag.Name] = true
	}
	used := make(map[string]bool)
	for path, methods := range doc.Paths {
		for method, op := range methods {
			for _, name := range op.Tags {
				used[name] = true
				if !declared[name] {
					t.Errorf("%s %s uses tag %q, which the document does not declare", method, path, name)
				}
			}
		}
	}
	for _, tag := range doc.Tags {
		if !used[tag.Name] {
			t.Errorf("tag %q is declared but no operation uses it", tag.Name)
		}
	}
}

func TestMCPRefusesSessionCookie(t *testing.T) {
	a := newFullApp(t)
	cookie := a.login(t)
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	a.srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestMCPGetIsNotTheSPA(t *testing.T) {
	a := newFullApp(t)
	rec := a.get("/mcp", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (not the SPA shell)", rec.Code)
	}
}

func TestMCPGetWithTokenIs405(t *testing.T) {
	a := newFullApp(t)
	raw := a.issueToken(t, auth.ScopeRecipesRead)
	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	rec := httptest.NewRecorder()
	a.srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}

func TestMCPTokenWithoutRecipesReadIs403(t *testing.T) {
	a := newFullApp(t)
	raw := a.issueToken(t, auth.ScopeUsersRead)
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+raw)
	rec := httptest.NewRecorder()
	a.srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

// TestMCPChecksOrigin pins the transport rule that a server MUST validate
// Origin: a token does not make a foreign page's request acceptable, while a
// same-origin or Origin-less client (every desktop MCP client) goes through.
func TestMCPChecksOrigin(t *testing.T) {
	a := newFullApp(t)
	raw := a.issueToken(t, auth.ScopeRecipesRead)
	cases := []struct {
		name   string
		origin string
		want   int
	}{
		{"foreign origin", "https://evil.example", http.StatusForbidden},
		{"same origin", "http://example.com", http.StatusOK},
		{"no origin", "", http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(
				`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"t","version":"0"}}}`))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", "application/json, text/event-stream")
			req.Header.Set("Authorization", "Bearer "+raw)
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			rec := httptest.NewRecorder()
			a.srv.Handler().ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.want, rec.Body.String())
			}
			if tc.want == http.StatusForbidden && !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/problem+json") {
				t.Fatalf("Content-Type = %q, want problem+json", rec.Header().Get("Content-Type"))
			}
		})
	}
}

// bearer adds an Authorization header to every request the MCP client sends.
type bearer struct{ raw string }

func (b bearer) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.raw)
	return http.DefaultTransport.RoundTrip(r)
}

// TestMCPSpeaksTheProtocol runs the SDK's own client against the whole
// server: initialize, tools/list and one write, through the same middleware
// chain and huma registry main.go builds, so a schema that only breaks under
// the production API config shows here.
func TestMCPSpeaksTheProtocol(t *testing.T) {
	a := newFullApp(t)
	raw := a.issueToken(t, auth.ScopeRecipesRead, auth.ScopeRecipesWrite, auth.ScopeRecipesDelete)
	ts := httptest.NewServer(a.srv.Handler())
	t.Cleanup(ts.Close)

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	cs, err := client.Connect(t.Context(), &mcp.StreamableClientTransport{
		Endpoint:   ts.URL + "/mcp",
		HTTPClient: &http.Client{Transport: bearer{raw: raw}},
		MaxRetries: -1,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	if name := cs.InitializeResult().ServerInfo.Name; name != "rezepte" {
		t.Fatalf("server name = %q", name)
	}

	tools, err := cs.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(tools.Tools) != 8 {
		t.Fatalf("%d tools, want 8", len(tools.Tools))
	}

	res, err := cs.CallTool(t.Context(), &mcp.CallToolParams{Name: "create_recipe", Arguments: map[string]any{"recipe": map[string]any{
		"title": "Soup", "description": "", "servings": 2,
		"prepMinutes": nil, "cookMinutes": nil, "sourceUrl": nil, "tags": []any{},
		"ingredientGroups": []any{map[string]any{"name": nil, "ingredients": []any{
			map[string]any{"quantity": 1, "unit": nil, "name": "Leek", "note": nil},
		}}},
		"steps": []any{map[string]any{"text": "Cook the leek.", "references": []any{}}},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("create_recipe: %v", res.Content)
	}
}
