package httpserver

import (
	_ "embed"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
)

const apiPrefix = "/api/v1"

// scalarTheme repaints the docs page in the app's colors. Scalar takes it
// as customCss and injects it as a style element; it cannot be served as a
// stylesheet, because huma renders the page itself and its CSP allows
// inline styles only (style-src 'unsafe-inline', no host).
//
//go:embed scalar_theme.css
var scalarTheme string

// isSpecRoute reports whether p is one of the routes huma registers for the
// contract itself rather than for an operation: the docs page, the schema
// registry and the OpenAPI document in every flavor huma emits - 3.1 and
// the "-3.0" downgrades, each as .json and .yaml. Matching by prefix rather
// than listing the six paths keeps the guard correct if an upgrade adds
// another flavor; the cost is that an operation must never be registered
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
	{Name: "comments", Description: "Household members' comments on a recipe."},
	{Name: "tags", Description: "The tags recipes are filed under."},
	{Name: "images", Description: "Uploading recipe images and serving them in their rendered sizes."},
	{Name: "transfer", Description: "Exporting recipes with their photos as a zip and importing such a file. Administrators only."},
	{Name: "shares", Description: "Public links to recipes: creating, listing and revoking them. Session only."},
	{Name: "public", Description: "What anyone may read without signing in: the recipe and photos behind a public link, and the cover a link preview shows."},
	{Name: "settings", Description: "The household-wide settings. Everyone reads them; only the owner changes them."},
	{Name: "users", Description: "Managing the accounts of an instance, which is for administrators, and reading any account's picture."},
	{Name: "tokens", Description: "Managing the API tokens a program authenticates with."},
	// x-displayName: the reference in docs/user titles a tag by its name, capitalized.
	{Name: "mcp", Description: "The Model Context Protocol endpoint an AI agent connects to. API tokens only.", Extensions: map[string]any{"x-displayName": "MCP"}},
}

// defaultVersion is what a build without -X main.version reports, both on
// /healthz and as the document's info.version.
const defaultVersion = "dev"

// newAPI configures huma on the given mux. Operations register with full
// paths (e.g. /api/v1/recipes) so the mux and OpenAPI agree.
//
// The API has no version of its own: info.version is the build version, so
// it is set by New once WithVersion has run, not here.
func newAPI(mux *http.ServeMux) huma.API {
	cfg := huma.DefaultConfig("Rezepte API", defaultVersion)
	cfg.Tags = apiTags
	cfg.OpenAPIPath = apiPrefix + "/openapi"
	cfg.DocsPath = apiPrefix + "/docs"
	cfg.SchemasPath = apiPrefix + "/schemas"
	cfg.DocsRenderer = huma.DocsRendererScalar
	cfg.DocsRendererConfig = map[string]any{
		"hideModels": true,
		"customCss":  scalarTheme,
		// Name the operations by their route, the way the generated reference
		// in docs/user/ does. Scalar defaults to the summary, which reads well
		// in prose and badly in a list: two lines per entry, and no way to see
		// at a glance which endpoint one is looking at.
		"operationTitleSource": "path",
		// Both buttons lead out of this page and into Scalar's own product -
		// the API client, and an assistant that would talk to their service
		// about our contract. The page is here to document the API, so
		// neither belongs on it.
		"hideClientButton": true,
		"agent":            map[string]any{"disabled": true},
	}
	return humago.New(mux, cfg)
}
