package auth_test

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
	"github.com/s-frei/rezepte/service/internal/user"
	"github.com/s-frei/rezepte/service/internal/userapi"
)

// newBearerStack builds the full server with a token service and returns the
// handler plus a raw token carrying only recipes:read.
func newBearerStack(t *testing.T) (http.Handler, string) {
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
	raw, _, err := tokens.Create(ctx, sam.ID, "reader", []string{auth.ScopeRecipesRead}, nil)
	if err != nil {
		t.Fatal(err)
	}
	cfg, _ := config.LoadFrom(map[string]string{})
	srv := httpserver.New(cfg, slog.New(slog.DiscardHandler), fstest.MapFS{},
		httpserver.WithAPIMiddleware(auth.Middleware(sessions, tokens, false)),
		httpserver.WithSecuritySchemes(auth.SecuritySchemes()))
	auth.Register(srv.API(), sessions, false)
	userapi.Register(srv.API(), users, sessions)
	return srv.Handler(), raw
}

func bearerReq(h http.Handler, method, path, token string) *httptest.ResponseRecorder { //nolint:unparam // helper mirrors brief signature; every current call site happens to use GET
	req := httptest.NewRequest(method, path, nil)
	req.Host = "localhost:8060"
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestBearerTokenWithoutTheScopeIsForbidden(t *testing.T) {
	h, raw := newBearerStack(t)
	// list-users declares users:read; the token only has recipes:read.
	if rec := bearerReq(h, http.MethodGet, "/api/v1/users", raw); rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 for a missing scope: %s", rec.Code, rec.Body.String())
	}
}

func TestBearerTokenIsRefusedOnSessionOnlyOperations(t *testing.T) {
	h, raw := newBearerStack(t)
	// GET /api/v1/auth/me keeps SessionSecurity: no token scheme at all.
	rec := bearerReq(h, http.MethodGet, "/api/v1/auth/me", raw)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 for an operation tokens cannot use: %s", rec.Code, rec.Body.String())
	}
}

func TestUnknownBearerTokenIsUnauthorized(t *testing.T) {
	h, _ := newBearerStack(t)
	rec := bearerReq(h, http.MethodGet, "/api/v1/users", auth.TokenPrefix+"nope")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401: %s", rec.Code, rec.Body.String())
	}
}

// TestBearerValueWithoutPrefixGetsAComprehensibleMessage covers a session
// cookie pasted as a bearer token by mistake: it must be told apart from a
// merely unknown token. TestRequireAuthValueWithoutPrefixGetsAComprehensibleMessage
// asserts the same message for the other bearer path (requireAuth).
func TestBearerValueWithoutPrefixGetsAComprehensibleMessage(t *testing.T) {
	h, _ := newBearerStack(t)
	rec := bearerReq(h, http.MethodGet, "/api/v1/users", "not-a-token-at-all")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401: %s", rec.Code, rec.Body.String())
	}
	want := `this is not an API token; API tokens begin with \"` + auth.TokenPrefix + `\"`
	if !strings.Contains(rec.Body.String(), want) {
		t.Fatalf("body = %s, want detail containing %q", rec.Body.String(), want)
	}
}

func TestSecuritySchemesAreDeclared(t *testing.T) {
	h, _ := newBearerStack(t)
	// The spec routes sit behind the session guard in production wiring, but
	// this stack installs none, so the document is readable here.
	rec := bearerReq(h, http.MethodGet, "/api/v1/openapi.json", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{`"session"`, `"token"`, `"bearer"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("openapi.json does not declare %s", want)
		}
	}
}
