package httpserver

import (
	"bytes"
	"fmt"
	"html"
	"io/fs"
	"mime"
	"net/http"
	"strings"
	"time"
)

const immutableCache = "public, max-age=31536000, immutable"

func init() {
	// Go's built-in table has no .webmanifest, and the distroless image
	// ships no /etc/mime.types, so without this the manifest would go out
	// as sniffed text/plain.
	if err := mime.AddExtensionType(".webmanifest", "application/manifest+json"); err != nil {
		panic(fmt.Errorf("register webmanifest type: %w", err))
	}
}

// SPAHandler serves files from fsys. Requests for paths that do not exist
// receive index.html so client-side routing works. API paths never fall back.
// secure says the instance is reached over HTTPS (REZEPTE_SECURE_COOKIES),
// which the app shell's link preview needs to name its image absolutely.
func SPAHandler(fsys fs.FS, secure bool) http.Handler {
	files := http.FileServerFS(fsys)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name != "" && exists(fsys, name) {
			if strings.HasPrefix(name, "_app/immutable/") {
				w.Header().Set("Cache-Control", immutableCache)
			}
			files.ServeHTTP(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		serveShell(w, r, fsys, secure)
	})
}

// serveShell answers with index.html, its link preview made absolute for the
// URL the request came in on.
func serveShell(w http.ResponseWriter, r *http.Request, fsys fs.FS, secure bool) {
	page, err := fs.ReadFile(fsys, "index.html")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	page = absoluteLinkPreview(page, requestOrigin(r, secure), r.URL.EscapedPath())
	http.ServeContent(w, r, "index.html", time.Time{}, bytes.NewReader(page))
}

// absoluteLinkPreview rewrites the link preview tags app.html writes as paths
// into absolute URLs: og:url becomes the page's own address, og:image and
// twitter:image are prefixed with origin. WhatsApp, Signal, Slack, Facebook
// and most other link-preview crawlers ignore a relative URL, so the paths
// the build writes would give them a card without a picture.
func absoluteLinkPreview(page []byte, origin, path string) []byte {
	page = bytes.Replace(page,
		[]byte(`property="og:url" content="/"`),
		[]byte(`property="og:url" content="`+html.EscapeString(origin+path)+`"`), 1)
	for _, key := range []string{`property="og:image"`, `name="twitter:image"`} {
		page = bytes.Replace(page,
			[]byte(key+` content="/`),
			[]byte(key+` content="`+html.EscapeString(origin)+`/`), 1)
	}
	return page
}

// requestOrigin is the public origin the request was addressed to. The host
// is the request's Host, which the reverse proxy must pass through unchanged
// anyway for the Origin check (see origin.go). The scheme is https when the
// instance says it is served over HTTPS, when the connection is TLS, or when
// the proxy says so in X-Forwarded-Proto; a forged header only changes the
// forger's own response.
func requestOrigin(r *http.Request, secure bool) string {
	scheme := "http"
	if secure || r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

func exists(fsys fs.FS, name string) bool {
	info, err := fs.Stat(fsys, name)
	return err == nil && !info.IsDir()
}
