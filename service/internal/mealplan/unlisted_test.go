package mealplan_test

import (
	"context"
	"testing"
	"time"

	"github.com/s-frei/rezepte/service/internal/mealplan"
)

// Unlisted counts the recipes still to shop for: on no list, on the
// reader's today or later, or on the tray; a past day never counts. The plan
// read and the plans week answer the same count for the same reader.
func TestUnlistedCountsFromTheReadersToday(t *testing.T) {
	ctx := context.Background()
	e := setup(t) // Wednesday 2026-11-18, 12:00 UTC
	p := e.plan(t)
	r := e.curry(t)
	plan := func(day *string) {
		t.Helper()
		if _, err := e.svc.Add(ctx, e.a.ID, p.ID, mon, sun, mealplan.AddEntry{Day: day, RecipeID: &r.ID}); err != nil {
			t.Fatal(err)
		}
	}
	plan(ptr("2026-11-17")) // yesterday
	plan(ptr("2026-11-18")) // today
	plan(ptr("2026-11-21")) // Saturday
	plan(nil)               // the tray
	plan(ptr("2026-11-10")) // last week
	plan(ptr("2026-11-24")) // next week
	e.add(t, p.ID, ptr("2026-11-19"), "Reste")

	count := func(ctx context.Context, from, to string) int {
		t.Helper()
		got, err := e.svc.Get(ctx, e.a.ID, p.ID, from, to)
		if err != nil {
			t.Fatal(err)
		}
		weeks, _, err := e.svc.PlansWeek(ctx, e.a.ID, from, to)
		if err != nil {
			t.Fatal(err)
		}
		for _, w := range weeks {
			if w.ID == p.ID && w.Unlisted != got.Unlisted {
				t.Fatalf("%s..%s: plans week %d, plan %d", from, to, w.Unlisted, got.Unlisted)
			}
		}
		return got.Unlisted
	}
	for _, c := range []struct {
		name, today, from, to string
		want                  int
	}{
		{"this week from today", "", mon, sun, 3},                               // today, Saturday, tray
		{"a past week counts only the tray", "", "2026-11-09", "2026-11-15", 1}, // last week's is past
		{"a future week counts every day", "", "2026-11-23", "2026-11-29", 2},   // next Tuesday, tray
		{"the reader's today", "2026-11-19", mon, sun, 2},                       // Wednesday is past for them
		{"a reader a day behind", "2026-11-17", mon, sun, 4},
	} {
		in := ctx
		if c.today != "" {
			in = mealplan.WithToday(ctx, c.today)
		}
		if got := count(in, c.from, c.to); got != c.want {
			t.Errorf("%s: unlisted = %d, want %d", c.name, got, c.want)
		}
	}

	// Without the reader's today the server's date decides: at 23:30 UTC it
	// is still Wednesday on the server, already Thursday in Berlin.
	e.now = time.Date(2026, 11, 18, 23, 30, 0, 0, time.UTC)
	if got := count(ctx, mon, sun); got != 3 {
		t.Errorf("server's Wednesday: unlisted = %d, want 3", got)
	}
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	if got := count(mealplan.WithToday(ctx, e.now.In(berlin).Format("2006-01-02")), mon, sun); got != 2 {
		t.Errorf("Berlin's Thursday: unlisted = %d, want 2", got)
	}
}
