package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func testFS() fstest.MapFS {
	return fstest.MapFS{
		"index.html":                 {Data: []byte("<html>app</html>")},
		"_app/immutable/chunks/a.js": {Data: []byte("console.log(1)")},
		"favicon.svg":                {Data: []byte("<svg/>")},
		"manifest.webmanifest":       {Data: []byte("{}")},
	}
}

func TestSPAServesManifestAsManifestJSON(t *testing.T) {
	rec := get(t, SPAHandler(testFS()), "/manifest.webmanifest")
	if got := rec.Header().Get("Content-Type"); got != "application/manifest+json" {
		t.Fatalf("Content-Type = %q, want application/manifest+json", got)
	}
}

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestSPAServesExistingFile(t *testing.T) {
	rec := get(t, SPAHandler(testFS()), "/favicon.svg")
	if rec.Code != http.StatusOK || rec.Body.String() != "<svg/>" {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
}

func TestSPAImmutableAssetsAreCached(t *testing.T) {
	rec := get(t, SPAHandler(testFS()), "/_app/immutable/chunks/a.js")
	if got := rec.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Fatalf("Cache-Control = %q", got)
	}
}

func TestSPAFallsBackToIndex(t *testing.T) {
	rec := get(t, SPAHandler(testFS()), "/recipes/some-slug")
	if rec.Code != http.StatusOK || rec.Body.String() != "<html>app</html>" {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("Cache-Control = %q, want no-cache", got)
	}
}

func TestSPADoesNotFallBackForAPI(t *testing.T) {
	rec := get(t, SPAHandler(testFS()), "/api/v1/unknown")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("got %d, want 404", rec.Code)
	}
}

func TestSPAFallbackRejectsNonGet(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/recipes/x", nil)
	SPAHandler(testFS()).ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("got %d, want 405", rec.Code)
	}
	if got := rec.Header().Get("Allow"); got != "GET, HEAD" {
		t.Fatalf("Allow = %q, want %q", got, "GET, HEAD")
	}
}
