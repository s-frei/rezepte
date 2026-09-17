package image

import (
	"net/http"
	"strings"
)

// immutableCache suits the files: an image id is never reused and a file
// never changes after it is written, so a browser may cache it for a year.
// "private" keeps shared caches out - the route is behind a session.
const immutableCache = "private, max-age=31536000, immutable"

// FileHandler serves GET /images/{recipeId}/{imageId}/{file} where file is
// "thumb.jpg", "detail.jpg" or "original.jpg". It is a plain mux handler
// (not a huma operation, whose JSON framing does not fit a JPEG stream);
// register it with httpserver.Server.Handle wrapped in auth.RequireSession.
// Anything but an existing variant is a 404; http.ServeContent adds Range
// and conditional-request support.
func FileHandler(svc *Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		variant, ok := strings.CutSuffix(r.PathValue("file"), ".jpg")
		if !ok {
			http.NotFound(w, r)
			return
		}
		f, err := svc.Open(r.PathValue("recipeId"), r.PathValue("imageId"), variant)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Cache-Control", immutableCache)
		http.ServeContent(w, r, "", info.ModTime(), f)
	})
}
