package httpserver

import (
	"net/http"
	"net/url"
	"strings"
)

// checkOrigin rejects mutating API requests, and every request to the MCP
// endpoint, whose Origin header names a different host. Together with
// SameSite=Lax cookies this blocks CSRF; on /mcp it is the Origin check the
// MCP Streamable HTTP transport requires of every server, whatever the
// method. Requests without an Origin header (non-browser clients) pass
// through.
func checkOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/mcp" || (isMutating(r.Method) && strings.HasPrefix(r.URL.Path, "/api/")) {
			if origin := r.Header.Get("Origin"); origin != "" {
				u, err := url.Parse(origin)
				if err != nil || !strings.EqualFold(u.Host, r.Host) {
					WriteProblem(w, http.StatusForbidden, "origin not allowed")
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

func isMutating(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	}
	return false
}
