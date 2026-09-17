package httpserver

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/danielgtaylor/huma/v2"
)

// checkOrigin rejects mutating API requests whose Origin header names a
// different host. Together with SameSite=Lax cookies this blocks CSRF.
// Requests without an Origin header (non-browser clients) pass through.
func checkOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isMutating(r.Method) && strings.HasPrefix(r.URL.Path, "/api/") {
			if origin := r.Header.Get("Origin"); origin != "" {
				u, err := url.Parse(origin)
				if err != nil || !strings.EqualFold(u.Host, r.Host) {
					writeForbiddenOrigin(w)
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

// writeForbiddenOrigin answers a rejected cross-origin request with an
// RFC 9457 problem+json body, matching every other error response the API
// returns.
func writeForbiddenOrigin(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(huma.ErrorModel{
		Title:  "Forbidden",
		Status: http.StatusForbidden,
		Detail: "origin not allowed",
	})
}

func isMutating(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	}
	return false
}
