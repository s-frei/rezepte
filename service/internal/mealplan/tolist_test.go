package mealplan_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/s-frei/rezepte/service/internal/mealplan"
	"github.com/s-frei/rezepte/service/internal/recipe"
	"github.com/s-frei/rezepte/service/internal/shopping"
)

// dal is a recipe for 4 with two ingredients: 200 g Linsen, 1 Zwiebel.
func (e *env) dal(t *testing.T) recipe.Recipe {
	t.Helper()
	g, a, b := "g", 200.0, 1.0
	r, err := e.recipes.Create(context.Background(), e.a.ID, recipe.Input{
		Title: "Dal", Servings: 4, Tags: []string{},
		IngredientGroups: []recipe.IngredientGroup{{Ingredients: []recipe.Ingredient{
			{Quantity: &a, Unit: &g, Name: "Linsen"}, {Quantity: &b, Name: "Zwiebel"},
		}}},
		Steps: []recipe.Step{{Text: "Kochen."}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

var both = []shopping.IngredientPick{{Group: 0, Index: 0}, {Group: 0, Index: 1}}

// planned adds recipe r on Wednesday 2026-11-18 at servings as userID and
// returns the entry id.
func (e *env) planned(t *testing.T, userID, planID string, r recipe.Recipe, servings int) string {
	t.Helper()
	p, err := e.svc.Add(context.Background(), userID, planID, mon, sun, mealplan.AddEntry{Day: ptr("2026-11-18"), RecipeID: &r.ID, Servings: &servings})
	if err != nil {
		t.Fatal(err)
	}
	return p.Entries[len(p.Entries)-1].ID
}

// toList puts the entries on a list as userID: listID, or a new list when "".
func (e *env) toList(t *testing.T, userID, planID, listID string, entryIDs ...string) (mealplan.Plan, string) {
	t.Helper()
	in := mealplan.ToList{}
	if listID == "" {
		in.NewListName = ptr("Einkauf 16.–22. Nov.")
	} else {
		in.ListID = &listID
	}
	for _, id := range entryIDs {
		in.Picks = append(in.Picks, mealplan.ToListPick{EntryID: id, Include: both})
	}
	p, id, err := e.svc.ToList(context.Background(), userID, planID, mon, sun, in)
	if err != nil {
		t.Fatal(err)
	}
	return p, id
}

func (e *env) list(t *testing.T, userID, listID string) shopping.List {
	t.Helper()
	l, err := e.lists.Get(context.Background(), userID, listID)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func signaled(t *testing.T, events <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-events:
	default:
		t.Fatalf("%s did not signal the list", what)
	}
}

func TestToListLinksEntriesAtPlannedServings(t *testing.T) {
	e := setup(t)
	id := e.plan(t).ID
	entry := e.planned(t, e.a.ID, id, e.dal(t), 2)
	p, listID := e.toList(t, e.a.ID, id, "", entry)
	l := e.list(t, e.a.ID, listID)
	if l.Name != "Einkauf 16.–22. Nov." || len(l.Entries) != 1 || l.Entries[0].Servings != 2 || len(l.Items) != 2 {
		t.Fatalf("list = %+v", l)
	}
	if *l.Items[0].Quantity != 100 {
		t.Fatalf("Linsen for 2 = %v, want 100", *l.Items[0].Quantity)
	}
	if got := p.Entries[0].Lists; len(got) != 1 || got[0].ID != listID || got[0].OpenCount != 2 {
		t.Fatalf("entry lists = %+v", got)
	}
	if len(p.Lists) != 1 || p.Lists[0].ID != listID || p.Unlisted != 0 {
		t.Fatalf("plan lists = %+v, unlisted %d", p.Lists, p.Unlisted)
	}
}

func TestServingsFollowIntoList(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	id := e.shared(t)
	entry := e.planned(t, e.a.ID, id, e.dal(t), 4)
	_, listID := e.toList(t, e.a.ID, id, "", entry)
	before := e.list(t, e.a.ID, listID).Revision
	events, cancel := e.lists.Hub().Subscribe(listID)
	defer cancel()
	if _, err := e.svc.SetServings(ctx, e.b.ID, id, entry, mon, sun, 6); err != nil {
		t.Fatal(err)
	}
	signaled(t, events, "servings")
	l := e.list(t, e.a.ID, listID)
	if l.Entries[0].Servings != 6 || *l.Items[0].Quantity != 300 || l.Revision <= before {
		t.Fatalf("after servings 6: entry %+v, Linsen %v, revision %d (was %d)", l.Entries[0], *l.Items[0].Quantity, l.Revision, before)
	}
}

// Lists show the planned day, so a move bumps and signals every linked list
// though their rows stay as they are.
func TestMoveBumpsLinkedLists(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	id := e.plan(t).ID
	entry := e.planned(t, e.a.ID, id, e.dal(t), 4)
	_, listID := e.toList(t, e.a.ID, id, "", entry)
	before := e.list(t, e.a.ID, listID)
	events, cancel := e.lists.Hub().Subscribe(listID)
	defer cancel()
	if _, err := e.svc.Move(ctx, e.a.ID, id, entry, mon, sun, ptr(sun), 0); err != nil {
		t.Fatal(err)
	}
	signaled(t, events, "move")
	l := e.list(t, e.a.ID, listID)
	if l.Revision <= before.Revision || len(l.Entries) != 1 || len(l.Items) != len(before.Items) || *l.Entries[0].PlanDay != sun {
		t.Fatalf("after move: revision %d (was %d), %d entries, %d items, day %v", l.Revision, before.Revision, len(l.Entries), len(l.Items), l.Entries[0].PlanDay)
	}
}

func TestRemoveFollowsIntoForeignList(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	id := e.shared(t)
	entry := e.planned(t, e.a.ID, id, e.dal(t), 4)
	_, listID := e.toList(t, e.a.ID, id, "", entry) // anna's private list
	before := e.list(t, e.a.ID, listID).Revision
	events, cancel := e.lists.Hub().Subscribe(listID)
	defer cancel()
	p, err := e.svc.Remove(ctx, e.b.ID, id, entry, mon, sun)
	if err != nil {
		t.Fatal(err)
	}
	signaled(t, events, "ben's remove")
	if len(p.Lists) != 0 {
		t.Fatalf("ben sees lists %+v", p.Lists)
	}
	l := e.list(t, e.a.ID, listID)
	if len(l.Entries) != 0 || len(l.Items) != 0 || l.OpenCount != 0 || l.Revision <= before {
		t.Fatalf("anna's list after ben's remove = %+v (revision was %d)", l, before)
	}
}

func TestRestoreBringsBackTickedBlock(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	id := e.plan(t).ID
	entry := e.planned(t, e.a.ID, id, e.dal(t), 4)
	_, listID := e.toList(t, e.a.ID, id, "", entry)
	ticked := []string{}
	for _, it := range e.list(t, e.a.ID, listID).Items {
		ticked = append(ticked, it.ID)
	}
	if _, err := e.lists.SetChecked(ctx, e.a.ID, listID, ticked, true); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Remove(ctx, e.a.ID, id, entry, mon, sun); err != nil {
		t.Fatal(err)
	}
	events, cancel := e.lists.Hub().Subscribe(listID)
	defer cancel()
	if _, err := e.svc.Restore(ctx, e.a.ID, id, entry, mon, sun); err != nil {
		t.Fatal(err)
	}
	signaled(t, events, "restore")
	l := e.list(t, e.a.ID, listID)
	if len(l.Items) != 2 {
		t.Fatalf("restored items = %+v", l.Items)
	}
	for i, it := range l.Items {
		if it.ID != ticked[i] || !it.Checked {
			t.Fatalf("restored item %d = %+v, want %s ticked", i, it, ticked[i])
		}
	}
}

func TestClearCheckedKeepsHiddenPlannedItems(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	id := e.plan(t).ID
	entry := e.planned(t, e.a.ID, id, e.dal(t), 4)
	_, listID := e.toList(t, e.a.ID, id, "", entry)
	l := e.list(t, e.a.ID, listID)
	if _, err := e.lists.SetChecked(ctx, e.a.ID, listID, []string{l.Items[0].ID}, true); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Remove(ctx, e.a.ID, id, entry, mon, sun); err != nil {
		t.Fatal(err)
	}
	if _, err := e.lists.ClearChecked(ctx, e.a.ID, listID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Restore(ctx, e.a.ID, id, entry, mon, sun); err != nil {
		t.Fatal(err)
	}
	if l := e.list(t, e.a.ID, listID); len(l.Items) != 2 || !l.Items[0].Checked {
		t.Fatalf("restored after clear = %+v", l.Items)
	}
}

func TestPurgeAndDeleteFollowIntoList(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	id := e.plan(t).ID
	r := e.dal(t)
	old, kept := e.planned(t, e.a.ID, id, r, 4), e.planned(t, e.a.ID, id, r, 4)
	_, listID := e.toList(t, e.a.ID, id, "", old, kept)
	if _, err := e.svc.Remove(ctx, e.a.ID, id, old, mon, sun); err != nil {
		t.Fatal(err)
	}
	e.now = e.now.Add(25 * time.Hour)
	other := e.add(t, id, ptr(mon), "Brot")

	before := e.list(t, e.a.ID, listID).Revision
	events, cancel := e.lists.Hub().Subscribe(listID)
	defer cancel()
	if _, err := e.svc.Remove(ctx, e.a.ID, id, other, mon, sun); err != nil { // purges old
		t.Fatal(err)
	}
	signaled(t, events, "purge")
	var n int
	if err := e.conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM shopping_list_entries WHERE list_id = ?`, listID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if l := e.list(t, e.a.ID, listID); n != 1 || l.Revision <= before {
		t.Fatalf("after purge: %d stored entries, revision %d (was %d)", n, l.Revision, before)
	}

	before = e.list(t, e.a.ID, listID).Revision
	if err := e.svc.Delete(ctx, e.a.ID, id); err != nil {
		t.Fatal(err)
	}
	signaled(t, events, "plan delete")
	if l := e.list(t, e.a.ID, listID); len(l.Entries) != 0 || len(l.Items) != 0 || l.Revision <= before {
		t.Fatalf("after plan delete = %+v (revision was %d)", l, before)
	}
}

func TestPlanHidesForeignLists(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	id := e.shared(t)
	entry := e.planned(t, e.a.ID, id, e.dal(t), 4)
	_, listID := e.toList(t, e.a.ID, id, "", entry)
	p, err := e.svc.Get(ctx, e.b.ID, id, mon, sun)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Entries[0].Lists) != 0 || len(p.Lists) != 0 || p.Unlisted != 1 {
		t.Fatalf("ben sees entry lists %+v, plan lists %+v, unlisted %d", p.Entries[0].Lists, p.Lists, p.Unlisted)
	}
	if _, err := e.lists.Get(ctx, e.b.ID, listID); !errors.Is(err, shopping.ErrNotFound) {
		t.Fatalf("ben reads anna's list: %v", err)
	}
}

func TestToListSkipsOnlyAccessibleLinks(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	id := e.shared(t)
	entry := e.planned(t, e.a.ID, id, e.dal(t), 4)
	_, annas := e.toList(t, e.a.ID, id, "", entry)
	e.toList(t, e.a.ID, id, annas, entry)
	if l := e.list(t, e.a.ID, annas); len(l.Entries) != 1 {
		t.Fatalf("anna's second ToList added again: %d entries", len(l.Entries))
	}
	bens, err := e.lists.Create(ctx, e.b.ID, "Bens Einkauf")
	if err != nil {
		t.Fatal(err)
	}
	p, _ := e.toList(t, e.b.ID, id, bens.ID, entry)
	if l := e.list(t, e.b.ID, bens.ID); len(l.Entries) != 1 {
		t.Fatalf("ben's ToList skipped an entry only on anna's list: %d entries", len(l.Entries))
	}
	if got := p.Entries[0].Lists; len(got) != 1 || got[0].ID != bens.ID {
		t.Fatalf("ben's entry lists = %+v", got)
	}
}

func TestToListNeedsListAccess(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	id := e.shared(t)
	entry := e.planned(t, e.a.ID, id, e.dal(t), 4)
	annas, err := e.lists.Create(ctx, e.a.ID, "Annas Einkauf")
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = e.svc.ToList(ctx, e.b.ID, id, mon, sun, mealplan.ToList{ListID: &annas.ID, Picks: []mealplan.ToListPick{{EntryID: entry, Include: both}}})
	if !errors.Is(err, shopping.ErrNotFound) {
		t.Fatalf("ben onto anna's list = %v, want shopping.ErrNotFound", err)
	}
	if _, _, err := e.svc.ToList(ctx, e.b.ID, e.plan(t).ID, mon, sun, mealplan.ToList{NewListName: ptr("x"), Picks: []mealplan.ToListPick{{EntryID: entry, Include: both}}}); !errors.Is(err, mealplan.ErrNotFound) {
		t.Fatalf("ben from anna's unshared plan = %v, want ErrNotFound", err)
	}
	if ls, _ := e.lists.Lists(ctx, e.b.ID); len(ls) != 0 {
		t.Fatalf("refused ToList left lists %+v", ls)
	}
}

func TestDeletedListReleasesEntries(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	id := e.plan(t).ID
	entry := e.planned(t, e.a.ID, id, e.dal(t), 4)
	_, listID := e.toList(t, e.a.ID, id, "", entry)
	if err := e.lists.Delete(ctx, e.a.ID, listID); err != nil {
		t.Fatal(err)
	}
	p, err := e.svc.Get(ctx, e.a.ID, id, mon, sun)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Entries[0].Lists) != 0 || len(p.Lists) != 0 || p.Unlisted != 1 {
		t.Fatalf("after list delete: entry lists %+v, plan lists %+v, unlisted %d", p.Entries[0].Lists, p.Lists, p.Unlisted)
	}
}

func TestFreeTextNeverGoesOnList(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	id := e.plan(t).ID
	entry := e.add(t, id, ptr(mon), "Pizza bestellen")
	_, _, err := e.svc.ToList(ctx, e.a.ID, id, mon, sun, mealplan.ToList{NewListName: ptr("x"), Picks: []mealplan.ToListPick{{EntryID: entry, Include: both}}})
	if !errors.Is(err, mealplan.ErrInvalid) {
		t.Fatalf("free text onto list = %v, want ErrInvalid", err)
	}
	if ls, _ := e.lists.Lists(ctx, e.a.ID); len(ls) != 0 {
		t.Fatalf("refused ToList left lists %+v", ls)
	}
}

func TestListDayOfPlannedEntry(t *testing.T) {
	e := setup(t)
	id := e.plan(t).ID
	entry := e.planned(t, e.a.ID, id, e.dal(t), 4)
	_, listID := e.toList(t, e.a.ID, id, "", entry)
	en := e.list(t, e.a.ID, listID).Entries[0]
	if en.PlanDay == nil || *en.PlanDay != "2026-11-18" || en.PlanID == nil || *en.PlanID != id {
		t.Fatalf("list entry = %+v", en)
	}
}
