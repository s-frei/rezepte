package shopping_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/s-frei/rezepte/service/internal/db/dbtest"
	"github.com/s-frei/rezepte/service/internal/recipe"
	"github.com/s-frei/rezepte/service/internal/shopping"
	"github.com/s-frei/rezepte/service/internal/user"
)

type env struct {
	conn    *sql.DB
	svc     *shopping.Service
	recipes *recipe.Service
	users   *user.Service
	a, b    user.User
}

func setup(t *testing.T) env {
	t.Helper()
	conn := dbtest.Open(t)
	users := user.NewService(conn, "")
	mk := func(name string, role user.Role) user.User {
		u, err := users.Create(context.Background(), user.CreateParams{Username: name, Password: "pw", Role: role})
		if err != nil {
			t.Fatal(err)
		}
		return u
	}
	return env{conn: conn, svc: shopping.NewService(conn), recipes: recipe.NewService(conn, t.TempDir()), users: users, a: mk("anna", user.RoleSuperadmin), b: mk("ben", user.RoleUser)}
}

func (e env) hackfleisch(t *testing.T) recipe.Recipe {
	t.Helper()
	q, g := 500.0, "g"
	one := 1.0
	prise := "Prise"
	name := "Sauce"
	r, err := e.recipes.Create(context.Background(), e.a.ID, recipe.Input{
		Title: "Hack", Servings: 4, Tags: []string{},
		IngredientGroups: []recipe.IngredientGroup{
			{Name: &name, Ingredients: []recipe.Ingredient{
				{Quantity: &q, Unit: &g, Name: "Rinderhackfleisch"},
				{Quantity: &one, Unit: &prise, Name: "Salz"},
			}},
		},
		Steps: []recipe.Step{{Text: "Kochen."}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func (e env) list(t *testing.T) shopping.List {
	t.Helper()
	l, err := e.svc.Create(context.Background(), e.a.ID, "Woche")
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func wantQty(t *testing.T, it shopping.Item, want float64) {
	t.Helper()
	if it.Quantity == nil || *it.Quantity != want {
		t.Fatalf("%s quantity = %v, want %v", it.Name, it.Quantity, want)
	}
}

func TestAddRecipeCopiesSelectedIngredientsAtServings(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	r := e.hackfleisch(t)
	l, err := e.svc.AddRecipe(ctx, e.a.ID, e.list(t).ID, r.ID, 6, []shopping.IngredientPick{{Group: 0, Index: 0}})
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Items) != 1 || len(l.Entries) != 1 {
		t.Fatalf("items/entries = %d/%d", len(l.Items), len(l.Entries))
	}
	it := l.Items[0]
	wantQty(t, it, 750)
	if it.Unit == nil || *it.Unit != "g" || it.Group == nil || *it.Group != "Sauce" || it.AddedBy != nil {
		t.Fatalf("item = %+v", it)
	}
	if l.Entries[0].BaseServings != 4 || l.Entries[0].Servings != 6 || l.Entries[0].RecipeSlug == nil || l.OpenCount != 1 {
		t.Fatalf("entry/counts = %+v %d", l.Entries[0], l.OpenCount)
	}
	l, err = e.svc.AddRecipe(ctx, e.a.ID, l.ID, r.ID, 2, []shopping.IngredientPick{{Group: 0, Index: 1}})
	if err != nil {
		t.Fatal(err)
	}
	if l.Items[1].Name != "Salz" {
		t.Fatalf("salt = %+v", l.Items[1])
	}
	wantQty(t, l.Items[1], 0.5) // 1 Prise at 4 servings, scaled to 2
	if _, err := e.svc.AddRecipe(ctx, e.a.ID, l.ID, r.ID, 2, []shopping.IngredientPick{{Group: 0, Index: 5}}); !errors.Is(err, shopping.ErrInvalid) {
		t.Fatalf("out of range: %v", err)
	}
}

func TestRecipeWithoutQuantityStaysWithout(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	r, err := e.recipes.Create(ctx, e.a.ID, recipe.Input{
		Title: "Salzig", Servings: 4, Tags: []string{},
		IngredientGroups: []recipe.IngredientGroup{{Ingredients: []recipe.Ingredient{{Name: "Salz"}}}},
		Steps:            []recipe.Step{{Text: "Salzen."}},
	})
	if err != nil {
		t.Fatal(err)
	}
	l, err := e.svc.AddRecipe(ctx, e.a.ID, e.list(t).ID, r.ID, 2, []shopping.IngredientPick{{Group: 0, Index: 0}})
	if err != nil {
		t.Fatal(err)
	}
	if l.Items[0].Quantity != nil || l.Items[0].Group != nil {
		t.Fatalf("item = %+v", l.Items[0])
	}
}

func TestRecipeEditDoesNotReachList(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	r := e.hackfleisch(t)
	l, err := e.svc.AddRecipe(ctx, e.a.ID, e.list(t).ID, r.ID, 4, []shopping.IngredientPick{{Group: 0, Index: 0}})
	if err != nil {
		t.Fatal(err)
	}
	q, g := 400.0, "g"
	name := "Sauce"
	_, err = e.recipes.Update(ctx, r.ID, e.a, recipe.Input{
		Title: "Hack", Servings: 4, Tags: []string{},
		IngredientGroups: []recipe.IngredientGroup{{Name: &name, Ingredients: []recipe.Ingredient{{Quantity: &q, Unit: &g, Name: "Rinderhackfleisch"}}}},
		Steps:            []recipe.Step{{Text: "Kochen."}},
	})
	if err != nil {
		t.Fatal(err)
	}
	l, err = e.svc.Get(ctx, e.a.ID, l.ID)
	if err != nil {
		t.Fatal(err)
	}
	wantQty(t, l.Items[0], 500)
}

func TestDeletedRecipeKeepsItems(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	r := e.hackfleisch(t)
	l, err := e.svc.AddRecipe(ctx, e.a.ID, e.list(t).ID, r.ID, 4, []shopping.IngredientPick{{Group: 0, Index: 0}})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.recipes.Delete(ctx, r.ID, e.a); err != nil {
		t.Fatal(err)
	}
	l, err = e.svc.Get(ctx, e.a.ID, l.ID)
	if err != nil {
		t.Fatal(err)
	}
	en := l.Entries[0]
	if en.Title != "Hack" || en.RecipeID != nil || en.RecipeSlug != nil || en.CoverImageID != nil || len(l.Items) != 1 {
		t.Fatalf("entry = %+v, items = %d", en, len(l.Items))
	}
}

func TestSetServingsRescalesAndKeepsTicks(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	r := e.hackfleisch(t)
	l, err := e.svc.AddRecipe(ctx, e.a.ID, e.list(t).ID, r.ID, 4, []shopping.IngredientPick{{Group: 0, Index: 0}})
	if err != nil {
		t.Fatal(err)
	}
	if l, err = e.svc.SetChecked(ctx, e.a.ID, l.ID, []string{l.Items[0].ID}, true); err != nil {
		t.Fatal(err)
	}
	l, err = e.svc.SetServings(ctx, e.a.ID, l.ID, l.Entries[0].ID, 2)
	if err != nil {
		t.Fatal(err)
	}
	wantQty(t, l.Items[0], 250)
	if !l.Items[0].Checked || l.DoneCount != 1 {
		t.Fatalf("tick lost: %+v", l.Items[0])
	}
	for _, n := range []int{0, 100} {
		if _, err := e.svc.SetServings(ctx, e.a.ID, l.ID, l.Entries[0].ID, n); !errors.Is(err, shopping.ErrInvalid) {
			t.Fatalf("servings %d: %v", n, err)
		}
	}
}

func TestDepartmentOverrideAppliesToAllLists(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	l1, l2 := e.list(t), e.list(t)
	for _, id := range []string{l1.ID, l2.ID} {
		if _, err := e.svc.AddFree(ctx, e.a.ID, id, "Kapern"); err != nil {
			t.Fatal(err)
		}
	}
	l, err := e.svc.AddFree(ctx, e.a.ID, l1.ID, "Quatsch")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.svc.SetDepartment(ctx, e.b.ID, " kaper ", shopping.Chilled); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{l1.ID, l2.ID} {
		if l, err = e.svc.Get(ctx, e.a.ID, id); err != nil {
			t.Fatal(err)
		}
		if l.Items[0].Department != shopping.Chilled {
			t.Fatalf("list %s: %q", id, l.Items[0].Department)
		}
	}
	l, _ = e.svc.Get(ctx, e.a.ID, l1.ID)
	if l.Items[1].Name != "Quatsch" || l.Items[1].Department != shopping.Other {
		t.Fatalf("quatsch = %+v", l.Items[1])
	}
	if err := e.svc.SetDepartment(ctx, e.a.ID, "x", "nope"); !errors.Is(err, shopping.ErrInvalid) {
		t.Fatalf("bad department: %v", err)
	}
}

func TestForeignListIsNotFound(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	r := e.hackfleisch(t)
	l, err := e.svc.AddRecipe(ctx, e.a.ID, e.list(t).ID, r.ID, 4, []shopping.IngredientPick{{Group: 0, Index: 0}})
	if err != nil {
		t.Fatal(err)
	}
	if l, err = e.svc.SetChecked(ctx, e.a.ID, l.ID, []string{l.Items[0].ID}, true); err != nil {
		t.Fatal(err)
	}
	id, itemID, entryID := l.ID, l.Items[0].ID, l.Entries[0].ID
	b := e.b.ID
	calls := map[string]func() error{
		"Get":          func() error { _, err := e.svc.Get(ctx, b, id); return err },
		"Rename":       func() error { _, err := e.svc.Rename(ctx, b, id, "Neu"); return err },
		"Delete":       func() error { return e.svc.Delete(ctx, b, id) },
		"AddRecipe":    func() error { _, err := e.svc.AddRecipe(ctx, b, id, r.ID, 2, nil); return err },
		"SetServings":  func() error { _, err := e.svc.SetServings(ctx, b, id, entryID, 2); return err },
		"RemoveEntry":  func() error { _, err := e.svc.RemoveEntry(ctx, b, id, entryID); return err },
		"AddFree":      func() error { _, err := e.svc.AddFree(ctx, b, id, "Milch"); return err },
		"SetChecked":   func() error { _, err := e.svc.SetChecked(ctx, b, id, []string{itemID}, false); return err },
		"RemoveItems":  func() error { _, err := e.svc.RemoveItems(ctx, b, id, []string{itemID}); return err },
		"ClearChecked": func() error { _, err := e.svc.ClearChecked(ctx, b, id); return err },
	}
	for name, call := range calls {
		if err := call(); !errors.Is(err, shopping.ErrNotFound) {
			t.Errorf("%s: %v, want ErrNotFound", name, err)
		}
	}
	after, err := e.svc.Get(ctx, e.a.ID, id)
	if err != nil {
		t.Fatal(err)
	}
	if after.Name != "Woche" || len(after.Items) != 1 || !after.Items[0].Checked || after.Entries[0].Servings != 4 || !after.UpdatedAt.Equal(l.UpdatedAt) {
		t.Fatalf("list changed: %+v", after)
	}
	if sums, err := e.svc.Lists(ctx, b); err != nil || len(sums) != 0 {
		t.Fatalf("lists of B = %v, %v", sums, err)
	}
}

func TestFreeItemsAndChecked(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	l := e.list(t)
	var err error
	for _, text := range []string{"2 l Milch", "Brot", "Eier"} {
		if l, err = e.svc.AddFree(ctx, e.a.ID, l.ID, text); err != nil {
			t.Fatal(err)
		}
	}
	m := l.Items[0]
	wantQty(t, m, 2)
	if m.Unit == nil || *m.Unit != "l" || m.Name != "Milch" || m.AddedBy == nil || m.AddedBy.ID != e.a.ID {
		t.Fatalf("milk = %+v", m)
	}
	if l.Items[1].Name != "Brot" || l.Items[2].Name != "Eier" || l.OpenCount != 3 {
		t.Fatalf("order/counts: %+v", l.Items)
	}
	if l, err = e.svc.SetChecked(ctx, e.a.ID, l.ID, []string{l.Items[0].ID, l.Items[1].ID}, true); err != nil {
		t.Fatal(err)
	}
	if l.OpenCount != 1 || l.DoneCount != 2 {
		t.Fatalf("counts = %d/%d", l.OpenCount, l.DoneCount)
	}
	if l, err = e.svc.ClearChecked(ctx, e.a.ID, l.ID); err != nil {
		t.Fatal(err)
	}
	if len(l.Items) != 1 || l.Items[0].Name != "Eier" || l.DoneCount != 0 {
		t.Fatalf("after clear: %+v", l.Items)
	}
	if l, err = e.svc.RemoveItems(ctx, e.a.ID, l.ID, []string{l.Items[0].ID}); err != nil || len(l.Items) != 0 {
		t.Fatalf("remove: %v %+v", err, l.Items)
	}
	for _, text := range []string{"", "   "} {
		if _, err := e.svc.AddFree(ctx, e.a.ID, l.ID, text); !errors.Is(err, shopping.ErrInvalid) {
			t.Fatalf("text %q: %v", text, err)
		}
	}
	if _, err := e.svc.Create(ctx, e.a.ID, " "); !errors.Is(err, shopping.ErrInvalid) {
		t.Fatalf("blank name: %v", err)
	}
	if _, err := e.svc.Rename(ctx, e.a.ID, l.ID, ""); !errors.Is(err, shopping.ErrInvalid) {
		t.Fatalf("blank rename: %v", err)
	}
}

func TestListsNewestFirstWithRecipes(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	r := e.hackfleisch(t)
	at := time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)
	e.svc.SetClock(func() time.Time { at = at.Add(time.Minute); return at })
	first := e.list(t)
	second, err := e.svc.Create(ctx, e.a.ID, "Zweite")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.AddRecipe(ctx, e.a.ID, first.ID, r.ID, 3, []shopping.IngredientPick{{Group: 0, Index: 0}}); err != nil {
		t.Fatal(err)
	}
	sums, err := e.svc.Lists(ctx, e.a.ID)
	if err != nil || len(sums) != 2 {
		t.Fatalf("lists = %v, %v", sums, err)
	}
	if sums[0].ID != first.ID {
		t.Fatalf("the touched list must come first: %+v", sums)
	}
	byID := map[string]shopping.Summary{sums[0].ID: sums[0], sums[1].ID: sums[1]}
	if got := byID[first.ID].Recipes; len(got) != 1 || got[0].Title != "Hack" || got[0].Servings != 3 {
		t.Fatalf("recipes = %+v", got)
	}
	if byID[second.ID].Recipes == nil {
		t.Fatal("recipes must be an empty slice, not nil")
	}
}

func TestCountsByCreator(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	e.list(t)
	e.list(t)
	got, err := e.svc.CountsByCreator(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[e.a.ID] != 2 {
		t.Fatalf("counts = %v", got)
	}
}

func TestUserDeletionDropsOwnLists(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	l, err := e.svc.Create(ctx, e.b.ID, "Bens Liste")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.AddFree(ctx, e.b.ID, l.ID, "Brot"); err != nil {
		t.Fatal(err)
	}
	if err := e.users.Delete(ctx, e.a, e.b.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Get(ctx, e.b.ID, l.ID); !errors.Is(err, shopping.ErrNotFound) {
		t.Fatalf("after delete: %v", err)
	}
	var n int
	if err := e.conn.QueryRow(`SELECT COUNT(*) FROM shopping_list_items`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("items left = %d, %v", n, err)
	}
}

func TestAddRecipeUnknownRecipeAndEmptyInclude(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	r := e.hackfleisch(t)
	l := e.list(t)
	ref := []shopping.IngredientPick{{Group: 0, Index: 0}}
	if _, err := e.svc.AddRecipe(ctx, e.a.ID, l.ID, "no-such-recipe", 2, ref); !errors.Is(err, shopping.ErrRecipeNotFound) {
		t.Errorf("unknown recipe: %v, want ErrRecipeNotFound", err)
	}
	// A foreign list wins over an unknown recipe.
	if _, err := e.svc.AddRecipe(ctx, e.b.ID, l.ID, "no-such-recipe", 2, ref); !errors.Is(err, shopping.ErrNotFound) {
		t.Errorf("foreign list: %v, want ErrNotFound", err)
	}
	if _, err := e.svc.AddRecipe(ctx, e.a.ID, l.ID, r.ID, 2, nil); !errors.Is(err, shopping.ErrInvalid) {
		t.Errorf("empty include: %v, want ErrInvalid", err)
	}
}

func TestCountsAreRowsOfMergedNames(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	l := e.list(t)
	var err error
	// "Zwiebel" and "Zwiebeln" are one row, "Brot" another.
	for _, text := range []string{"Zwiebel", "2 Zwiebeln", "Brot"} {
		if l, err = e.svc.AddFree(ctx, e.a.ID, l.ID, text); err != nil {
			t.Fatal(err)
		}
	}
	counts := func(when string, open, done int) {
		t.Helper()
		if l.OpenCount != open || l.DoneCount != done {
			t.Errorf("%s: list counts %d/%d, want %d/%d", when, l.OpenCount, l.DoneCount, open, done)
		}
		sums, err := e.svc.Lists(ctx, e.a.ID)
		if err != nil || len(sums) != 1 {
			t.Fatalf("lists = %v, %v", sums, err)
		}
		if sums[0].OpenCount != open || sums[0].DoneCount != done {
			t.Errorf("%s: overview counts %d/%d, want %d/%d", when, sums[0].OpenCount, sums[0].DoneCount, open, done)
		}
	}
	counts("fresh", 2, 0)
	// Half the onion row ticked: still one open row, nothing done.
	if l, err = e.svc.SetChecked(ctx, e.a.ID, l.ID, []string{l.Items[0].ID}, true); err != nil {
		t.Fatal(err)
	}
	counts("half ticked", 2, 0)
	if l, err = e.svc.SetChecked(ctx, e.a.ID, l.ID, []string{l.Items[1].ID}, true); err != nil {
		t.Fatal(err)
	}
	counts("fully ticked", 1, 1)
}
