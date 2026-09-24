package httpserver

import (
	"fmt"
	"io/fs"
	"mime"
	"net/http"
	"strings"
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
func SPAHandler(fsys fs.FS) http.Handler {
	files := http.FileServerFS(fsys)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name == "" {
			name = "index.html"
		}
		if exists(fsys, name) {
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
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFileFS(w, r, fsys, "index.html")
	})
}

func exists(fsys fs.FS, name string) bool {
	info, err := fs.Stat(fsys, name)
	return err == nil && !info.IsDir()
}
