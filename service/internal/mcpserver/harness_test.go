package mcpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/db/dbtest"
	"github.com/s-frei/rezepte/service/internal/recipe"
	"github.com/s-frei/rezepte/service/internal/user"
)

type env struct {
	svc    *recipe.Service
	users  *user.Service
	tokens *auth.TokenService
	owner  user.User
	url    string
	// base is the caller Handler builds, without a user; connectAs fills
	// one in.
	base caller
}

func newEnv(t *testing.T) env {
	t.Helper()
	conn := dbtest.Open(t)
	users := user.NewService(conn)
	tokens := auth.NewTokenService(conn, users)
	svc := recipe.NewService(conn)
	_, api := humatest.New(t)
	recipe.Register(api, svc)
	owner, err := users.Create(t.Context(), user.CreateParams{Username: "cook", Password: "secret123", Role: user.RoleAdmin})
	if err != nil {
		t.Fatal(err)
	}
	h, err := Handler(svc, api, "test")
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(auth.RequireToken(tokens, auth.ScopeRecipesRead)(h))
	t.Cleanup(ts.Close)
	create, err := recipeInputSchema(api)
	if err != nil {
		t.Fatal(err)
	}
	update, err := recipeUpdateSchema(api)
	if err != nil {
		t.Fatal(err)
	}
	search, err := searchSchema()
	if err != nil {
		t.Fatal(err)
	}
	base := caller{svc: svc, create: create, update: update, search: search}
	return env{svc: svc, users: users, tokens: tokens, owner: owner, url: ts.URL, base: base}
}

// member creates a plain user, whose editing rights are narrower than the
// admin owner's.
func (e env) member(t *testing.T, name string) user.User {
	t.Helper()
	u, err := e.users.Create(t.Context(), user.CreateParams{Username: name, Password: "secret123", Role: user.RoleUser})
	if err != nil {
		t.Fatal(err)
	}
	return u
}

// connectAs registers the tools scopes cover for u on a server of its own
// and connects to it in memory. It skips RequireToken, which authenticates
// only an admin's token, so a test can check what the tools do for a user
// no token acts as today: they hand that user to the editing rights rather
// than rely on the token gate to have ruled them out.
func (e env) connectAs(t *testing.T, u user.User, scopes ...string) *mcp.ClientSession {
	t.Helper()
	c := e.base
	c.user = u
	s := mcp.NewServer(&mcp.Implementation{Name: "rezepte", Version: "test"}, nil)
	for _, tl := range tools {
		if slices.Contains(scopes, tl.scope) {
			tl.add(s, c)
		}
	}
	st, ct := mcp.NewInMemoryTransports()
	ss, err := s.Connect(t.Context(), st, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ss.Close() })
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(t.Context(), ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

// bearer adds an Authorization header to every request the client sends.
type bearer struct {
	raw  string
	next http.RoundTripper
}

func (b bearer) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.raw)
	return b.next.RoundTrip(r)
}

// connect issues a token with scopes and returns an MCP client session
// speaking the real protocol to the test server.
func (e env) connect(t *testing.T, scopes ...string) *mcp.ClientSession {
	t.Helper()
	raw, _, err := e.tokens.Create(t.Context(), e.owner.ID, "mcp-test", scopes, nil)
	if err != nil {
		t.Fatal(err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	cs, err := client.Connect(t.Context(), &mcp.StreamableClientTransport{
		Endpoint:   e.url,
		HTTPClient: &http.Client{Transport: bearer{raw: raw, next: http.DefaultTransport}},
		MaxRetries: -1,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func (e env) seed(t *testing.T, title string, tags ...string) recipe.Recipe {
	t.Helper()
	return e.seedAs(t, e.owner, recipe.PolicyDefault, title, tags...)
}

// seedAs creates a recipe authored by author under policy.
func (e env) seedAs(t *testing.T, author user.User, policy recipe.Policy, title string, tags ...string) recipe.Recipe {
	t.Helper()
	r, err := e.svc.Create(context.Background(), author.ID, recipe.Input{
		Title: title, Servings: 2, Tags: tags, EditPolicy: policy,
		IngredientGroups: []recipe.IngredientGroup{{Ingredients: []recipe.Ingredient{{Name: "Leek"}}}},
		Steps:            []recipe.Step{{Text: "Cook the leek.", References: []recipe.IngredientRef{}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func toolNames(t *testing.T, cs *mcp.ClientSession) []string {
	t.Helper()
	res, err := cs.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(res.Tools))
	for _, tl := range res.Tools {
		names = append(names, tl.Name)
	}
	return names
}

func call(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	res, err := cs.CallTool(t.Context(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return res
}
