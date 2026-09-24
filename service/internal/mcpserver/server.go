package mcpserver

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"slices"

	"github.com/danielgtaylor/huma/v2"
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

// Handler serves MCP over stateless Streamable HTTP. Wrap it in
// auth.RequireToken: it reads the caller from auth.UserFrom and the token's
// scopes from auth.ScopesFrom. api must already have recipe.Register applied.
func Handler(svc *recipe.Service, api huma.API, version string) (http.Handler, error) {
	create, err := recipeInputSchema(api)
	if err != nil {
		return nil, fmt.Errorf("create_recipe schema: %w", err)
	}
	update, err := recipeUpdateSchema(api)
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
			// Not behind RequireToken: there is no one to act as. The SDK
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
		// servers. /mcp always requires a bearer token (auth.RequireToken),
		// which a rebinding page can neither know nor send, while the guard
		// would 403 every request from a reverse proxy on the same host: a
		// loopback local address carrying the public Host header.
		DisableLocalhostProtection: true,
	}), nil
}

// fill sets r's caller-dependent fields for the token owner - the favorite
// star and what they may do with it - the way REST's fillCaller does.
func (c caller) fill(ctx context.Context, r *recipe.Recipe) error {
	fav, err := c.svc.IsFavorite(ctx, c.user.ID, r.ID)
	if err != nil {
		return fmt.Errorf("read favorite: %w", err)
	}
	r.Favorite = fav
	if err := c.svc.FillAccess(ctx, c.user, r); err != nil {
		return fmt.Errorf("fill access: %w", err)
	}
	return nil
}
