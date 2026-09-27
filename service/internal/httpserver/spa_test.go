package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
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
	rec := get(t, SPAHandler(testFS(), false, nil), "/manifest.webmanifest")
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
	rec := get(t, SPAHandler(testFS(), false, nil), "/favicon.svg")
	if rec.Code != http.StatusOK || rec.Body.String() != "<svg/>" {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
}

func TestSPAImmutableAssetsAreCached(t *testing.T) {
	rec := get(t, SPAHandler(testFS(), false, nil), "/_app/immutable/chunks/a.js")
	if got := rec.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Fatalf("Cache-Control = %q", got)
	}
}

func TestSPAFallsBackToIndex(t *testing.T) {
	rec := get(t, SPAHandler(testFS(), false, nil), "/recipes/some-slug")
	if rec.Code != http.StatusOK || rec.Body.String() != "<html>app</html>" {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("Cache-Control = %q, want no-cache", got)
	}
}

func TestSPADoesNotFallBackForAPI(t *testing.T) {
	rec := get(t, SPAHandler(testFS(), false, nil), "/api/v1/unknown")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("got %d, want 404", rec.Code)
	}
}

func TestSPAFallbackRejectsNonGet(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/recipes/x", nil)
	SPAHandler(testFS(), false, nil).ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("got %d, want 405", rec.Code)
	}
	if got := rec.Header().Get("Allow"); got != "GET, HEAD" {
		t.Fatalf("Allow = %q, want %q", got, "GET, HEAD")
	}
}

// shell is the link preview block of app.html as the build writes it.
const shell = `<meta property="og:url" content="/" />
<meta property="og:image" content="/og.png" />
<meta property="og:image:width" content="1280" />
<meta name="twitter:image" content="/og.png" />`

func shellFS() fstest.MapFS {
	return fstest.MapFS{"index.html": {Data: []byte(shell)}}
}

func serveShellFor(t *testing.T, secure bool, host, path string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Host = host
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	SPAHandler(shellFS(), secure, nil).ServeHTTP(rec, req)
	return rec
}

func TestSPAShellMakesTheLinkPreviewAbsolute(t *testing.T) {
	cases := []struct {
		name    string
		secure  bool
		path    string
		headers map[string]string
		origin  string
		page    string
	}{
		{"plain http", false, "/recipes/x", nil, "http://rezepte.example", "/recipes/x"},
		{"secure cookies", true, "/recipes/x?tab=steps", nil, "https://rezepte.example", "/recipes/x"},
		{"forwarded https", false, "/", map[string]string{"X-Forwarded-Proto": "https"}, "https://rezepte.example", "/"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := serveShellFor(t, tc.secure, "rezepte.example", tc.path, tc.headers)
			want := `<meta property="og:url" content="` + tc.origin + tc.page + `" />
<meta property="og:image" content="` + tc.origin + `/og.png" />
<meta property="og:image:width" content="1280" />
<meta name="twitter:image" content="` + tc.origin + `/og.png" />`
			if rec.Code != http.StatusOK || rec.Body.String() != want {
				t.Fatalf("got %d\n%s\nwant\n%s", rec.Code, rec.Body.String(), want)
			}
			if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
				t.Fatalf("Content-Type = %q", got)
			}
		})
	}
}

func TestSPAShellEscapesTheHost(t *testing.T) {
	rec := serveShellFor(t, false, `evil"><script>`, "/", nil)
	if strings.Contains(rec.Body.String(), "<script>") {
		t.Fatalf("host reached the page unescaped:\n%s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `content="http://evil&#34;&gt;&lt;script&gt;/og.png"`) {
		t.Fatalf("og:image not escaped:\n%s", rec.Body.String())
	}
}

func TestSPAShellAnswersHead(t *testing.T) {
	req := httptest.NewRequest(http.MethodHead, "/recipes/x", nil)
	rec := httptest.NewRecorder()
	SPAHandler(shellFS(), false, nil).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.Len() != 0 {
		t.Fatalf("got %d with %d bytes, want 200 and no body", rec.Code, rec.Body.Len())
	}
}

func TestSPAShellShowsAPagesOwnLinkPreview(t *testing.T) {
	fsys := fstest.MapFS{"index.html": {Data: []byte(`<title>Rezepte</title>
<meta property="og:title" content="Rezepte" />
<meta property="og:description" content="A recipe manager" />
<meta property="og:url" content="/" />
<meta property="og:image" content="/og.png" />
<meta property="og:image:type" content="image/png" />
<meta property="og:image:width" content="1280" />
<meta property="og:image:height" content="640" />
<meta
	property="og:image:alt"
	content="The Rezepte logo"
/>
<meta name="twitter:image" content="/og.png" />
<meta name="twitter:image:alt" content="The Rezepte logo" />`)}}
	preview := func(r *http.Request) *LinkPreview {
		if r.URL.Path != "/recipes/salat" {
			return nil
		}
		return &LinkPreview{
			URL: "/recipes/salat?share=t", Title: `Salat "scharf" & kalt`, Description: "Frisch.",
			Image: "/link-preview/r/i?share=t", ImageWidth: 960, ImageHeight: 640,
			ImageType: "image/jpeg", ImageAlt: "Salat",
		}
	}
	req := httptest.NewRequest(http.MethodGet, "/recipes/salat?share=t", nil)
	req.Host = "rezepte.example"
	rec := httptest.NewRecorder()
	SPAHandler(fsys, true, preview).ServeHTTP(rec, req)
	want := `<title>Salat &#34;scharf&#34; &amp; kalt · Rezepte</title>
<meta property="og:title" content="Salat &#34;scharf&#34; &amp; kalt" />
<meta property="og:description" content="Frisch." />
<meta property="og:url" content="https://rezepte.example/recipes/salat?share=t" />
<meta property="og:image" content="https://rezepte.example/link-preview/r/i?share=t" />
<meta property="og:image:type" content="image/jpeg" />
<meta property="og:image:width" content="960" />
<meta property="og:image:height" content="640" />
<meta
	property="og:image:alt"
	content="Salat"
/>
<meta name="twitter:image" content="https://rezepte.example/link-preview/r/i?share=t" />
<meta name="twitter:image:alt" content="Salat" />`
	if got := rec.Body.String(); got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "rezepte.example"
	rec = httptest.NewRecorder()
	SPAHandler(fsys, true, preview).ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), `<meta property="og:title" content="Rezepte" />`) ||
		!strings.Contains(rec.Body.String(), `<meta property="og:image" content="https://rezepte.example/og.png" />`) {
		t.Fatalf("app preview changed:\n%s", rec.Body.String())
	}
}
