package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/s-frei/rezepte/service/internal/auth"
)

// TestProblemBodies pins the exact bytes of the 401 and 403 problem+json
// answers the mux middleware writes outside huma.
func TestProblemBodies(t *testing.T) {
	env := newRequireEnv(t)
	h := auth.RequireToken(env.tokens, auth.ScopeRecipesRead, auth.ScopeRecipesWrite)(okHandler())
	usersOnly := env.issue(t, auth.ScopeUsersRead)
	cases := []struct {
		name, authz string
		status      int
		body        string
	}{
		{"no token", "", 401, `{"title":"Unauthorized","status":401,"detail":"api token required"}` + "\n"},
		{"missing scope", "Bearer " + usersOnly, 403, `{"title":"Forbidden","status":403,"detail":"api token is missing scope recipes:read, recipes:write"}` + "\n"},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
		if tc.authz != "" {
			req.Header.Set("Authorization", tc.authz)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != tc.status || rec.Body.String() != tc.body {
			t.Errorf("%s = %d %q, want %d %q", tc.name, rec.Code, rec.Body.String(), tc.status, tc.body)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
			t.Errorf("%s: Content-Type = %q", tc.name, ct)
		}
	}
}
