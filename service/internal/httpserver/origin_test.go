package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCheckOrigin(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	h := checkOrigin(ok)

	cases := []struct {
		name   string
		method string
		path   string
		origin string
		want   int
	}{
		{"get without origin", http.MethodGet, "/api/v1/x", "", 200},
		{"post without origin (non-browser)", http.MethodPost, "/api/v1/x", "", 200},
		{"post same origin", http.MethodPost, "/api/v1/x", "http://localhost:8060", 200},
		{"post foreign origin", http.MethodPost, "/api/v1/x", "https://evil.example", 403},
		{"delete foreign origin", http.MethodDelete, "/api/v1/x", "https://evil.example", 403},
		{"get foreign origin is fine", http.MethodGet, "/api/v1/x", "https://evil.example", 200},
		{"post foreign origin outside api", http.MethodPost, "/healthz", "https://evil.example", 200},
		{"mcp post foreign origin", http.MethodPost, "/mcp", "https://evil.example", 403},
		{"mcp get foreign origin", http.MethodGet, "/mcp", "https://evil.example", 403},
		{"mcp delete foreign origin", http.MethodDelete, "/mcp", "https://evil.example", 403},
		{"mcp malformed origin", http.MethodPost, "/mcp", "://", 403},
		{"mcp post same origin", http.MethodPost, "/mcp", "http://localhost:8060", 200},
		{"mcp post without origin", http.MethodPost, "/mcp", "", 200},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.Host = "localhost:8060"
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("got %d, want %d", rec.Code, tc.want)
			}
			if tc.want == http.StatusForbidden {
				if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/problem+json") {
					t.Fatalf("Content-Type = %q, want application/problem+json prefix", ct)
				}
				if !strings.Contains(rec.Body.String(), `"status":403`) {
					t.Fatalf("body = %s, want it to contain \"status\":403", rec.Body.String())
				}
			}
		})
	}
}
