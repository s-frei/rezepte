package userapi_test

import (
	"context"
	"encoding/json"
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
	"github.com/s-frei/rezepte/service/internal/user"
	"github.com/s-frei/rezepte/service/internal/userapi"
)

// newHandler seeds an admin "sam" and a member "kim" (both password "pw")
// and returns the full-stack handler, following internal/recipe/handler_test.go.
func newHandler(t *testing.T) http.Handler {
	t.Helper()
	conn := dbtest.Open(t)
	users := user.NewService(conn)
	for _, seed := range []struct {
		name string
		role user.Role
	}{{"sam", user.RoleAdmin}, {"kim", user.RoleUser}} {
		if _, err := users.Create(context.Background(), seed.name, "pw", seed.role); err != nil {
			t.Fatal(err)
		}
	}
	cfg, _ := config.LoadFrom(map[string]string{})
	sessions := auth.NewService(conn, users)
	srv := httpserver.New(cfg, slog.New(slog.DiscardHandler), fstest.MapFS{},
		httpserver.WithAPIMiddleware(auth.Middleware(sessions, false)))
	auth.Register(srv.API(), sessions, false)
	userapi.Register(srv.API(), users, sessions)
	return srv.Handler()
}

func doReq(h http.Handler, method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
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

func loginAs(t *testing.T, h http.Handler, username, password string) *http.Cookie {
	t.Helper()
	rec := doReq(h, http.MethodPost, "/api/v1/auth/login", `{"username":"`+username+`","password":"`+password+`"}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("login %s: status %d: %s", username, rec.Code, rec.Body.String())
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.CookieName {
			return c
		}
	}
	t.Fatal("no session cookie set")
	return nil
}

func listUsers(t *testing.T, h http.Handler, cookie *http.Cookie) []userapi.UserAccount {
	t.Helper()
	rec := doReq(h, http.MethodGet, "/api/v1/users", "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status %d: %s", rec.Code, rec.Body.String())
	}
	var list userapi.UserAccountList
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	return list.Items
}

func idOf(t *testing.T, items []userapi.UserAccount, username string) string {
	t.Helper()
	for _, u := range items {
		if u.Username == username {
			return u.ID
		}
	}
	t.Fatalf("no user %q in %+v", username, items)
	return ""
}

func TestUsersRequireAdmin(t *testing.T) {
	h := newHandler(t)
	if rec := doReq(h, http.MethodGet, "/api/v1/users", "", nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous: status %d", rec.Code)
	}
	kim := loginAs(t, h, "kim", "pw")
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/v1/users", ""},
		{http.MethodPost, "/api/v1/users", `{"username":"lea","password":"lea-password","role":"user"}`},
		{http.MethodPatch, "/api/v1/users/x", `{"role":"admin"}`},
		{http.MethodDelete, "/api/v1/users/x", ""},
	} {
		rec := doReq(h, tc.method, tc.path, tc.body, kim)
		if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "admin role required") {
			t.Fatalf("%s %s as member: status %d: %s", tc.method, tc.path, rec.Code, rec.Body.String())
		}
	}
}

func TestListAndCreate(t *testing.T) {
	h := newHandler(t)
	sam := loginAs(t, h, "sam", "pw")

	items := listUsers(t, h, sam)
	if len(items) != 2 || items[0].Username != "kim" || items[1].Username != "sam" || items[1].Role != "admin" || items[1].CreatedAt.IsZero() {
		t.Fatalf("items = %+v", items)
	}

	rec := doReq(h, http.MethodPost, "/api/v1/users", `{"username":"KIM","password":"kim-password","role":"user"}`, sam)
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate: status %d: %s", rec.Code, rec.Body.String())
	}
	rec = doReq(h, http.MethodPost, "/api/v1/users", `{"username":"lea","password":"short","role":"user"}`, sam)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "body.password") {
		t.Fatalf("short password: status %d: %s", rec.Code, rec.Body.String())
	}
	rec = doReq(h, http.MethodPost, "/api/v1/users", `{"username":"   ","password":"lea-password","role":"user"}`, sam)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "body.username") {
		t.Fatalf("blank username: status %d: %s", rec.Code, rec.Body.String())
	}
	rec = doReq(h, http.MethodPost, "/api/v1/users", `{"username":"lea","password":"lea-password","role":"user"}`, sam)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: status %d: %s", rec.Code, rec.Body.String())
	}
	var created userapi.UserAccount
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.ID == "" || created.Username != "lea" || created.Role != "user" {
		t.Fatalf("created = %+v", created)
	}
	if len(listUsers(t, h, sam)) != 3 {
		t.Fatal("expected 3 users after create")
	}
	if loginAs(t, h, "lea", "lea-password") == nil {
		t.Fatal("new user cannot log in")
	}
}

func TestUpdateRole(t *testing.T) {
	h := newHandler(t)
	sam := loginAs(t, h, "sam", "pw")
	items := listUsers(t, h, sam)
	samID, kimID := idOf(t, items, "sam"), idOf(t, items, "kim")

	rec := doReq(h, http.MethodPatch, "/api/v1/users/"+samID, `{"role":"user"}`, sam)
	if rec.Code != http.StatusConflict {
		t.Fatalf("demote only admin: status %d: %s", rec.Code, rec.Body.String())
	}
	rec = doReq(h, http.MethodPatch, "/api/v1/users/"+kimID, `{"role":"admin"}`, sam)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"role":"admin"`) {
		t.Fatalf("promote kim: status %d: %s", rec.Code, rec.Body.String())
	}
	rec = doReq(h, http.MethodPatch, "/api/v1/users/"+samID, `{"role":"user"}`, sam)
	if rec.Code != http.StatusOK {
		t.Fatalf("demote sam with second admin: status %d: %s", rec.Code, rec.Body.String())
	}
	rec = doReq(h, http.MethodPatch, "/api/v1/users/"+kimID, `{}`, sam)
	if rec.Code != http.StatusForbidden {
		// sam is a member now: the admin check comes first.
		t.Fatalf("empty body as demoted sam: status %d: %s", rec.Code, rec.Body.String())
	}
	kim := loginAs(t, h, "kim", "pw")
	rec = doReq(h, http.MethodPatch, "/api/v1/users/"+kimID, `{}`, kim)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("empty body: status %d: %s", rec.Code, rec.Body.String())
	}
	rec = doReq(h, http.MethodPatch, "/api/v1/users/missing", `{"role":"user"}`, kim)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown id: status %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdatePasswordEndsSessions(t *testing.T) {
	h := newHandler(t)
	sam := loginAs(t, h, "sam", "pw")
	kimID := idOf(t, listUsers(t, h, sam), "kim")
	kim1 := loginAs(t, h, "kim", "pw")
	kim2 := loginAs(t, h, "kim", "pw")

	rec := doReq(h, http.MethodPatch, "/api/v1/users/"+kimID, `{"password":"reset-password"}`, sam)
	if rec.Code != http.StatusOK {
		t.Fatalf("reset: status %d: %s", rec.Code, rec.Body.String())
	}
	for i, c := range []*http.Cookie{kim1, kim2} {
		if rec := doReq(h, http.MethodGet, "/api/v1/auth/me", "", c); rec.Code != http.StatusUnauthorized {
			t.Fatalf("kim session %d survived the reset: status %d", i+1, rec.Code)
		}
	}
	if rec := doReq(h, http.MethodPost, "/api/v1/auth/login", `{"username":"kim","password":"pw"}`, nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("old password: status %d", rec.Code)
	}
	loginAs(t, h, "kim", "reset-password")
}

func TestDelete(t *testing.T) {
	h := newHandler(t)
	sam := loginAs(t, h, "sam", "pw")
	items := listUsers(t, h, sam)
	samID, kimID := idOf(t, items, "sam"), idOf(t, items, "kim")
	kim := loginAs(t, h, "kim", "pw")

	if rec := doReq(h, http.MethodDelete, "/api/v1/users/"+samID, "", sam); rec.Code != http.StatusConflict {
		t.Fatalf("self delete: status %d: %s", rec.Code, rec.Body.String())
	}
	if rec := doReq(h, http.MethodDelete, "/api/v1/users/"+kimID, "", sam); rec.Code != http.StatusNoContent {
		t.Fatalf("delete kim: status %d: %s", rec.Code, rec.Body.String())
	}
	if rec := doReq(h, http.MethodGet, "/api/v1/auth/me", "", kim); rec.Code != http.StatusUnauthorized {
		t.Fatalf("kim's session survived the delete: status %d", rec.Code)
	}
	if rec := doReq(h, http.MethodDelete, "/api/v1/users/"+kimID, "", sam); rec.Code != http.StatusNotFound {
		t.Fatalf("delete twice: status %d: %s", rec.Code, rec.Body.String())
	}
	if len(listUsers(t, h, sam)) != 1 {
		t.Fatal("expected only sam to remain")
	}
}
