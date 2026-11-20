package mcpserver

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/mealplan"
	"github.com/s-frei/rezepte/service/internal/recipe"
	"github.com/s-frei/rezepte/service/internal/shopping"
)

var (
	planReadTools  = []string{"list_meal_plans", "get_meal_plan"}
	planWriteTools = []string{"add_to_meal_plan", "update_meal_plan_entry", "remove_meal_plan_entry", "put_meal_plan_on_list"}
)

func TestPlanToolsNeedScope(t *testing.T) {
	e := newEnv(t)
	count := func(names []string) int {
		n := 0
		for _, w := range append(slices.Clone(planReadTools), planWriteTools...) {
			if slices.Contains(names, w) {
				n++
			}
		}
		return n
	}
	if n := count(toolNames(t, e.connect(t, auth.ScopeRecipesRead, auth.ScopeListsWrite))); n != 0 {
		t.Fatalf("lists scopes see %d plan tools", n)
	}
	if n := count(toolNames(t, e.connect(t, auth.ScopeRecipesRead, auth.ScopePlanRead))); n != 2 {
		t.Fatalf("plan:read sees %d plan tools, want 2", n)
	}
	// Putting a plan on a list writes a list: without lists:write that tool is not offered.
	names := toolNames(t, e.connect(t, auth.ScopeRecipesRead, auth.ScopePlanRead, auth.ScopePlanWrite))
	if n := count(names); n != 5 || slices.Contains(names, "put_meal_plan_on_list") {
		t.Fatalf("plan:read+write sees %d plan tools %v, want 5 without put_meal_plan_on_list", n, names)
	}
	if n := count(toolNames(t, e.connect(t, auth.ScopeRecipesRead, auth.ScopePlanRead, auth.ScopePlanWrite, auth.ScopeListsWrite))); n != 6 {
		t.Fatalf("plan:read+write+lists:write sees %d plan tools, want 6", n)
	}
}

func TestAddToMealPlanBySlug(t *testing.T) {
	e := newEnv(t)
	r := e.seed(t, "Leek soup")
	cs := e.connect(t, auth.ScopeRecipesRead, auth.ScopePlanRead, auth.ScopePlanWrite)
	got := structured[mealplan.Plan](t, call(t, cs, "add_to_meal_plan", map[string]any{"slug": r.Slug, "day": "2026-11-17", "servings": 4}))
	_ = got
	plan := structured[mealplan.Plan](t, call(t, cs, "get_meal_plan", map[string]any{"from": "2026-11-16", "to": "2026-11-22"}))
	if !plan.Personal || len(plan.Entries) != 1 || plan.Entries[0].Servings == nil || *plan.Entries[0].Servings != 4 || *plan.Entries[0].RecipeID != r.ID {
		t.Fatalf("plan = %+v", plan)
	}
	res := call(t, cs, "add_to_meal_plan", map[string]any{"slug": r.Slug, "recipeId": r.ID})
	if !res.IsError {
		t.Fatal("both slug and recipeId accepted")
	}
	free := structured[mealplan.Plan](t, call(t, cs, "add_to_meal_plan", map[string]any{"title": "Leftovers", "day": "2026-11-18"}))
	_ = free
	plan = structured[mealplan.Plan](t, call(t, cs, "get_meal_plan", map[string]any{"from": "2026-11-16", "to": "2026-11-22"}))
	if len(plan.Entries) != 2 {
		t.Fatalf("entries = %+v", plan.Entries)
	}
	id := plan.Entries[0].ID
	call(t, cs, "update_meal_plan_entry", map[string]any{"planId": plan.ID, "entryId": id, "servings": 6, "from": "2026-11-16", "to": "2026-11-22"})
	call(t, cs, "remove_meal_plan_entry", map[string]any{"planId": plan.ID, "entryId": plan.Entries[1].ID})
	plan = structured[mealplan.Plan](t, call(t, cs, "get_meal_plan", map[string]any{"from": "2026-11-16", "to": "2026-11-22"}))
	if len(plan.Entries) != 1 || *plan.Entries[0].Servings != 6 {
		t.Fatalf("entries = %+v", plan.Entries)
	}
}

