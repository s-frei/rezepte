package httpserver

import (
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
)

const apiPrefix = "/api/v1"

// isSpecRoute reports whether p is one of the routes huma registers for the
// contract itself rather than for an operation: the docs page, the schema
// registry and the OpenAPI document in every flavour huma emits - 3.1 and
// the "-3.0" downgrades, each as .json and .yaml. Matching by prefix rather
// than listing the six paths keeps the guard correct if an upgrade adds
// another flavour; the cost is that an operation must never be registered
// at a path starting with /api/v1/openapi or /api/v1/schemas/.
func isSpecRoute(p string) bool {
	return p == apiPrefix+"/docs" ||
		strings.HasPrefix(p, apiPrefix+"/openapi") ||
		strings.HasPrefix(p, apiPrefix+"/schemas/")
}

// newAPI configures huma on the given mux. Operations register with full
// paths (e.g. /api/v1/recipes) so the mux and OpenAPI agree.
func newAPI(mux *http.ServeMux) huma.API {
	cfg := huma.DefaultConfig("Rezepte API", "0.1.0")
	cfg.OpenAPIPath = apiPrefix + "/openapi"
	cfg.DocsPath = apiPrefix + "/docs"
	cfg.SchemasPath = apiPrefix + "/schemas"
	cfg.DocsRenderer = huma.DocsRendererScalar
	cfg.DocsRendererConfig = map[string]any{"hideModels": true}
	return humago.New(mux, cfg)
}
