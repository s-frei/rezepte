package httpserver

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/s-frei/rezepte/service/internal/config"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	cfg, err := config.LoadFrom(map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	static := fstest.MapFS{"index.html": {Data: []byte("app")}}
	return New(cfg, slog.New(slog.DiscardHandler), static)
}

func TestHealthz(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestServer(t).Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != "ok" {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
}

func TestOpenAPIIsServed(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestServer(t).Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/openapi.json", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"title":"Rezepte API"`) {
		t.Fatalf("body does not contain API title: %s", rec.Body.String())
	}
}

func TestDocsIsServed(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestServer(t).Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/docs", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "scalar") {
		t.Fatalf("got %d, body contains scalar: %v", rec.Code, strings.Contains(rec.Body.String(), "scalar"))
	}
}

func TestSPAFallbackIsWired(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestServer(t).Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/recipes/x", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "app" {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
}

func TestStatusRecorderUnwrapsForResponseController(t *testing.T) {
	inner := httptest.NewRecorder()
	rec := &statusRecorder{ResponseWriter: inner, status: http.StatusOK}
	if rec.Unwrap() != inner {
		t.Fatalf("Unwrap() = %v, want %v", rec.Unwrap(), inner)
	}
	if err := http.NewResponseController(rec).Flush(); err != nil {
		t.Fatalf("Flush() via ResponseController: %v", err)
	}
}