func TestPutMealPlanOnListSkipsStaples(t *testing.T) {
	e := newEnv(t)
	r, err := e.svc.Create(t.Context(), e.owner.ID, recipe.Input{
		Title: "Soup", Servings: 2,
		IngredientGroups: []recipe.IngredientGroup{{Ingredients: []recipe.Ingredient{{Name: "Leek"}, {Name: "Salt"}}}},
		Steps:            []recipe.Step{{Text: "Cook.", References: []recipe.IngredientRef{}, Times: []recipe.StepTime{}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	cs := e.connect(t, auth.ScopeRecipesRead, auth.ScopePlanRead, auth.ScopePlanWrite, auth.ScopeListsRead, auth.ScopeListsWrite)
	plan := structured[mealplan.Plan](t, call(t, cs, "add_to_meal_plan", map[string]any{"recipeId": r.ID, "day": "2026-11-17"}))
	if _, err := e.plans.SetStaple(t.Context(), e.owner.ID, plan.ID, "Salt", true); err != nil {
		t.Fatal(err)
	}
	res := call(t, cs, "put_meal_plan_on_list", map[string]any{"planId": plan.ID, "from": "2026-11-16", "to": "2026-11-22", "newListName": "Week"})
	if res.IsError {
		t.Fatalf("%s", text(res))
	}
	lists, _ := e.lists.Lists(t.Context(), e.owner.ID)
	if len(lists) != 1 {
		t.Fatalf("lists = %+v", lists)
	}
	l, err := e.lists.Get(t.Context(), e.owner.ID, lists[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, it := range l.Items {
		names = append(names, it.Name)
	}
	if !slices.Equal(names, []string{"Leek"}) {
		t.Fatalf("items = %v", names)
	}
	_ = shopping.List{}
}

func (e env) planTools(t *testing.T) *mcp.ClientSession {
	t.Helper()
	return e.connect(t, auth.ScopeRecipesRead, auth.ScopePlanRead, auth.ScopePlanWrite, auth.ScopeListsRead, auth.ScopeListsWrite)
}

// someone else's plan and list answer "not found" and stay as they were.
func TestPlanToolsRefuseForeignPlanAndList(t *testing.T) {
	e := newEnv(t)
	other := e.member(t, "jana")
	r := e.seed(t, "Leek soup")
	theirs, _, err := e.plans.Plans(t.Context(), other.ID)
	if err != nil {
		t.Fatal(err)
	}
	from, to := e.plans.Week()
	in, err := e.plans.Add(t.Context(), other.ID, theirs[0].ID, from, to, mealplan.AddEntry{Title: new("Secret")})
	if err != nil {
		t.Fatal(err)
	}
	list, err := e.lists.Create(t.Context(), other.ID, "Hers")
	if err != nil {
		t.Fatal(err)
	}
	cs := e.planTools(t)
	for name, args := range map[string]map[string]any{
		"get_meal_plan":          {"planId": theirs[0].ID},
		"add_to_meal_plan":       {"planId": theirs[0].ID, "title": "Mine now"},
		"update_meal_plan_entry": {"planId": theirs[0].ID, "entryId": in.Entries[0].ID, "tray": true},
		"remove_meal_plan_entry": {"planId": theirs[0].ID, "entryId": in.Entries[0].ID},
	} {
		if res := call(t, cs, name, args); !res.IsError || !strings.Contains(text(res), "not found") {
			t.Errorf("%s: isError=%v %q", name, res.IsError, text(res))
		}
	}
	got, err := e.plans.Get(t.Context(), other.ID, theirs[0].ID, from, to)
	if err != nil || len(got.Entries) != 1 || got.Entries[0].Title != "Secret" {
		t.Fatalf("their plan changed: %+v, %v", got.Entries, err)
	}
	// Our own plan, their list.
	mine := structured[mealplan.Plan](t, call(t, cs, "add_to_meal_plan", map[string]any{"recipeId": r.ID, "day": from}))
	res := call(t, cs, "put_meal_plan_on_list", map[string]any{"planId": mine.ID, "from": from, "to": to, "listId": list.ID})
	if !res.IsError || !strings.Contains(text(res), "shopping list not found") {
		t.Fatalf("a foreign list: isError=%v %q", res.IsError, text(res))
	}
	if l, err := e.lists.Get(t.Context(), other.ID, list.ID); err != nil || len(l.Entries) != 0 || len(l.Items) != 0 {
		t.Fatalf("their list changed: %+v, %v", l, err)
	}
}

func TestMealPlanDefaultsToPersonalPlanAndThisWeek(t *testing.T) {
	e := newEnv(t)
	r := e.seed(t, "Leek soup")
	cs := e.planTools(t)
	from, _ := e.plans.Week()
	call(t, cs, "add_to_meal_plan", map[string]any{"recipeId": r.ID, "day": from})
	call(t, cs, "add_to_meal_plan", map[string]any{"recipeId": r.ID, "day": time.Now().AddDate(0, 0, -30).Format(time.DateOnly)})
	p := structured[mealplan.Plan](t, call(t, cs, "get_meal_plan", nil))
	if !p.Personal || p.From != from || len(p.Entries) != 1 || p.Entries[0].Day == nil || *p.Entries[0].Day != from {
		t.Fatalf("plan = %+v", p)
	}
	if res := call(t, cs, "get_meal_plan", map[string]any{"from": from}); !res.IsError {
		t.Fatal("from without to accepted")
	}
}

func TestUpdateMealPlanEntryMovesToDayAndTray(t *testing.T) {
	e := newEnv(t)
	r := e.seed(t, "Leek soup")
	cs := e.planTools(t)
	from, to := e.plans.Week()
	p := structured[mealplan.Plan](t, call(t, cs, "add_to_meal_plan", map[string]any{"recipeId": r.ID, "day": from}))
	id := p.Entries[0].ID
	p = structured[mealplan.Plan](t, call(t, cs, "update_meal_plan_entry", map[string]any{"planId": p.ID, "entryId": id, "day": to, "servings": 5}))
	if d := p.Entries[0].Day; d == nil || *d != to || *p.Entries[0].Servings != 5 {
		t.Fatalf("after day move: %+v", p.Entries[0])
	}
	p = structured[mealplan.Plan](t, call(t, cs, "update_meal_plan_entry", map[string]any{"planId": p.ID, "entryId": id, "tray": true}))
	if p.Entries[0].Day != nil {
		t.Fatalf("after tray move: %+v", p.Entries[0])
	}
	for _, args := range []map[string]any{{"day": from, "tray": true}, {}} {
		args["planId"], args["entryId"] = p.ID, id
		if res := call(t, cs, "update_meal_plan_entry", args); !res.IsError {
			t.Fatalf("%v accepted", args)
		}
	}
}

// A refused servings change must not leave the entry moved.
func TestUpdateMealPlanEntryIsAllOrNothing(t *testing.T) {
	e := newEnv(t)
	cs := e.planTools(t)
	from, to := e.plans.Week()
	p := structured[mealplan.Plan](t, call(t, cs, "add_to_meal_plan", map[string]any{"title": "Leftovers", "day": from}))
	res := call(t, cs, "update_meal_plan_entry", map[string]any{"planId": p.ID, "entryId": p.Entries[0].ID, "day": to, "servings": 3})
	if !res.IsError {
		t.Fatal("servings on a free text accepted")
	}
	got := structured[mealplan.Plan](t, call(t, cs, "get_meal_plan", map[string]any{"planId": p.ID}))
	if d := got.Entries[0].Day; d == nil || *d != from {
		t.Fatalf("entry moved to %v despite the refusal", d)
	}
}

func TestPutMealPlanOnListWithOnlyStaplesFails(t *testing.T) {
	e := newEnv(t)
	r := e.seed(t, "Leek soup")
	cs := e.planTools(t)
	from, to := e.plans.Week()
	p := structured[mealplan.Plan](t, call(t, cs, "add_to_meal_plan", map[string]any{"recipeId": r.ID, "day": from}))
	if _, err := e.plans.SetStaple(t.Context(), e.owner.ID, p.ID, "Leek", true); err != nil {
		t.Fatal(err)
	}
	res := call(t, cs, "put_meal_plan_on_list", map[string]any{"planId": p.ID, "from": from, "to": to, "newListName": "Week"})
	if !res.IsError || !strings.Contains(text(res), "nothing to put") {
		t.Fatalf("isError=%v %q", res.IsError, text(res))
	}
	if lists, _ := e.lists.Lists(t.Context(), e.owner.ID); len(lists) != 0 {
		t.Fatalf("lists = %+v", lists)
	}
}
