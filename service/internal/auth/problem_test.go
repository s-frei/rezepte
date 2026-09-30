package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/s-frei/rezepte/service/internal/auth"
)

// TestProblemBodies pins the exact bytes of the 401 problem+json answer
// RequireAuthOrLogin writes outside huma.
func TestProblemBodies(t *testing.T) {
	env := newRequireEnv(t)
	h := auth.RequireAuthOrLogin(env.sessions, env.tokens, false)(okHandler())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/openapi.json", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	want := `{"title":"Unauthorized","status":401,"detail":"authentication required"}` + "\n"
	if rec.Code != http.StatusUnauthorized || rec.Body.String() != want {
		t.Errorf("= %d %q, want 401 %q", rec.Code, rec.Body.String(), want)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("Content-Type = %q", ct)
	}
}
