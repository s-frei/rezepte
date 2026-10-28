package httpserver

import (
	"io/fs"
	"net/http"
)

// ManifestHandler serves the app manifest in the instance's locale: the
// generated manifest.<locale>.webmanifest when it exists, else the base
// manifest.webmanifest. The manifest is not per user, so the instance locale
// (not Accept-Language) decides, like the rest of the server-rendered text.
func ManifestHandler(fsys fs.FS, locale string) http.Handler {
	name := "manifest.webmanifest"
	if localized := "manifest." + locale + ".webmanifest"; locale != "" {
		if _, err := fs.Stat(fsys, localized); err == nil {
			name = localized
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/manifest+json")
		// A stale manifest keeps old shortcuts on installed apps.
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFileFS(w, r, fsys, name)
	})
}
