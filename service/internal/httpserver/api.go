package httpserver

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
)

const apiPrefix = "/api/v1"

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
