package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestManifestHandlerServesTheInstanceLocale(t *testing.T) {
	fsys := fstest.MapFS{
		"manifest.webmanifest":    {Data: []byte(`{"lang":"en"}`)},
		"manifest.de.webmanifest": {Data: []byte(`{"lang":"de"}`)},
	}
	for locale, want := range map[string]string{"de": `"lang":"de"`, "en": `"lang":"en"`, "xx": `"lang":"en"`} {
		rec := httptest.NewRecorder()
		ManifestHandler(fsys, locale).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/manifest.webmanifest", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status = %d", locale, rec.Code)
		}
		if got := rec.Header().Get("Content-Type"); got != "application/manifest+json" {
			t.Errorf("%s: Content-Type = %q", locale, got)
		}
		if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
			t.Errorf("%s: Cache-Control = %q", locale, got)
		}
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("%s: body = %q, want %s", locale, rec.Body.String(), want)
		}
	}
}
