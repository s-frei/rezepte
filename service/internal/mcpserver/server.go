package mcpserver

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/recipe"
	"github.com/s-frei/rezepte/service/internal/user"
)

// caller is what a tool handler works with: the service, the token owner it
// acts as, and the recipe schemas built once at start-up. user is the whole
// account, not just its id, because editing rights read its role.
type caller struct {
	svc    *recipe.Service
	user   user.User
	create *jsonschema.Schema
	update *jsonschema.Schema
	search *jsonschema.Schema
}

// resolveAsAddTool resolves s the way mcp.AddTool does. AddTool resolves a
// tool's InputSchema when the tool is registered and panics if that fails;
// Handler registers tools per request, so doing it once at start-up with the
// same options turns a broken schema into a boot error instead of a panic on
// the first request that registers the tool.
func resolveAsAddTool(s *jsonschema.Schema) error {
	// The options mcp.AddTool passes (go-sdk mcp/server.go, setSchema).
	if _, err := s.Resolve(&jsonschema.ResolveOptions{ValidateDefaults: true}); err != nil {
		return fmt.Errorf("resolve: %w", err)
	}
	return nil
}

// tool pairs a tool with the scope a token needs to be offered it.
type tool struct {
	scope string
	add   func(s *mcp.Server, c caller)
}

// tools is the scope table. A request's server is built with exactly the
// entries its token covers, so tools/list shows only what the token may call
// and a call to anything else fails as an unknown tool - the list and the
// check cannot disagree, because they are the same registration.
var tools []tool

// Register mounts Handler at /mcp as huma operations marked auth.TokenOnly
// with auth.ScopeRecipesRead, so a token with no recipe scope never gets a
// server. None of them declares a body, so huma leaves the request body to
// the SDK (verified in v2.39.1: processInputType leaves hasInputBody false).
// Only POST is documented: the handler is stateless, so GET and DELETE
// answer 405 once authenticated. They are registered hidden all the same,
// so they are refused like POST instead of falling through to the SPA.
// api must already have recipe.Register applied.
func Register(api huma.API, svc *recipe.Service, version string) error {
	h, err := Handler(svc, api, version)
	if err != nil {
		return err
	}
	serve := func(context.Context, *struct{}) (*huma.StreamResponse, error) {
		return &huma.StreamResponse{Body: func(ctx huma.Context) {
			r, w := humago.Unwrap(ctx)
			h.ServeHTTP(w, r)
		}}, nil
	}
	huma.Register(api, huma.Operation{
		OperationID: "mcp",
		Method:      http.MethodPost,
		Path:        "/mcp",
		Summary:     "Talk to Rezepte over the Model Context Protocol",
		Description: "The endpoint an MCP client such as an AI agent connects to: stateless Streamable HTTP, one JSON-RPC message per request, answered as JSON. The tools describe themselves over tools/list; the token's scopes decide which ones it lists. See https://modelcontextprotocol.io/specification.",
		Tags:        []string{"mcp"},
		Security:    auth.TokenOnly(auth.ScopeRecipesRead),
		Errors:      []int{403},
		Responses: map[string]*huma.Response{"200": {
			Description: "The JSON-RPC response",
			Content:     map[string]*huma.MediaType{"application/json": {Schema: &huma.Schema{Type: "object"}}},
		}},
	}, serve)
	for _, method := range []string{http.MethodGet, http.MethodDelete} {
		huma.Register(api, huma.Operation{
			OperationID: "mcp-" + strings.ToLower(method),
			Method:      method,
			Path:        "/mcp",
			Security:    auth.TokenOnly(auth.ScopeRecipesRead),
			Hidden:      true,
		}, serve)
	}
	return nil
}

// Handler serves MCP over stateless Streamable HTTP; Register mounts it. It
// reads the caller from auth.UserFrom and the token's scopes from
// auth.ScopesFrom. api must already have recipe.Register applied.
func Handler(svc *recipe.Service, api huma.API, version string) (http.Handler, error) {
	create, err := recipeSchema(api, false)
	if err != nil {
		return nil, fmt.Errorf("create_recipe schema: %w", err)
	}
	update, err := recipeSchema(api, true)
	if err != nil {
		return nil, fmt.Errorf("update_recipe schema: %w", err)
	}
	search, err := searchSchema()
	if err != nil {
		return nil, err
	}
	for name, s := range map[string]*jsonschema.Schema{"create_recipe": create, "update_recipe": update, "search_recipes": search} {
		if err := resolveAsAddTool(s); err != nil {
			return nil, fmt.Errorf("resolve %s schema: %w", name, err)
		}
	}
	cache := mcp.NewSchemaCache()
	impl := &mcp.Implementation{Name: "rezepte", Version: version}
	return mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		u, ok := auth.UserFrom(r.Context())
		if !ok {
			// Not behind auth.TokenOnly: there is no one to act as. The SDK
			// answers a nil server with 400.
			return nil
		}
		scopes := auth.ScopesFrom(r.Context())
		s := mcp.NewServer(impl, &mcp.ServerOptions{SchemaCache: cache})
		c := caller{svc: svc, user: u, create: create, update: update, search: search}
		for _, t := range tools {
			if slices.Contains(scopes, t.scope) {
				t.add(s, c)
			}
		}
		return s
	}, &mcp.StreamableHTTPOptions{
		Stateless:    true,
		JSONResponse: true,
		Logger:       slog.Default(),
		// The SDK's DNS-rebinding guard protects unauthenticated local MCP
		// servers. /mcp always requires a bearer token (auth.TokenOnly),
		// which a rebinding page can neither know nor send, while the guard
		// would 403 every request from a reverse proxy on the same host: a
		// loopback local address carrying the public Host header.
		DisableLocalhostProtection: true,
	}), nil
}

// fill sets r's caller-dependent fields for the token owner.
func (c caller) fill(ctx context.Context, r *recipe.Recipe) error {
	return c.svc.FillCaller(ctx, c.user, r)
}
