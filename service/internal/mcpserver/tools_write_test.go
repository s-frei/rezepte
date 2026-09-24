package mcpserver

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/recipe"
)

var full = []string{auth.ScopeRecipesRead, auth.ScopeRecipesWrite, auth.ScopeRecipesDelete}

func text(res *mcp.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

func TestToolListsPerScope(t *testing.T) {
	e := newEnv(t)
	cases := []struct {
		scopes []string
		want   int
	}{
		{[]string{auth.ScopeRecipesRead}, 3},
		{[]string{auth.ScopeRecipesRead, auth.ScopeRecipesWrite}, 7},
		{full, 8},
	}
	for _, tc := range cases {
		if got := toolNames(t, e.connect(t, tc.scopes...)); len(got) != tc.want {
			t.Errorf("%v: %d tools %v, want %d", tc.scopes, len(got), got, tc.want)
		}
	}
}

func TestCreateRecipe(t *testing.T) {
	e := newEnv(t)
	cs := e.connect(t, full...)
	rec := validRecipe()
	rec["steps"] = []any{map[string]any{"text": "Wash the leek.", "references": []any{
		map[string]any{"word": "leek", "groupName": nil, "ingredientName": "Leek"},
	}}}
	got := structured[struct {
		ID        string `json:"id"`
		CreatedBy struct {
			ID string `json:"id"`
		} `json:"createdBy"`
	}](t, call(t, cs, "create_recipe", map[string]any{"recipe": rec}))
	if got.CreatedBy.ID != e.owner.ID {
		t.Fatalf("createdBy = %q, want token owner %q", got.CreatedBy.ID, e.owner.ID)
	}
	stored, err := e.svc.ByID(t.Context(), got.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(stored.Steps[0].References) != 1 {
		t.Fatalf("references not stored: %+v", stored.Steps)
	}
}

func TestCreateRecipeRejectsInvalidInput(t *testing.T) {
	e := newEnv(t)
	cs := e.connect(t, full...)
	rec := validRecipe()
	rec["sourceUrl"] = "javascript:alert(1)"
	if res := call(t, cs, "create_recipe", map[string]any{"recipe": rec}); !res.IsError || !strings.Contains(text(res), "sourceUrl") {
		t.Fatalf("javascript: source URL not refused by the schema: %q", text(res))
	}
}

func TestCreateRecipeReportsReferencePath(t *testing.T) {
	e := newEnv(t)
	cs := e.connect(t, full...)
	rec := validRecipe()
	rec["steps"] = []any{map[string]any{"text": "Cook.", "references": []any{
		map[string]any{"word": "leek", "groupName": nil, "ingredientName": "Leek"},
	}}}
	res := call(t, cs, "create_recipe", map[string]any{"recipe": rec})
	if !res.IsError || !strings.Contains(text(res), "steps[0].references[0].word") {
		t.Fatalf("got %q", text(res))
	}
}

func TestUpdateRecipe(t *testing.T) {
	e := newEnv(t)
	r := e.seed(t, "Leek soup")
	cs := e.connect(t, full...)
	rec := validRecipe()
	rec["title"] = "Leek and potato soup"
	call(t, cs, "update_recipe", map[string]any{"id": r.ID, "recipe": rec})
	got, err := e.svc.ByID(t.Context(), r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Leek and potato soup" || got.Slug != r.Slug {
		t.Fatalf("got %q / %q", got.Title, got.Slug)
	}
}

func TestUpdateRecipeRequiresReferences(t *testing.T) {
	e := newEnv(t)
	r := e.seed(t, "Leek soup")
	cs := e.connect(t, full...)
	rec := validRecipe()
	rec["steps"] = []any{map[string]any{"text": "Cook the leek."}}
	if res := call(t, cs, "update_recipe", map[string]any{"id": r.ID, "recipe": rec}); !res.IsError || !strings.Contains(text(res), "references") {
		t.Fatalf("step without references not refused by the schema: %q", text(res))
	}
}

func TestUpdateRecipeNotFound(t *testing.T) {
	e := newEnv(t)
	res := call(t, e.connect(t, full...), "update_recipe", map[string]any{"id": "missing", "recipe": validRecipe()})
	if !res.IsError || !strings.Contains(text(res), "recipe not found") {
		t.Fatalf("got %q", text(res))
	}
}

func TestDeleteRecipe(t *testing.T) {
	e := newEnv(t)
	r := e.seed(t, "Leek soup")
	call(t, e.connect(t, full...), "delete_recipe", map[string]any{"id": r.ID})
	if _, err := e.svc.ByID(t.Context(), r.ID); !errors.Is(err, recipe.ErrNotFound) {
		t.Fatalf("still there: %v", err)
	}
}

func TestUpdateRecipeFollowsEditingRights(t *testing.T) {
	e := newEnv(t)
	anna := e.member(t, "anna")
	locked := e.seedAs(t, anna, recipe.PolicyLocked, "Leek soup")
	open := e.seedAs(t, anna, recipe.PolicyOpen, "Leek pie")
	ben := e.connectAs(t, e.member(t, "ben"), full...)

	rec := validRecipe()
	res := call(t, ben, "update_recipe", map[string]any{"id": locked.ID, "recipe": rec})
	if !res.IsError || !strings.Contains(text(res), "may not make this change") {
		t.Fatalf("member edited a locked recipe: %q", text(res))
	}
	rec["editPolicy"] = "locked"
	res = call(t, ben, "update_recipe", map[string]any{"id": open.ID, "recipe": rec})
	if !res.IsError || !strings.Contains(text(res), "editPolicy") {
		t.Fatalf("member changed the policy of an open recipe: %q", text(res))
	}
	delete(rec, "editPolicy")
	if res := call(t, ben, "update_recipe", map[string]any{"id": open.ID, "recipe": rec}); res.IsError {
		t.Fatalf("member refused on an open recipe: %q", text(res))
	}

	rec["title"] = "Leek and potato soup"
	admin := e.connect(t, full...)
	if res := call(t, admin, "update_recipe", map[string]any{"id": locked.ID, "recipe": rec}); res.IsError {
		t.Fatalf("admin refused on a locked recipe: %q", text(res))
	}
	got, err := e.svc.ByID(t.Context(), locked.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Leek and potato soup" || got.EditPolicy != recipe.PolicyLocked {
		t.Fatalf("got %q under %q", got.Title, got.EditPolicy)
	}
}

func TestDeleteRecipeFollowsEditingRights(t *testing.T) {
	e := newEnv(t)
	anna := e.member(t, "anna")
	r := e.seedAs(t, anna, recipe.PolicyOpen, "Leek soup")
	res := call(t, e.connectAs(t, e.member(t, "ben"), full...), "delete_recipe", map[string]any{"id": r.ID})
	if !res.IsError || !strings.Contains(text(res), "only the recipe's author or an admin") {
		t.Fatalf("member deleted someone else's recipe: %q", text(res))
	}
	if res := call(t, e.connect(t, full...), "delete_recipe", map[string]any{"id": r.ID}); res.IsError {
		t.Fatalf("admin refused: %q", text(res))
	}
	if _, err := e.svc.ByID(t.Context(), r.ID); !errors.Is(err, recipe.ErrNotFound) {
		t.Fatalf("still there: %v", err)
	}
}

func TestDeleteIsDestructive(t *testing.T) {
	e := newEnv(t)
	res, err := e.connect(t, full...).ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	byName := func(name string) *mcp.ToolAnnotations {
		i := slices.IndexFunc(res.Tools, func(tl *mcp.Tool) bool { return tl.Name == name })
		if i < 0 {
			t.Fatalf("%s not listed", name)
		}
		return res.Tools[i].Annotations
	}
	if a := byName("delete_recipe"); a == nil || a.DestructiveHint == nil || !*a.DestructiveHint {
		t.Error("delete_recipe lacks destructiveHint")
	}
	for _, name := range []string{"create_recipe", "add_favorite", "remove_favorite"} {
		if a := byName(name); a == nil || a.DestructiveHint == nil || *a.DestructiveHint {
			t.Errorf("%s: destructiveHint = %v, want explicit false", name, a)
		}
	}
}

// TestToolsAreClosedWorld pins openWorldHint: false on every tool. Each one
// works on this instance's own collection only, and the SDK leaves the hint
// unset, which clients read as true.
func TestToolsAreClosedWorld(t *testing.T) {
	e := newEnv(t)
	res, err := e.connect(t, full...).ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Tools) != 8 {
		t.Fatalf("%d tools, want 8", len(res.Tools))
	}
	for _, tl := range res.Tools {
		if a := tl.Annotations; a == nil || a.OpenWorldHint == nil || *a.OpenWorldHint {
			t.Errorf("%s: openWorldHint not explicitly false", tl.Name)
		}
	}
}

// TestDocumentHelpNamesNullableFields checks that the write tools say which
// fields take null, so a model does not send null for description or [].
func TestDocumentHelpNamesNullableFields(t *testing.T) {
	for _, want := range []string{"every field", "description is a string", "prepMinutes", "quantity", "never null", "ingredientGroups", "at least one group"} {
		if !strings.Contains(documentHelp, want) {
			t.Errorf("documentHelp lacks %q", want)
		}
	}
}

func TestFavorites(t *testing.T) {
	e := newEnv(t)
	r := e.seed(t, "Leek soup")
	cs := e.connect(t, auth.ScopeRecipesRead, auth.ScopeRecipesWrite)
	call(t, cs, "add_favorite", map[string]any{"id": r.ID})
	if on, _ := e.svc.IsFavorite(t.Context(), e.owner.ID, r.ID); !on {
		t.Fatal("add_favorite did not star")
	}
	call(t, cs, "remove_favorite", map[string]any{"id": r.ID})
	if on, _ := e.svc.IsFavorite(t.Context(), e.owner.ID, r.ID); on {
		t.Fatal("remove_favorite did not unstar")
	}
}
