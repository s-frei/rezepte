package httpserver

import (
	"bytes"
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

func TestUnknownAPIRouteIsProblemJSON(t *testing.T) {
	for _, path := range []string{"/api/v1/does-not-exist", "/api"} {
		t.Run(path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			newTestServer(t).Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
			if rec.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404", rec.Code)
			}
			if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/problem+json") {
				t.Fatalf("Content-Type = %q, want application/problem+json prefix", ct)
			}
			if !strings.Contains(rec.Body.String(), `"status":404`) {
				t.Fatalf("body = %s, want it to contain \"status\":404", rec.Body.String())
			}
		})
	}
}

func TestLogRequestsLevelByStatus(t *testing.T) {
	cfg, err := config.LoadFrom(map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	static := fstest.MapFS{"index.html": {Data: []byte("app")}}

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	handler := New(cfg, logger, static).Handler()

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/nope", nil))
	out := buf.String()
	if !strings.Contains(out, "level=WARN") || !strings.Contains(out, "status=404") {
		t.Fatalf("log output = %q, want a WARN line with status=404", out)
	}

	buf.Reset()
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if buf.Len() != 0 {
		t.Fatalf("log output = %q, want no line for a healthy 200 request at Warn level", buf.String())
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
