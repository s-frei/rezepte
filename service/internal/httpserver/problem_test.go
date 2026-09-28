package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestProblemBodies pins the exact bytes of the problem+json answers the
// server writes outside huma, so they stay identical to huma's own.
func TestProblemBodies(t *testing.T) {
	h := newTestServer(t).Handler()
	cases := []struct {
		method, path, origin string
		status               int
		body                 string
	}{
		{http.MethodPost, "/api/v1/x", "https://evil.example", 403, `{"title":"Forbidden","status":403,"detail":"origin not allowed"}` + "\n"},
		{http.MethodGet, "/api/v1/nope", "", 404, `{"title":"Not Found","status":404,"detail":"no such API route"}` + "\n"},
		{http.MethodGet, "/.well-known/oauth", "", 404, `{"title":"Not Found","status":404,"detail":"no such document"}` + "\n"},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		req.Host = "localhost:8060"
		if tc.origin != "" {
			req.Header.Set("Origin", tc.origin)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != tc.status || rec.Body.String() != tc.body {
			t.Errorf("%s %s = %d %q, want %d %q", tc.method, tc.path, rec.Code, rec.Body.String(), tc.status, tc.body)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
			t.Errorf("%s %s: Content-Type = %q", tc.method, tc.path, ct)
		}
	}
}
