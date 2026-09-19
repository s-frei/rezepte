package httpserver_test

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/config"
	"github.com/s-frei/rezepte/service/internal/db/dbtest"
	"github.com/s-frei/rezepte/service/internal/httpserver"
	"github.com/s-frei/rezepte/service/internal/image"
	"github.com/s-frei/rezepte/service/internal/recipe"
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

	srv := httpserver.New(cfg, slog.New(slog.DiscardHandler), fstest.MapFS{"index.html": {Data: []byte("app")}},
		httpserver.WithAPIMiddleware(auth.Middleware(sessions, cfg.SecureCookies)),
		httpserver.WithSpecGuard(auth.RequireSessionOrLogin(sessions, cfg.SecureCookies)))
	auth.Register(srv.API(), sessions, cfg.SecureCookies)
	imageDir := filepath.Join(t.TempDir(), "images")
	recipe.Register(srv.API(), recipe.NewService(conn, recipe.WithImageDir(imageDir)))
	images := image.NewService(conn, imageDir)
	image.Register(srv.API(), images)
	srv.Handle("GET /images/{recipeId}/{imageId}/{file}",
		auth.RequireSession(sessions, cfg.SecureCookies)(image.FileHandler(images)))
	userapi.Register(srv.API(), users, sessions)
	return fullApp{srv: srv, users: users, sessions: sessions}
}

// login creates a plain user and returns the cookie a browser would hold
// after signing in. Role user, not admin: reading the contract is not an
// administrative act, and the test would pass either way if it were.
func (a fullApp) login(t *testing.T) *http.Cookie {
	t.Helper()
	ctx := t.Context()
	if _, err := a.users.Create(ctx, "reader", "secret123", user.RoleUser); err != nil {
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
// OpenAPI document in all four flavours it emits (3.1 plus the 3.0
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
// docs publish - see ADR 0017.
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
// auth.RequireSessionOrLogin promises: on the whole server, a person opening
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
	// One operation per feature package: proof that all four registrations
	// made it into the same document.
	for _, id := range []string{"login", "list-recipes", "upload-image", "list-users"} {
		if !ids[id] {
			t.Errorf("operation %q missing from the OpenAPI document", id)
		}
	}
}
