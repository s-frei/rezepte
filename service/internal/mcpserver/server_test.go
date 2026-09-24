package mcpserver

import (
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/s-frei/rezepte/service/internal/auth"
)

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
