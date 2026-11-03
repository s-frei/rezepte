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

// share makes memberID an accepted member of a list (Task 2 adds the invite).
func (e env) share(t *testing.T, listID, memberID string, accepted bool) {
	t.Helper()
	var at *string
	if accepted {
		now := "2026-11-03T10:00:00Z"
		at = &now
	}
	if _, err := e.conn.Exec(`INSERT INTO shopping_list_members (list_id, user_id, invited_at, accepted_at) VALUES (?, ?, ?, ?)`,
		listID, memberID, "2026-11-03T09:00:00Z", at); err != nil {
		t.Fatal(err)
	}
}

func TestMemberCanReadAndChange(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	r := e.hackfleisch(t)
	l := e.list(t)
	e.share(t, l.ID, e.b.ID, true)
	m := e.b.ID
	steps := []func() (shopping.List, error){
		func() (shopping.List, error) { return e.svc.Get(ctx, m, l.ID) },
		func() (shopping.List, error) { return e.svc.Rename(ctx, m, l.ID, "Neu") },
		func() (shopping.List, error) {
			return e.svc.AddRecipe(ctx, m, l.ID, r.ID, 4, []shopping.IngredientPick{{Group: 0, Index: 0}})
		},
		func() (shopping.List, error) { return e.svc.AddFree(ctx, m, l.ID, "Milch") },
	}
	var cur shopping.List
	for i, f := range steps {
		var err error
		if cur, err = f(); err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
	}
	ids := []string{cur.Items[0].ID}
	for _, f := range []struct {
		name string
		f    func() (shopping.List, error)
	}{
		{"SetChecked", func() (shopping.List, error) { return e.svc.SetChecked(ctx, m, l.ID, ids, true) }},
		{"ClearChecked", func() (shopping.List, error) { return e.svc.ClearChecked(ctx, m, l.ID) }},
		{"SetServings", func() (shopping.List, error) { return e.svc.SetServings(ctx, m, l.ID, cur.Entries[0].ID, 2) }},
		{"RemoveEntry", func() (shopping.List, error) { return e.svc.RemoveEntry(ctx, m, l.ID, cur.Entries[0].ID) }},
		{"RemoveItems", func() (shopping.List, error) { return e.svc.RemoveItems(ctx, m, l.ID, ids) }},
	} {
		if _, err := f.f(); err != nil {
			t.Errorf("%s: %v", f.name, err)
		}
	}
	sums, err := e.svc.Lists(ctx, e.a.ID)
	if err != nil || len(sums) != 1 {
		t.Fatalf("lists = %v, %v", sums, err)
	}
	if sums[0].UpdatedBy == nil || sums[0].UpdatedBy.ID != m {
		t.Fatalf("updatedBy = %+v, want member", sums[0].UpdatedBy)
	}
}

func TestMemberCannotDelete(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	l := e.list(t)
	e.share(t, l.ID, e.b.ID, true)
	if err := e.svc.Delete(ctx, e.b.ID, l.ID); !errors.Is(err, shopping.ErrNotFound) {
		t.Fatalf("Delete = %v", err)
	}
	if _, err := e.svc.Get(ctx, e.a.ID, l.ID); err != nil {
		t.Fatal(err)
	}
}

func TestPendingInviteeCannotRead(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	l := e.list(t)
	e.share(t, l.ID, e.b.ID, false)
	if _, err := e.svc.Get(ctx, e.b.ID, l.ID); !errors.Is(err, shopping.ErrNotFound) {
		t.Fatalf("Get = %v", err)
	}
	if _, err := e.svc.AddFree(ctx, e.b.ID, l.ID, "Milch"); !errors.Is(err, shopping.ErrNotFound) {
		t.Fatalf("AddFree = %v", err)
	}
	if sums, err := e.svc.Lists(ctx, e.b.ID); err != nil || len(sums) != 0 {
		t.Fatalf("lists = %v, %v", sums, err)
	}
	got, err := e.svc.Get(ctx, e.a.ID, l.ID)
	if err != nil || len(got.Pending) != 1 || got.Pending[0].ID != e.b.ID || len(got.Members) != 0 {
		t.Fatalf("owner view = %+v, %v", got, err)
	}
}

func TestStrangerCannotRead(t *testing.T) {
	e := setup(t)
	l, err := e.svc.Create(context.Background(), e.b.ID, "Bens")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Get(context.Background(), e.a.ID, l.ID); !errors.Is(err, shopping.ErrNotFound) {
		t.Fatalf("admin Get = %v", err)
	}
}

