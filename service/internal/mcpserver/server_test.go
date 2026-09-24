package mcpserver

import (
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
