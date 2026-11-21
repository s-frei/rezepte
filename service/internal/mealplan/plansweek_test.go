package mealplan_test

import (
	"context"
	"errors"
	"testing"

	"github.com/s-frei/rezepte/service/internal/mealplan"
)

// The plans page carries each plan's week in two queries: its dated entries
// (not the tray, not other weeks), their kinds, the lists the week and the
// tray are on and the recipes on none, as Get answers them for the same
// reader.
func TestPlansWeekSummarizesEachPlansWeek(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	house := e.shared(t)
	dal, gone := e.dal(t), e.curry(t)
	plan := func(day *string, r string) string {
		t.Helper()
		p, err := e.svc.Add(ctx, e.a.ID, house, mon, sun, mealplan.AddEntry{Day: day, RecipeID: &r})
		if err != nil {
			t.Fatal(err)
		}
		for _, en := range p.Entries {
			if sameDay(en.Day, day) && en.RecipeID != nil && *en.RecipeID == r {
				return en.ID
			}
		}
		t.Fatal("no planned entry")
		return ""
	}
	e.add(t, house, ptr("2026-11-17"), "Reste")
	wed := plan(ptr("2026-11-18"), dal.ID)
	plan(ptr("2026-11-19"), gone.ID)
	plan(ptr("2026-11-20"), dal.ID) // on no list
	tray := plan(nil, dal.ID)
	e.add(t, house, ptr("2026-11-09"), "Letzte Woche")
	_, annas := e.toList(t, e.a.ID, house, "", wed)
	_, trayList := e.toList(t, e.a.ID, house, "", tray)
	if err := e.recipes.Delete(ctx, gone.ID, e.a); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Create(ctx, e.b.ID, "Bens Plan"); err != nil {
		t.Fatal(err)
	}
	pending, err := e.svc.Create(ctx, e.b.ID, "Einladung")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Invite(ctx, e.b.ID, pending.ID, e.a.ID); err != nil {
		t.Fatal(err)
	}

	plans, invs, err := e.svc.PlansWeek(ctx, e.a.ID, mon, sun)
	if err != nil {
		t.Fatal(err)
	}
	summaries, _, err := e.svc.Plans(ctx, e.a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != len(summaries) || len(plans) != 2 {
		t.Fatalf("plans = %+v, want anna's %d", plans, len(summaries))
	}
	for i := range plans {
		if plans[i].ID != summaries[i].ID {
			t.Fatalf("order %d = %s, want %s as Plans", i, plans[i].ID, summaries[i].ID)
		}
	}
	if len(invs) != 1 || invs[0].PlanID != pending.ID {
		t.Fatalf("invitations = %+v", invs)
	}
	var w mealplan.WeekPlan
	for _, p := range plans {
		if p.ID == house {
			w = p
		} else if !p.Personal || len(p.Entries) != 0 || p.Unlisted != 0 || len(p.Lists) != 0 {
			t.Fatalf("personal plan = %+v, want an empty week", p)
		}
	}
	if len(w.Entries) != 4 || len(w.Members) != 1 {
		t.Fatalf("household = %+v", w)
	}
	if got := w.Entries[0]; got.Day != "2026-11-17" || got.Kind != "text" || got.Title != "Reste" || got.RecipeID != nil {
		t.Fatalf("Tuesday = %+v", got)
	}
	if got := w.Entries[1]; got.ID != wed || got.Kind != "recipe" || got.RecipeID == nil {
		t.Fatalf("Wednesday = %+v", got)
	}
	// The recipe is gone: the entry is still a recipe, without its id.
	if got := w.Entries[2]; got.Day != "2026-11-19" || got.Kind != "recipe" || got.RecipeID != nil {
		t.Fatalf("Thursday = %+v", got)
	}
	full, err := e.svc.Get(ctx, e.a.ID, house, mon, sun)
	if err != nil {
		t.Fatal(err)
	}
	if len(w.Lists) != 2 || len(full.Lists) != 2 || w.Lists[0] != full.Lists[0] || w.Lists[1] != full.Lists[1] {
		t.Fatalf("week lists %+v, plan lists %+v; want the same two", w.Lists, full.Lists)
	}
	if ids := map[string]bool{w.Lists[0].ID: true, w.Lists[1].ID: true}; !ids[annas] || !ids[trayList] {
		t.Fatalf("lists = %+v, want the week's and the tray's", w.Lists)
	}
	if w.Unlisted != 1 || full.Unlisted != 1 {
		t.Fatalf("unlisted = %d (plan %d), want Friday's", w.Unlisted, full.Unlisted)
	}
	if w.Tray != 1 || len(w.Pending) != 0 {
		t.Fatalf("tray = %d, pending = %+v; want the one tray entry and nobody waiting", w.Tray, w.Pending)
	}

	// Ben is in the household but on none of anna's lists.
	bens, _, err := e.svc.PlansWeek(ctx, e.b.ID, mon, sun)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, p := range bens {
		if p.ID == pending.ID && (len(p.Pending) != 1 || p.Pending[0].ID != e.a.ID) {
			t.Fatalf("ben's invitation plan pending = %+v, want anna", p.Pending)
		}
		if p.ID != house {
			continue
		}
		if len(p.Pending) != 0 {
			t.Fatalf("ben sees anna's pending invitees: %+v", p.Pending)
		}
		found = true
		if len(p.Entries) != 4 || len(p.Lists) != 0 || p.Unlisted != 3 {
			t.Fatalf("ben's household = %+v, want four meals, no lists, three recipes unlisted", p)
		}
	}
	if !found {
		t.Fatal("ben's week has no household")
	}
	if _, _, err := e.svc.PlansWeek(ctx, e.a.ID, mon, "2027-03-01"); !errors.Is(err, mealplan.ErrInvalid) {
		t.Fatalf("a long range: %v", err)
	}
}

func sameDay(a, b *string) bool { return a == nil && b == nil || a != nil && b != nil && *a == *b }

// One entry's preview ignores the range: a day entry and a tray entry each
// come back alone, a free text is refused.
func TestPreviewOne(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	id := e.plan(t).ID
	r := e.dal(t)
	wed := e.planned(t, e.a.ID, id, r, 2)
	e.planned(t, e.a.ID, id, r, 4)
	p, err := e.svc.Add(ctx, e.a.ID, id, mon, sun, mealplan.AddEntry{RecipeID: &r.ID})
	if err != nil {
		t.Fatal(err)
	}
	var tray string
	for _, en := range p.Entries {
		if en.Day == nil {
			tray = en.ID
		}
	}
	for _, c := range []struct {
		entry    string
		day      bool
		servings int
	}{{wed, true, 2}, {tray, false, 4}} {
		got, err := e.svc.PreviewOne(ctx, e.a.ID, id, c.entry)
		if err != nil {
			t.Fatal(err)
		}
		if got.EntryID != c.entry || (got.Day != nil) != c.day || got.Servings != c.servings || len(got.Ingredients) != 2 {
			t.Fatalf("preview of %s = %+v", c.entry, got)
		}
	}
	text := e.add(t, id, ptr("2026-11-18"), "Reste")
	if _, err := e.svc.PreviewOne(ctx, e.a.ID, id, text); !errors.Is(err, mealplan.ErrInvalid) {
		t.Fatalf("free text preview: %v", err)
	}
	if _, err := e.svc.PreviewOne(ctx, e.b.ID, id, wed); !errors.Is(err, mealplan.ErrNotFound) {
		t.Fatalf("ben previews anna's entry: %v", err)
	}
}
