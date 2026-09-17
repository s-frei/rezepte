package auth_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/danielgtaylor/huma/v2"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/config"
	"github.com/s-frei/rezepte/service/internal/db/dbtest"
	"github.com/s-frei/rezepte/service/internal/httpserver"
	"github.com/s-frei/rezepte/service/internal/user"
)

func newHandler(t *testing.T) http.Handler {
	t.Helper()
	conn := dbtest.Open(t)
	users := user.NewService(conn)
	if _, err := users.Create(context.Background(), "sam", "pw", user.RoleAdmin); err != nil {
		t.Fatal(err)
	}
	cfg, _ := config.LoadFrom(map[string]string{})
	srv := httpserver.New(cfg, slog.New(slog.DiscardHandler), fstest.MapFS{})
	auth.Register(srv.API(), auth.NewService(conn, users), false)
	return srv.Handler()
}

func do(h http.Handler, method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Host = "localhost:8060"
	req.Header.Set("Origin", "http://localhost:8060")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func login(t *testing.T, h http.Handler) *http.Cookie {
	t.Helper()
	rec := do(h, http.MethodPost, "/api/v1/auth/login", `{"username":"sam","password":"pw"}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("login status %d: %s", rec.Code, rec.Body.String())
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.CookieName {
			if !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.Path != "/" {
				t.Fatalf("cookie flags wrong: %+v", c)
			}
			return c
		}
	}
	t.Fatal("no session cookie set")
	return nil
}

func TestLoginRejectsWrongPassword(t *testing.T) {
	rec := do(newHandler(t), http.MethodPost, "/api/v1/auth/login", `{"username":"sam","password":"x"}`, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/problem+json") {
		t.Fatalf("content type %q", ct)
	}
}

func TestLoginValidatesBody(t *testing.T) {
	rec := do(newHandler(t), http.MethodPost, "/api/v1/auth/login", `{"username":""}`, nil)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
}

func TestMeRequiresSession(t *testing.T) {
	rec := do(newHandler(t), http.MethodGet, "/api/v1/auth/me", "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestLoginMeLogout(t *testing.T) {
	h := newHandler(t)
	cookie := login(t, h)

	rec := do(h, http.MethodGet, "/api/v1/auth/me", "", cookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"username":"sam"`) {
		t.Fatalf("me: %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"role":"admin"`) {
		t.Fatalf("me lacks role: %s", rec.Body.String())
	}

	rec = do(h, http.MethodPost, "/api/v1/auth/logout", "", cookie)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("logout: %d %s", rec.Code, rec.Body.String())
	}
	cleared := false
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.CookieName && c.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Fatal("logout did not clear the cookie")
	}

	rec = do(h, http.MethodGet, "/api/v1/auth/me", "", cookie)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("me after logout: %d", rec.Code)
	}
}

func TestOpenAPIDeclaresSessionSecurity(t *testing.T) {
	rec := do(newHandler(t), http.MethodGet, "/api/v1/openapi.json", "", nil)
	body := rec.Body.String()
	if !strings.Contains(body, `"session"`) || !strings.Contains(body, `"in":"cookie"`) {
		t.Fatalf("security scheme missing: %s", body)
	}
}

// pathParam matches OpenAPI {param}-style path segments.
var pathParam = regexp.MustCompile(`\{[^}]+\}`)

// TestEveryProtectedOperationRejectsAnonymous guards against a future
// operation being registered with Security: auth.SessionSecurity before
// auth.Register wires up the middleware (api.UseMiddleware only affects
// operations registered afterwards). Today this only covers logout and me,
// but it protects operations added later too.
func TestEveryProtectedOperationRejectsAnonymous(t *testing.T) {
	conn := dbtest.Open(t)
	users := user.NewService(conn)
	cfg, _ := config.LoadFrom(map[string]string{})
	srv := httpserver.New(cfg, slog.New(slog.DiscardHandler), fstest.MapFS{})
	auth.Register(srv.API(), auth.NewService(conn, users), false)
	h := srv.Handler()

	tested := 0
	for path, item := range srv.API().OpenAPI().Paths {
		for method, op := range map[string]*huma.Operation{
			http.MethodGet:     item.Get,
			http.MethodPut:     item.Put,
			http.MethodPost:    item.Post,
			http.MethodDelete:  item.Delete,
			http.MethodPatch:   item.Patch,
			http.MethodOptions: item.Options,
			http.MethodHead:    item.Head,
			http.MethodTrace:   item.Trace,
		} {
			if op == nil || len(op.Security) == 0 {
				continue
			}
			tested++
			concretePath := pathParam.ReplaceAllString(path, "x")
			rec := do(h, method, concretePath, "", nil)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("%s %s: status %d, want 401", method, concretePath, rec.Code)
			}
			if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/problem+json") {
				t.Fatalf("%s %s: content type %q, want application/problem+json prefix", method, concretePath, ct)
			}
		}
	}
	if tested == 0 {
		t.Fatal("no protected operations found to exercise")
	}
}
