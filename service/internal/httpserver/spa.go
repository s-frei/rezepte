package httpserver

import (
	"bytes"
	"fmt"
	"html"
	"io/fs"
	"mime"
	"net/http"
	"regexp"
	"slices"
	"strconv"
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

// LinkPreview is what a shared link's preview shows for one page in place of
// the app's own title, description and picture. Paths in URL and Image are
// made absolute for the request's origin.
type LinkPreview struct {
	// URL is the page's path and query, which becomes og:url: Facebook and
	// LinkedIn scrape og:url again, so it must be the address that yields
	// this preview.
	URL         string
	Title       string
	Description string
	// Image is empty to keep the app's own picture.
	Image                   string
	ImageWidth, ImageHeight int
	ImageType, ImageAlt     string
}

// PreviewFunc returns the link preview for the page a request asks for, or
// nil for the app's own.
type PreviewFunc func(r *http.Request) *LinkPreview

// SPAHandler serves files from fsys. Requests for paths that do not exist
// receive index.html so client-side routing works. API paths never fall back.
// secure says the instance is reached over HTTPS (REZEPTE_SECURE_COOKIES),
// which the app shell's link preview needs to name its URLs absolutely.
// preview, when not nil, supplies a page's own link preview.
func SPAHandler(fsys fs.FS, secure bool, preview PreviewFunc) http.Handler {
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
		var p *LinkPreview
		if preview != nil {
			p = preview(r)
		}
		serveShell(w, r, fsys, requestOrigin(r, secure), p)
	})
}

// serveShell answers with index.html, its link preview filled in for the
// page and made absolute for origin.
func serveShell(w http.ResponseWriter, r *http.Request, fsys fs.FS, origin string, p *LinkPreview) {
	page, err := fs.ReadFile(fsys, "index.html")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	page = renderLinkPreview(page, origin, r.URL.EscapedPath(), p)
	http.ServeContent(w, r, "index.html", time.Time{}, bytes.NewReader(page))
}

// previewKeys are the tags app.html carries for a link preview, the ones
// renderLinkPreview may rewrite.
var previewKeys = []string{
	"og:url", "og:title", "og:description",
	"og:image", "og:image:type", "og:image:width", "og:image:height", "og:image:alt",
	"twitter:image", "twitter:image:alt",
}

// metaContent matches a tag's content attribute, keyed by its property or
// name; group 1 is the value.
var metaContent = func() map[string]*regexp.Regexp {
	m := make(map[string]*regexp.Regexp, len(previewKeys))
	for _, key := range previewKeys {
		m[key] = regexp.MustCompile(`(?:property|name)="` + regexp.QuoteMeta(key) + `"\s+content="([^"]*)"`)
	}
	return m
}()

var titleTag = regexp.MustCompile(`<title>[^<]*</title>`)

// renderLinkPreview fills the link preview tags app.html carries: p's title,
// description and picture when there is one, and absolute URLs either way.
// WhatsApp, Signal, Slack, Facebook and most other link-preview crawlers
// ignore a relative URL, and the build can only write paths.
func renderLinkPreview(page []byte, origin, path string, p *LinkPreview) []byte {
	url := path
	image := getMeta(page, "og:image")
	if p != nil {
		url = p.URL
		page = setMeta(page, "og:title", p.Title)
		page = setMeta(page, "og:description", p.Description)
		page = titleTag.ReplaceAllLiteral(page, []byte("<title>"+html.EscapeString(p.Title)+" · Rezepte</title>"))
		if p.Image != "" {
			image = p.Image
			page = setMeta(page, "og:image:type", p.ImageType)
			page = setMeta(page, "og:image:width", strconv.Itoa(p.ImageWidth))
			page = setMeta(page, "og:image:height", strconv.Itoa(p.ImageHeight))
			page = setMeta(page, "og:image:alt", p.ImageAlt)
			page = setMeta(page, "twitter:image:alt", p.ImageAlt)
		}
	}
	if strings.HasPrefix(image, "/") {
		image = origin + image
	}
	page = setMeta(page, "og:url", origin+url)
	page = setMeta(page, "og:image", image)
	return setMeta(page, "twitter:image", image)
}

// getMeta returns the content of the tag key names, unescaped.
func getMeta(page []byte, key string) string {
	m := metaContent[key].FindSubmatch(page)
	if m == nil {
		return ""
	}
	return html.UnescapeString(string(m[1]))
}

// setMeta replaces the content of the tag key names with value, escaped. A
// page without that tag is returned unchanged.
func setMeta(page []byte, key, value string) []byte {
	loc := metaContent[key].FindSubmatchIndex(page)
	if loc == nil {
		return page
	}
	return slices.Concat(page[:loc[2]], []byte(html.EscapeString(value)), page[loc[3]:])
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
