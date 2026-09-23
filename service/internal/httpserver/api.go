package httpserver

import (
	_ "embed"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
)

const apiPrefix = "/api/v1"

// scalarTheme repaints the docs page in the app's colours. Scalar takes it
// as customCss and injects it as a style element; it cannot be served as a
// stylesheet, because huma renders the page itself and its CSP allows
// inline styles only (style-src 'unsafe-inline', no host).
//
//go:embed scalar_theme.css
var scalarTheme string

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

// apiTags declares the tags the operations refer to. huma.DefaultConfig
// declares none, and an operation's own Tags field does not create a
// declaration - so without this the document groups operations under names it
// never defines, the Scalar page at /api/v1/docs shows bare slugs instead of
// titled sections, and the generated reference in docs/user/ has nothing to
// build a page per tag from.
//
// The order is the order the documentation presents: what everything else
// needs first, then the recipe domain, then the administrative endpoints.
var apiTags = []*huma.Tag{
	{Name: "auth", Description: "Signing in, signing out and reading the current session."},
	{Name: "recipes", Description: "Creating, reading, updating and deleting recipes, and searching them."},
	{Name: "tags", Description: "The tags recipes are filed under."},
	{Name: "images", Description: "Uploading recipe images and serving them in their rendered sizes."},
	{Name: "users", Description: "Managing the accounts of an instance. Administrators only."},
	{Name: "tokens", Description: "Managing the API tokens a program authenticates with."},
}

// newAPI configures huma on the given mux. Operations register with full
// paths (e.g. /api/v1/recipes) so the mux and OpenAPI agree.
func newAPI(mux *http.ServeMux) huma.API {
	cfg := huma.DefaultConfig("Rezepte API", "0.1.0")
	cfg.Tags = apiTags
	cfg.OpenAPIPath = apiPrefix + "/openapi"
	cfg.DocsPath = apiPrefix + "/docs"
	cfg.SchemasPath = apiPrefix + "/schemas"
	cfg.DocsRenderer = huma.DocsRendererScalar
	cfg.DocsRendererConfig = map[string]any{
		"hideModels": true,
		"customCss":  scalarTheme,
		// Both buttons lead out of this page and into Scalar's own product -
		// the API client, and an assistant that would talk to their service
		// about our contract. The page is here to document the API, so
		// neither belongs on it.
		"hideClientButton": true,
		"agent":            map[string]any{"disabled": true},
	}
	return humago.New(mux, cfg)
}
