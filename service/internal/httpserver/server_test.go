package httpserver

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/danielgtaylor/huma/v2"

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

func TestServerErrorsHideDetails(t *testing.T) {
	cfg, err := config.LoadFrom(map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	srv := New(cfg, logger, fstest.MapFS{})

	huma.Register(srv.API(), huma.Operation{
		OperationID: "boom",
		Method:      http.MethodGet,
		Path:        "/api/v1/boom",
	}, func(context.Context, *struct{}) (*struct{}, error) {
		return nil, errors.New("secret database detail")
	})

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/boom", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/problem+json") {
		t.Fatalf("Content-Type = %q, want application/problem+json prefix", ct)
	}
	if strings.Contains(rec.Body.String(), "secret") {
		t.Fatalf("body leaked internal error detail: %s", rec.Body.String())
	}
	logged := buf.String()
	if !strings.Contains(logged, "secret") {
		t.Fatalf("logger did not record the internal error: %s", logged)
	}
	if !strings.Contains(logged, "operation=boom") {
		t.Fatalf("logger did not record the operation id: %s", logged)
	}
	if !strings.Contains(logged, "method=GET") || !strings.Contains(logged, "path=/api/v1/boom") {
		t.Fatalf("logger did not record method/path: %s", logged)
	}
}

// TestWithAPIMiddlewareAppliesBeforeRegistration proves that an Option
// passed to New applies before the caller can register any operation: a
// middleware that rejects every protected operation still rejects an
// operation registered strictly after New returns.
func TestWithAPIMiddlewareAppliesBeforeRegistration(t *testing.T) {
	cfg, err := config.LoadFrom(map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	rejectProtected := func(api huma.API) func(huma.Context, func(huma.Context)) {
		return func(ctx huma.Context, next func(huma.Context)) {
			if len(ctx.Operation().Security) > 0 {
				_ = huma.WriteErr(api, ctx, http.StatusUnauthorized, "rejected by test middleware")
				return
			}
			next(ctx)
		}
	}
	srv := New(cfg, slog.New(slog.DiscardHandler), fstest.MapFS{}, WithAPIMiddleware(rejectProtected))

	// Registered after New returns - if the middleware were not already
	// installed by this point, this operation would run unauthenticated.
	huma.Register(srv.API(), huma.Operation{
		OperationID: "protected",
		Method:      http.MethodGet,
		Path:        "/api/v1/protected",
		Security:    []map[string][]string{{"session": {}}},
	}, func(context.Context, *struct{}) (*struct{}, error) {
		return &struct{}{}, nil
	})

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/protected", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (WithAPIMiddleware must apply before an operation can be registered)", rec.Code)
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

func TestHandleRegistersMuxRouteAheadOfSPA(t *testing.T) {
	srv := newTestServer(t)
	srv.Handle("GET /images/{recipeId}/{imageId}/{file}", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(r.PathValue("recipeId") + "/" + r.PathValue("file")))
	}))
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/images/r1/i1/thumb.jpg", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "r1/thumb.jpg" {
		t.Fatalf("got %d %q, want the mux route not the SPA", rec.Code, rec.Body.String())
	}
	// A shorter path under /images/ still falls through to the SPA.
	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/images/r1", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "app" {
		t.Fatalf("got %d %q, want SPA fallback", rec.Code, rec.Body.String())
	}
}
