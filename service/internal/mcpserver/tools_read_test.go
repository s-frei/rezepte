package mcpserver

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/recipe"
)

func structured[T any](t *testing.T, res *mcp.CallToolResult) T {
	t.Helper()
	if res.IsError {
		t.Fatalf("tool error: %v", res.Content)
	}
	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestSearchRecipes(t *testing.T) {
	e := newEnv(t)
	e.seed(t, "Leek soup", "soup")
	e.seed(t, "Apple pie", "dessert")
	cs := e.connect(t, auth.ScopeRecipesRead)
	// Both seeds hold a leek, so the filters are told apart by title and tag.
	for _, args := range []map[string]any{
		{"query": "pie"},
		{"tags": []any{" Dessert "}},
	} {
		page := structured[struct {
			Items []struct{ Title string } `json:"items"`
			Total int                      `json:"total"`
		}](t, call(t, cs, "search_recipes", args))
		if page.Total != 1 || page.Items[0].Title != "Apple pie" {
			t.Fatalf("%v: page = %+v", args, page)
		}
	}
}

func TestSearchRecipesEmptyCollection(t *testing.T) {
	e := newEnv(t)
	cs := e.connect(t, auth.ScopeRecipesRead)
	res := call(t, cs, "search_recipes", map[string]any{})
	if res.IsError {
		t.Fatalf("empty search is an error: %v", res.Content)
	}
}

func TestSearchRecipesRejectsLongQuery(t *testing.T) {
	e := newEnv(t)
	cs := e.connect(t, auth.ScopeRecipesRead)
	res := call(t, cs, "search_recipes", map[string]any{"query": strings.Repeat("a", 101)})
	if !res.IsError {
		t.Fatal("101-character query accepted")
	}
}

// TestSearchRecipesBoundsInput holds search_recipes to the bounds REST puts
// on GET /recipes. Each refusal is a tool error a model can read - never one
// that names the Go type the arguments decode into.
func TestSearchRecipesBoundsInput(t *testing.T) {
	e := newEnv(t)
	cs := e.connect(t, auth.ScopeRecipesRead)
	for name, args := range map[string]map[string]any{
		"unknown sort":    {"sort": "bogus"},
		"zero limit":      {"limit": 0},
		"limit above 100": {"limit": 101},
		"zero page":       {"page": 0},
		"huge page":       {"page": 1e12},
		"page overflow":   {"page": 1e30},
		"negative time":   {"maxMinutes": -1},
		"time above 1440": {"maxMinutes": 1441},
		"long query":      {"query": strings.Repeat("ä", 101)},
		"long author":     {"author": strings.Repeat("ä", 51)},
	} {
		t.Run(name, func(t *testing.T) {
			res := call(t, cs, "search_recipes", args)
			text := fmt.Sprint(res.Content[0].(*mcp.TextContent).Text)
			if !res.IsError {
				t.Fatalf("accepted: %s", text)
			}
			t.Log(text)
			for _, internal := range []string{"searchIn", "Go struct", "of type int"} {
				if strings.Contains(text, internal) {
					t.Fatalf("error names %q: %s", internal, text)
				}
			}
		})
	}
}

// TestSearchRecipesAcceptsBounds is the other edge: the limits themselves
// are allowed, and "ä" counts as one character, as it does over REST.
func TestSearchRecipesAcceptsBounds(t *testing.T) {
	e := newEnv(t)
	cs := e.connect(t, auth.ScopeRecipesRead)
	res := call(t, cs, "search_recipes", map[string]any{
		"sort": "title", "limit": 100, "page": 1, "maxMinutes": 1440,
		"query": strings.Repeat("ä", 100), "author": strings.Repeat("ä", 50),
	})
	if res.IsError {
		t.Fatalf("bounds refused: %v", res.Content)
	}
}

// TestSearchRecipesEmptySortIsDefault mirrors REST's ?sort=: an empty sort
// is the default order (updated, newest first), not an error.
func TestSearchRecipesEmptySortIsDefault(t *testing.T) {
	e := newEnv(t)
	e.seed(t, "Apple pie")
	e.seed(t, "Leek soup")
	cs := e.connect(t, auth.ScopeRecipesRead)
	page := structured[struct {
		Items []struct{ Title string } `json:"items"`
	}](t, call(t, cs, "search_recipes", map[string]any{"sort": ""}))
	if len(page.Items) != 2 || page.Items[0].Title != "Leek soup" {
		t.Fatalf("items = %+v, want the newest first", page.Items)
	}
}

func TestGetRecipeByIDAndSlug(t *testing.T) {
	e := newEnv(t)
	r := e.seed(t, "Leek soup")
	if err := e.svc.SetFavorite(t.Context(), e.owner.ID, r.ID, true); err != nil {
		t.Fatal(err)
	}
	cs := e.connect(t, auth.ScopeRecipesRead)
	for _, args := range []map[string]any{{"id": r.ID}, {"slug": r.Slug}} {
		got := structured[struct {
			ID        string `json:"id"`
			Favorite  bool   `json:"favorite"`
			CanEdit   bool   `json:"canEdit"`
			CanDelete bool   `json:"canDelete"`
		}](t, call(t, cs, "get_recipe", args))
		if got.ID != r.ID || !got.Favorite || !got.CanEdit || !got.CanDelete {
			t.Fatalf("%v: got %+v", args, got)
		}
	}
}

func TestGetRecipeSaysWhatAMemberMayDo(t *testing.T) {
	e := newEnv(t)
	r := e.seedAs(t, e.owner, recipe.PolicyLocked, "Leek soup")
	cs := e.connectAs(t, e.member(t, "ben"), auth.ScopeRecipesRead)
	got := structured[struct {
		Locked    bool `json:"locked"`
		CanEdit   bool `json:"canEdit"`
		CanDelete bool `json:"canDelete"`
	}](t, call(t, cs, "get_recipe", map[string]any{"id": r.ID}))
	if !got.Locked || got.CanEdit || got.CanDelete {
		t.Fatalf("got %+v, want locked and neither edit nor delete", got)
	}
}

func TestGetRecipeNeedsExactlyOneKey(t *testing.T) {
	e := newEnv(t)
	cs := e.connect(t, auth.ScopeRecipesRead)
	for _, args := range []map[string]any{{}, {"id": "a", "slug": "b"}} {
		if res := call(t, cs, "get_recipe", args); !res.IsError {
			t.Fatalf("%v accepted", args)
		}
	}
}

func TestGetRecipeNotFound(t *testing.T) {
	e := newEnv(t)
	res := call(t, e.connect(t, auth.ScopeRecipesRead), "get_recipe", map[string]any{"id": "missing"})
	if !res.IsError || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "recipe not found") {
		t.Fatalf("got %+v", res)
	}
}

func TestListTags(t *testing.T) {
	e := newEnv(t)
	e.seed(t, "Leek soup", "soup")
	got := structured[struct {
		Tags []struct {
			Name  string `json:"name"`
			Count int    `json:"count"`
		} `json:"tags"`
	}](t, call(t, e.connect(t, auth.ScopeRecipesRead), "list_tags", map[string]any{}))
	if len(got.Tags) != 1 || got.Tags[0].Name != "soup" || got.Tags[0].Count != 1 {
		t.Fatalf("tags = %+v", got.Tags)
	}
}

func TestToolErrorHidesInternals(t *testing.T) {
	var logged bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logged, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	err := toolError(t.Context(), fmt.Errorf("list tags: %w", errors.New("sqlite: table recipes is locked")))
	if err.Error() != "internal error" {
		t.Fatalf("err = %q, want a generic message", err)
	}
	if !strings.Contains(logged.String(), "table recipes is locked") {
		t.Fatalf("real error not logged: %q", logged.String())
	}
	if got := toolError(t.Context(), fmt.Errorf("get recipe: %w", recipe.ErrNotFound)); got.Error() != "recipe not found" {
		t.Fatalf("not found = %q", got)
	}
}
