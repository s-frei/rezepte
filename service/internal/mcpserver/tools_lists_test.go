package mcpserver

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/shopping"
)

var (
	readTools  = []string{"list_shopping_lists", "get_shopping_list"}
	writeTools = []string{"create_shopping_list", "add_recipe_to_shopping_list", "add_shopping_items", "check_shopping_items", "remove_shopping_items"}
)

func (e env) list(t *testing.T) shopping.List {
	t.Helper()
	l, err := e.lists.Create(t.Context(), e.owner.ID, "Weekend")
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func TestShoppingToolsNeedScope(t *testing.T) {
	e := newEnv(t)
	has := func(names []string) int {
		n := 0
		for _, w := range append(slices.Clone(readTools), writeTools...) {
			if slices.Contains(names, w) {
				n++
			}
		}
		return n
	}
	if n := has(toolNames(t, e.connect(t, auth.ScopeRecipesRead))); n != 0 {
		t.Fatalf("recipes:read sees %d shopping tools", n)
	}
	got := toolNames(t, e.connect(t, auth.ScopeRecipesRead, auth.ScopeListsRead))
	if has(got) != 2 || !slices.Contains(got, readTools[0]) || !slices.Contains(got, readTools[1]) {
		t.Fatalf("lists:read tools = %v", got)
	}
	if n := has(toolNames(t, e.connect(t, auth.ScopeRecipesRead, auth.ScopeListsRead, auth.ScopeListsWrite))); n != 7 {
		t.Fatalf("lists:read+write sees %d shopping tools, want 7", n)
	}
}

func TestGetShoppingListForeign(t *testing.T) {
	e := newEnv(t)
	other := e.member(t, "jana")
	l, err := e.lists.Create(t.Context(), other.ID, "Hers")
	if err != nil {
		t.Fatal(err)
	}
	cs := e.connect(t, auth.ScopeRecipesRead, auth.ScopeListsRead)
	res := call(t, cs, "get_shopping_list", map[string]any{"listId": l.ID})
	if !res.IsError || !strings.Contains(text(res), "not found") {
		t.Fatalf("isError=%v %q", res.IsError, text(res))
	}
	out := structured[struct {
		Lists       []shopping.Summary    `json:"lists"`
		Invitations []shopping.Invitation `json:"invitations"`
	}](t, call(t, cs, "list_shopping_lists", nil))
	if len(out.Lists) != 0 {
		t.Fatalf("lists = %+v", out.Lists)
	}
}

func TestAddRecipeDefaults(t *testing.T) {
	e := newEnv(t)
	r := e.seed(t, "Leek soup")
	l := e.list(t)
	cs := e.connect(t, auth.ScopeRecipesRead, auth.ScopeListsWrite)
	got := structured[shopping.List](t, call(t, cs, "add_recipe_to_shopping_list", map[string]any{"listId": l.ID, "slug": r.Slug}))
	if len(got.Entries) != 1 || got.Entries[0].Servings != r.Servings || len(got.Items) != 1 || got.Items[0].Name != "Leek" {
		t.Fatalf("list = %+v", got)
	}
	res := call(t, cs, "add_recipe_to_shopping_list", map[string]any{"listId": l.ID, "slug": r.Slug, "recipeId": r.ID})
	if !res.IsError || !strings.Contains(text(res), "exactly one") {
		t.Fatalf("both: %q", text(res))
	}
}

func TestAddRecipeRejectsServings(t *testing.T) {
	e := newEnv(t)
	r := e.seed(t, "Leek soup")
	l := e.list(t)
	cs := e.connect(t, auth.ScopeRecipesRead, auth.ScopeListsWrite)
	for _, n := range []int{0, 100} {
		res := call(t, cs, "add_recipe_to_shopping_list", map[string]any{"listId": l.ID, "recipeId": r.ID, "servings": n})
		if !res.IsError {
			t.Fatalf("servings %d accepted", n)
		}
	}
	if got, _ := e.lists.Get(t.Context(), e.owner.ID, l.ID); len(got.Entries) != 0 {
		t.Fatalf("list changed: %+v", got)
	}
}

func TestAddShoppingItemsParses(t *testing.T) {
	e := newEnv(t)
	l := e.list(t)
	cs := e.connect(t, auth.ScopeRecipesRead, auth.ScopeListsWrite)
	got := structured[shopping.List](t, call(t, cs, "add_shopping_items", map[string]any{"listId": l.ID, "items": []string{"2 l Milch", "Brot"}}))
	if len(got.Items) != 2 {
		t.Fatalf("items = %+v", got.Items)
	}
	m := got.Items[0]
	if m.Name != "Milch" || m.Quantity == nil || *m.Quantity != 2 || m.Unit == nil || *m.Unit != "l" {
		t.Fatalf("milk = %+v", m)
	}
	for _, items := range [][]string{{}, make([]string, 51)} {
		res := call(t, cs, "add_shopping_items", map[string]any{"listId": l.ID, "items": items})
		if !res.IsError || !strings.Contains(text(res), "nothing was added") {
			t.Fatalf("%d items: %q", len(items), text(res))
		}
	}
	res := call(t, cs, "add_shopping_items", map[string]any{"listId": l.ID, "items": []string{"Eier", " "}})
	if !res.IsError || !strings.Contains(text(res), "1 of 2") {
		t.Fatalf("blank line: %q", text(res))
	}
}

func TestCheckAndRemoveItems(t *testing.T) {
	e := newEnv(t)
	l := e.list(t)
	cs := e.connect(t, auth.ScopeRecipesRead, auth.ScopeListsWrite)
	l = structured[shopping.List](t, call(t, cs, "add_shopping_items", map[string]any{"listId": l.ID, "items": []string{"Milch", "Brot"}}))
	id := l.Items[0].ID
	l = structured[shopping.List](t, call(t, cs, "check_shopping_items", map[string]any{"listId": l.ID, "itemIds": []string{id}, "checked": true}))
	if !l.Items[0].Checked || l.DoneCount != 1 {
		t.Fatalf("after check = %+v", l)
	}
	l = structured[shopping.List](t, call(t, cs, "check_shopping_items", map[string]any{"listId": l.ID, "itemIds": []string{id}, "checked": false}))
	if l.Items[0].Checked {
		t.Fatal("still checked")
	}
	l = structured[shopping.List](t, call(t, cs, "remove_shopping_items", map[string]any{"listId": l.ID, "itemIds": []string{id}}))
	if len(l.Items) != 1 || l.Items[0].Name != "Brot" {
		t.Fatalf("after remove = %+v", l.Items)
	}
}

func TestToolChangePublishes(t *testing.T) {
	e := newEnv(t)
	l := e.list(t)
	events, cancel := e.lists.Hub().Subscribe(l.ID)
	defer cancel()
	cs := e.connect(t, auth.ScopeRecipesRead, auth.ScopeListsWrite)
	call(t, cs, "add_shopping_items", map[string]any{"listId": l.ID, "items": []string{"Milch"}})
	select {
	case <-events:
	case <-time.After(time.Second):
		t.Fatal("no signal")
	}
}
