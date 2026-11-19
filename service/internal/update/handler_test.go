package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/config"
	"github.com/s-frei/rezepte/service/internal/db/dbtest"
	"github.com/s-frei/rezepte/service/internal/httpserver"
	"github.com/s-frei/rezepte/service/internal/user"
)

func newHandler(t *testing.T, feedBody string) http.Handler {
	t.Helper()
	conn := dbtest.Open(t)
	users := user.NewService(conn, "")
	for name, role := range map[string]user.Role{"olga": user.RoleSuperadmin, "adam": user.RoleAdmin, "mia": user.RoleUser} {
		if _, err := users.Create(context.Background(), user.CreateParams{Username: name, Password: "pw", Role: role}); err != nil {
			t.Fatal(err)
		}
	}
	cfg, _ := config.LoadFrom(map[string]string{})
	sessions := auth.NewService(conn, users)
	tokens := auth.NewTokenService(conn, users)
	srv := httpserver.New(cfg, quiet, fstest.MapFS{}, httpserver.Options{
		APIMiddleware: auth.Middleware(sessions, tokens, false),
	})
	auth.Register(srv.API(), sessions, auth.NewDevice(make([]byte, 32)), false)
	feedSrv, _ := feed(t, feedBody, 200)
	svc := NewService("1.5.0", feedSrv.URL, true, feedSrv.Client(), quiet)
	svc.Check(context.Background())
	Register(srv.API(), svc)
	return srv.Handler()
}

func get(t *testing.T, h http.Handler, username string) *httptest.ResponseRecorder {
	t.Helper()
	login := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"`+username+`","password":"pw"}`))
	login.Header.Set("Content-Type", "application/json")
	login.Host = "localhost:8060"
	login.Header.Set("Origin", "http://localhost:8060")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, login)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/settings/update", nil)
	req.Host = "localhost:8060"
	for _, c := range rec.Result().Cookies() {
		req.AddCookie(c)
	}
	out := httptest.NewRecorder()
	h.ServeHTTP(out, req)
	return out
}

func TestUpdateStatusOwnerOnly(t *testing.T) {
	h := newHandler(t, release("v1.6.0", false, false))
	for _, name := range []string{"mia", "adam"} {
		if rec := get(t, h, name); rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "superadmin role required") {
			t.Errorf("%s: %d %s", name, rec.Code, rec.Body.String())
		}
	}
	rec := get(t, h, "olga")
	want := `"latest":{"version":"1.6.0","publishedAt":"2026-11-01T10:00:00Z","changelogUrl":"https://getrezepte.app/changelog/v1.6.0/"}`
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"state":"available"`) || !strings.Contains(rec.Body.String(), `"current":"1.5.0"`) || !strings.Contains(rec.Body.String(), want) {
		t.Errorf("owner: %d %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateStatusCurrentHasNoLatest(t *testing.T) {
	rec := get(t, newHandler(t, release("v1.5.0", false, false)), "olga")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"state":"current"`) || strings.Contains(rec.Body.String(), "latest") {
		t.Errorf("owner: %d %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateStatusOmitsMissingPublishedAt(t *testing.T) {
	rec := get(t, newHandler(t, `{"tag_name":"v1.6.0"}`), "olga")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"state":"available"`) || strings.Contains(rec.Body.String(), "publishedAt") {
		t.Errorf("owner: %d %s", rec.Code, rec.Body.String())
	}
}