func TestListsIncludesShared(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	l := e.list(t)
	e.share(t, l.ID, e.b.ID, true)
	own, err := e.svc.Lists(ctx, e.a.ID)
	if err != nil || len(own) != 1 || len(own[0].Members) != 1 || own[0].Members[0].ID != e.b.ID || own[0].Owner.ID != e.a.ID {
		t.Fatalf("owner lists = %+v, %v", own, err)
	}
	mem, err := e.svc.Lists(ctx, e.b.ID)
	if err != nil || len(mem) != 1 || mem[0].Owner.ID != e.a.ID || mem[0].ID != l.ID {
		t.Fatalf("member lists = %+v, %v", mem, err)
	}
}

func TestCreatorCannotLeave(t *testing.T) {
	e := setup(t)
	l := e.list(t)
	if err := e.svc.Leave(context.Background(), e.a.ID, l.ID); !errors.Is(err, shopping.ErrInvalid) {
		t.Fatalf("Leave = %v", err)
	}
}

func TestLeaveEndsAccess(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	l := e.list(t)
	e.share(t, l.ID, e.b.ID, true)
	if err := e.svc.Leave(ctx, e.b.ID, l.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Get(ctx, e.b.ID, l.ID); !errors.Is(err, shopping.ErrNotFound) {
		t.Fatalf("Get = %v", err)
	}
	if got, _ := e.svc.Get(ctx, e.a.ID, l.ID); len(got.Members) != 0 {
		t.Fatalf("members = %v", got.Members)
	}
	if err := e.svc.Leave(ctx, e.b.ID, l.ID); !errors.Is(err, shopping.ErrNotFound) {
		t.Fatalf("second Leave = %v", err)
	}
}

func TestDeletedAccountLeavesFreeItems(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	l := e.list(t)
	e.share(t, l.ID, e.b.ID, true)
	if _, err := e.svc.AddFree(ctx, e.b.ID, l.ID, "Milch"); err != nil {
		t.Fatal(err)
	}
	if err := e.users.Delete(ctx, e.a, e.b.ID); err != nil {
		t.Fatal(err)
	}
	got, err := e.svc.Get(ctx, e.a.ID, l.ID)
	if err != nil || len(got.Items) != 1 || got.Items[0].AddedBy != nil {
		t.Fatalf("after account delete = %+v, %v", got, err)
	}
	if sums, _ := e.svc.Lists(ctx, e.a.ID); len(sums) != 1 || sums[0].UpdatedBy != nil {
		t.Fatalf("summary = %+v", sums)
	}
	e.share(t, l.ID, e.a.ID, true) // a row that must vanish with the list
	if err := e.svc.Delete(ctx, e.a.ID, l.ID); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := e.conn.QueryRow(`SELECT COUNT(*) FROM shopping_list_members`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("members left = %d, %v", n, err)
	}
}

type recorder struct{ got []string }

func (r *recorder) ListInvited(_ context.Context, listID, inviterID, inviteeID string) {
	r.got = append(r.got, listID+"/"+inviterID+"/"+inviteeID)
}

func TestInviteAcceptFlow(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	rec := &recorder{}
	e.svc.SetInviteNotifier(rec)
	l, err := e.svc.AddFree(ctx, e.a.ID, e.list(t).ID, "Milch")
	if err != nil {
		t.Fatal(err)
	}
	got, err := e.svc.Invite(ctx, e.a.ID, l.ID, e.b.ID)
	if err != nil || len(got.Pending) != 1 || got.Pending[0].ID != e.b.ID {
		t.Fatalf("Invite = %+v, %v", got, err)
	}
	if want := []string{l.ID + "/" + e.a.ID + "/" + e.b.ID}; len(rec.got) != 1 || rec.got[0] != want[0] {
		t.Fatalf("notified = %v, want %v", rec.got, want)
	}
	sums, err := e.svc.Lists(ctx, e.b.ID)
	if err != nil || len(sums) != 0 {
		t.Fatalf("invitee lists = %+v, %v", sums, err)
	}
	invs, err := e.svc.Invitations(ctx, e.b.ID)
	if err != nil || len(invs) != 1 || invs[0].ListName != "Woche" || invs[0].From.ID != e.a.ID || invs[0].OpenCount != l.OpenCount || invs[0].OpenCount != 1 {
		t.Fatalf("invitations = %+v, %v", invs, err)
	}
	if _, err := e.svc.Get(ctx, e.b.ID, l.ID); !errors.Is(err, shopping.ErrNotFound) {
		t.Fatalf("Get before accept = %v", err)
	}
	if got, err = e.svc.Accept(ctx, e.b.ID, l.ID); err != nil || got.ID != l.ID || len(got.Pending) != 0 {
		t.Fatalf("Accept = %+v, %v", got, err)
	}
	if got, _ = e.svc.Get(ctx, e.a.ID, l.ID); len(got.Members) != 1 || got.Members[0].ID != e.b.ID || len(got.Pending) != 0 {
		t.Fatalf("owner list = %+v", got)
	}
	if invs, _ = e.svc.Invitations(ctx, e.b.ID); len(invs) != 0 {
		t.Fatalf("invitations after accept = %+v", invs)
	}
}

func TestInviteIsIdempotent(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	rec := &recorder{}
	e.svc.SetInviteNotifier(rec)
	l := e.list(t)
	for range 2 {
		if _, err := e.svc.Invite(ctx, e.a.ID, l.ID, e.b.ID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := e.svc.Accept(ctx, e.b.ID, l.ID); err != nil {
		t.Fatal(err)
	}
	got, err := e.svc.Invite(ctx, e.a.ID, l.ID, e.b.ID)
	if err != nil || len(got.Members) != 1 || len(got.Pending) != 0 {
		t.Fatalf("Invite accepted = %+v, %v", got, err)
	}
	if len(rec.got) != 1 {
		t.Fatalf("notifications = %v", rec.got)
	}
}

func TestInviteRejectsSelfAndUnknown(t *testing.T) {
	e := setup(t)
	l := e.list(t)
	for _, id := range []string{e.a.ID, "nope"} {
		if _, err := e.svc.Invite(context.Background(), e.a.ID, l.ID, id); !errors.Is(err, shopping.ErrInvalid) {
			t.Fatalf("Invite %q = %v", id, err)
		}
	}
}

func TestOnlyCreatorShares(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	l := e.list(t)
	e.share(t, l.ID, e.b.ID, true)
	if _, err := e.svc.Invite(ctx, e.b.ID, l.ID, e.a.ID); !errors.Is(err, shopping.ErrNotFound) {
		t.Fatalf("Invite = %v", err)
	}
	if _, err := e.svc.RemoveMember(ctx, e.b.ID, l.ID, e.b.ID); !errors.Is(err, shopping.ErrNotFound) {
		t.Fatalf("RemoveMember = %v", err)
	}
}

func TestDeclineIsLeave(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	l := e.list(t)
	if _, err := e.svc.Invite(ctx, e.a.ID, l.ID, e.b.ID); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.Leave(ctx, e.b.ID, l.ID); err != nil {
		t.Fatal(err)
	}
	if invs, _ := e.svc.Invitations(ctx, e.b.ID); len(invs) != 0 {
		t.Fatalf("invitations = %+v", invs)
	}
	if got, _ := e.svc.Get(ctx, e.a.ID, l.ID); len(got.Pending) != 0 {
		t.Fatalf("pending = %+v", got.Pending)
	}
}

func TestRemovedMemberLosesAccess(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	l := e.list(t)
	if _, err := e.svc.Invite(ctx, e.a.ID, l.ID, e.b.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Accept(ctx, e.b.ID, l.ID); err != nil {
		t.Fatal(err)
	}
	if got, err := e.svc.RemoveMember(ctx, e.a.ID, l.ID, e.b.ID); err != nil || len(got.Members) != 0 {
		t.Fatalf("RemoveMember = %+v, %v", got, err)
	}
	if _, err := e.svc.Get(ctx, e.b.ID, l.ID); !errors.Is(err, shopping.ErrNotFound) {
		t.Fatalf("Get = %v", err)
	}
	// A pending invitation is withdrawn the same way.
	if _, err := e.svc.Invite(ctx, e.a.ID, l.ID, e.b.ID); err != nil {
		t.Fatal(err)
	}
	if got, err := e.svc.RemoveMember(ctx, e.a.ID, l.ID, e.b.ID); err != nil || len(got.Pending) != 0 {
		t.Fatalf("withdraw = %+v, %v", got, err)
	}
	if _, err := e.svc.RemoveMember(ctx, e.a.ID, l.ID, e.b.ID); !errors.Is(err, shopping.ErrNotFound) {
		t.Fatalf("second RemoveMember = %v", err)
	}
}

func TestAcceptWithoutInvitation(t *testing.T) {
	e := setup(t)
	l := e.list(t)
	if _, err := e.svc.Accept(context.Background(), e.b.ID, l.ID); !errors.Is(err, shopping.ErrNotFound) {
		t.Fatalf("Accept = %v", err)
	}
}
