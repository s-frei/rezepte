package settings_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/config"
	"github.com/s-frei/rezepte/service/internal/db/dbtest"
	"github.com/s-frei/rezepte/service/internal/httpserver"
	"github.com/s-frei/rezepte/service/internal/settings"
	"github.com/s-frei/rezepte/service/internal/user"
)

func newHandler(t *testing.T) http.Handler {
	t.Helper()
	conn := dbtest.Open(t)
	users := user.NewService(conn)
	for name, role := range map[string]user.Role{"olga": user.RoleSuperadmin, "adam": user.RoleAdmin, "mia": user.RoleUser} {
		if _, err := users.Create(context.Background(), user.CreateParams{Username: name, Password: "pw", Role: role}); err != nil {
			t.Fatal(err)
		}
	}
	cfg, _ := config.LoadFrom(map[string]string{})
	sessions := auth.NewService(conn, users)
	tokens := auth.NewTokenService(conn, users)
	srv := httpserver.New(cfg, slog.New(slog.DiscardHandler), fstest.MapFS{},
		httpserver.WithAPIMiddleware(auth.Middleware(sessions, tokens, false)))
	auth.Register(srv.API(), sessions, false)
	settings.Register(srv.API(), settings.NewService(conn))
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

func login(t *testing.T, h http.Handler, username string) *http.Cookie {
	t.Helper()
	rec := do(h, http.MethodPost, "/api/v1/auth/login", `{"username":"`+username+`","password":"pw"}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("login %s: %d %s", username, rec.Code, rec.Body.String())
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.CookieName {
			return c
		}
	}
	t.Fatal("no session cookie")
	return nil
}

func TestSettingsEndpoints(t *testing.T) {
	h := newHandler(t)

	rec := do(h, http.MethodGet, "/api/v1/settings", "", login(t, h, "mia"))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"recipesLockedByDefault":false`) {
		t.Errorf("get as member: %d %s", rec.Code, rec.Body.String())
	}

	body := `{"recipesLockedByDefault":true}`
	for _, name := range []string{"mia", "adam"} {
		rec = do(h, http.MethodPatch, "/api/v1/settings", body, login(t, h, name))
		if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "superadmin role required") {
			t.Errorf("patch as %s: %d %s", name, rec.Code, rec.Body.String())
		}
	}

	rec = do(h, http.MethodPatch, "/api/v1/settings", body, login(t, h, "olga"))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"recipesLockedByDefault":true`) {
		t.Errorf("patch as owner: %d %s", rec.Code, rec.Body.String())
	}

	rec = do(h, http.MethodPatch, "/api/v1/settings", body, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("patch without session: %d", rec.Code)
	}
}
