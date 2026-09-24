package mcpserver

import (
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/recipe"
)

// TestHandlerFailsAtBootWithoutRecipeRegister pins Handler's contract: a
// broken schema is a start-up error, never a panic on the first request that
// needs it. Without recipe.Register, componentDefs cannot find the recipe
// components, which is the cheapest way to make the schema build fail.
func TestHandlerFailsAtBootWithoutRecipeRegister(t *testing.T) {
	_, api := humatest.New(t)
	if _, err := Handler(&recipe.Service{}, api, "test"); err == nil || !strings.Contains(err.Error(), "recipe.Register") {
		t.Fatalf("err = %v, want a recipe.Register error", err)
	}
}

// TestBootResolveMatchesAddTool pins that the start-up check is as strict as
// mcp.AddTool, which resolves with ValidateDefaults: a default the schema
// itself refuses must fail at boot, not panic on the first request.
func TestBootResolveMatchesAddTool(t *testing.T) {
	s := &jsonschema.Schema{Type: "object", Properties: map[string]*jsonschema.Schema{
		"limit": {Type: "integer", Minimum: jsonschema.Ptr(1.0), Default: json.RawMessage(`0`)},
	}}
	if err := resolveAsAddTool(s); err == nil {
		t.Fatal("invalid default accepted at boot")
	}
}

func TestReadTokenListsReadTools(t *testing.T) {
	e := newEnv(t)
	got := toolNames(t, e.connect(t, auth.ScopeRecipesRead))
	slices.Sort(got)
	want := []string{"get_recipe", "list_tags", "search_recipes"}
	if !slices.Equal(got, want) {
		t.Fatalf("tools = %v, want %v", got, want)
	}
}

func TestReadTokenCannotCallWriteTool(t *testing.T) {
	e := newEnv(t)
	cs := e.connect(t, auth.ScopeRecipesRead)
	_, err := cs.CallTool(t.Context(), &mcp.CallToolParams{Name: "delete_recipe", Arguments: map[string]any{"id": "x"}})
	if err == nil || !strings.Contains(err.Error(), `unknown tool "delete_recipe"`) {
		t.Fatalf("err = %v, want unknown tool", err)
	}
}

func TestReadToolsAreAnnotatedReadOnly(t *testing.T) {
	e := newEnv(t)
	res, err := e.connect(t, auth.ScopeRecipesRead).ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tl := range res.Tools {
		if tl.Annotations == nil || !tl.Annotations.ReadOnlyHint {
			t.Errorf("%s: readOnlyHint not set", tl.Name)
		}
	}
}

// TestAcceptsReverseProxyHost pins the request a reverse proxy on the same
// host sends: it arrives on a loopback listener (httptest binds 127.0.0.1)
// carrying the public Host header. The SDK's DNS-rebinding guard would 403
// it; the bearer token is what protects /mcp.
func TestAcceptsReverseProxyHost(t *testing.T) {
	e := newEnv(t)
	raw, _, err := e.tokens.Create(t.Context(), e.owner.ID, "proxy", []string{auth.ScopeRecipesRead}, nil)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, e.url,
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "recipes.example.org"
	req.Header.Set("Authorization", "Bearer "+raw)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK || !strings.Contains(string(body), `"search_recipes"`) {
		t.Fatalf("status %d, body %s", res.StatusCode, body)
	}
}
