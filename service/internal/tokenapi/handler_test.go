package tokenapi_test

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
	"github.com/s-frei/rezepte/service/internal/tokenapi"
	"github.com/s-frei/rezepte/service/internal/user"
)

// newHandler seeds an admin "sam" and a member "kim" (both password "pw")
// and returns the full-stack handler.
func newHandler(t *testing.T) http.Handler {
	t.Helper()
	conn := dbtest.Open(t)
	users := user.NewService(conn)
	for _, seed := range []struct {
		name string
		role user.Role
	}{{"sam", user.RoleAdmin}, {"kim", user.RoleUser}} {
		if _, err := users.Create(context.Background(), user.CreateParams{Username: seed.name, Password: "pw", Role: seed.role}); err != nil {
			t.Fatal(err)
		}
	}
	cfg, _ := config.LoadFrom(map[string]string{})
	sessions := auth.NewService(conn, users)
	tokens := auth.NewTokenService(conn, users)
	srv := httpserver.New(cfg, slog.New(slog.DiscardHandler), fstest.MapFS{},
		httpserver.WithAPIMiddleware(auth.Middleware(sessions, tokens, false)))
	auth.Register(srv.API(), sessions, false)
	tokenapi.Register(srv.API(), tokens)
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

func loginAs(t *testing.T, h http.Handler, username, password string) *http.Cookie { //nolint:unparam // helper mirrors brief signature; every current call site happens to use "pw"
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

func TestCreateReturnsTheRawTokenOnce(t *testing.T) {
	h := newHandler(t)
	admin := loginAs(t, h, "sam", "pw")

	rec := doReq(h, http.MethodPost, "/api/v1/tokens",
		`{"name":"mcp","scopes":["recipes:read"],"expiresInDays":30}`, admin)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status %d: %s", rec.Code, rec.Body.String())
	}
	var created tokenapi.CreatedAPIToken
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(created.Token, auth.TokenPrefix) {
		t.Fatalf("token = %q, want the %s prefix", created.Token, auth.TokenPrefix)
	}
	if created.ExpiresAt == nil {
		t.Fatal("ExpiresAt = nil, want a date 30 days out")
	}
	if created.OwnerUsername != "sam" {
		t.Fatalf("OwnerUsername = %q, want sam", created.OwnerUsername)
	}

	rec = doReq(h, http.MethodGet, "/api/v1/tokens", "", admin)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status %d: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), created.Token) {
		t.Fatal("the list response contains the raw token")
	}
	var list tokenapi.APITokenList
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 1 || list.Items[0].Prefix != created.Token[:8] {
		t.Fatalf("list = %+v", list.Items)
	}
}

func TestCreateWithoutExpiryIsAllowed(t *testing.T) {
	h := newHandler(t)
	admin := loginAs(t, h, "sam", "pw")
	rec := doReq(h, http.MethodPost, "/api/v1/tokens",
		`{"name":"forever","scopes":["recipes:read","recipes:write"]}`, admin)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var created tokenapi.CreatedAPIToken
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.ExpiresAt != nil {
		t.Fatalf("ExpiresAt = %v, want nil", created.ExpiresAt)
	}
}

func TestMembersAreForbidden(t *testing.T) {
	h := newHandler(t)
	member := loginAs(t, h, "kim", "pw")
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/v1/tokens", ""},
		{http.MethodPost, "/api/v1/tokens", `{"name":"x","scopes":["recipes:read"]}`},
		{http.MethodDelete, "/api/v1/tokens/whatever", ""},
	} {
		if rec := doReq(h, tc.method, tc.path, tc.body, member); rec.Code != http.StatusForbidden {
			t.Fatalf("%s %s: status %d, want 403", tc.method, tc.path, rec.Code)
		}
	}
}

func TestATokenCannotManageTokens(t *testing.T) {
	h := newHandler(t)
	admin := loginAs(t, h, "sam", "pw")
	rec := doReq(h, http.MethodPost, "/api/v1/tokens",
		`{"name":"mcp","scopes":["users:read","users:write"]}`, admin)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var created tokenapi.CreatedAPIToken
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	// Even with every user scope, the tokens endpoints stay session-only.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tokens", nil)
	req.Host = "localhost:8060"
	req.Header.Set("Authorization", "Bearer "+created.Token)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403: a token must never manage tokens", rec.Code)
	}
}

func TestDeleteRevokesAndIsIdempotentlyNotFound(t *testing.T) {
	h := newHandler(t)
	admin := loginAs(t, h, "sam", "pw")
	rec := doReq(h, http.MethodPost, "/api/v1/tokens", `{"name":"mcp","scopes":["recipes:read"]}`, admin)
	var created tokenapi.CreatedAPIToken
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if rec := doReq(h, http.MethodDelete, "/api/v1/tokens/"+created.ID, "", admin); rec.Code != http.StatusNoContent {
		t.Fatalf("delete status %d: %s", rec.Code, rec.Body.String())
	}
	if rec := doReq(h, http.MethodDelete, "/api/v1/tokens/"+created.ID, "", admin); rec.Code != http.StatusNotFound {
		t.Fatalf("second delete status %d, want 404", rec.Code)
	}
}

func TestUnknownScopeIsRejected(t *testing.T) {
	h := newHandler(t)
	admin := loginAs(t, h, "sam", "pw")
	rec := doReq(h, http.MethodPost, "/api/v1/tokens", `{"name":"x","scopes":["recipes:frobnicate"]}`, admin)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status %d, want 422: %s", rec.Code, rec.Body.String())
	}
}
