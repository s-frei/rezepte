package mealplan_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/s-frei/rezepte/service/internal/db"
	"github.com/s-frei/rezepte/service/internal/db/dbtest"
	"github.com/s-frei/rezepte/service/internal/db/sqlc"
	"github.com/s-frei/rezepte/service/internal/mealplan"
	"github.com/s-frei/rezepte/service/internal/recipe"
	"github.com/s-frei/rezepte/service/internal/shopping"
	"github.com/s-frei/rezepte/service/internal/user"
)

// The week of 2026-11-16 (Monday) to 2026-11-22 (Sunday).
const mon, sun = "2026-11-16", "2026-11-22"

type env struct {
	conn    *sql.DB
	svc     *mealplan.Service
	lists   *shopping.Service
	recipes *recipe.Service
	a, b    user.User
	now     time.Time
}

func setup(t *testing.T) *env {
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
	lists := shopping.NewService(conn)
	e := &env{
		conn: conn, svc: mealplan.NewService(conn, lists), lists: lists, recipes: recipe.NewService(conn, t.TempDir()),
		a: mk("anna", user.RoleSuperadmin), b: mk("ben", user.RoleUser),
		now: time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC),
	}
	e.svc.SetClock(func() time.Time { return e.now })
	return e
}

func (e *env) curry(t *testing.T) recipe.Recipe {
	t.Helper()
	q, g := 200.0, "g"
	r, err := e.recipes.Create(context.Background(), e.a.ID, recipe.Input{
		Title: "Curry", Servings: 4, Tags: []string{},
		IngredientGroups: []recipe.IngredientGroup{{Ingredients: []recipe.Ingredient{{Quantity: &q, Unit: &g, Name: "Reis"}}}},
		Steps:            []recipe.Step{{Text: "Kochen."}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func (e *env) plan(t *testing.T) mealplan.Plan {
	t.Helper()
	p, err := e.svc.Create(context.Background(), e.a.ID, "Haushalt")
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func ptr[T any](v T) *T { return &v }

// add puts a free-text entry on day (nil: the tray) and returns its id, or
// "" for a day outside this week.
func (e *env) add(t *testing.T, planID string, day *string, title string) string {
	t.Helper()
	p, err := e.svc.Add(context.Background(), e.a.ID, planID, mon, sun, mealplan.AddEntry{Day: day, Title: &title})
	if err != nil {
		t.Fatal(err)
	}
	for _, en := range p.Entries {
		if en.Title == title {
			return en.ID
		}
	}
	return ""
}

// titles returns the entry titles of a day (nil: the tray) in order, and
// fails unless their positions are 0..n-1.
func titles(t *testing.T, p mealplan.Plan, day *string) []string {
	t.Helper()
	var out []string
	for _, en := range p.Entries {
		if (en.Day == nil) != (day == nil) || day != nil && *en.Day != *day {
			continue
		}
		if en.Position != len(out) {
			t.Fatalf("%q at position %d, want %d", en.Title, en.Position, len(out))
		}
		out = append(out, en.Title)
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestPersonalPlanIsCreatedOnce(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	if _, _, err := e.svc.Plans(ctx, e.a.ID); err != nil {
		t.Fatal(err)
	}
	e.plan(t)
	plans, invites, err := e.svc.Plans(ctx, e.a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 2 || len(invites) != 0 {
		t.Fatalf("plans = %+v, invitations = %+v", plans, invites)
	}
	if p := plans[0]; !p.Personal || p.Name != nil || p.Owner.ID != e.a.ID {
		t.Fatalf("first plan = %+v, want anna's personal plan", p)
	}
	if p := plans[1]; p.Personal || p.Name == nil || *p.Name != "Haushalt" {
		t.Fatalf("second plan = %+v", p)
	}
}

func TestCountsByCreatorSkipsThePersonalPlan(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	for _, id := range []string{e.a.ID, e.b.ID} {
		if _, _, err := e.svc.Plans(ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	p := e.plan(t)
	e.plan(t)
	// A member's plan is the owner's, not theirs.
	if _, err := e.svc.Invite(ctx, e.a.ID, p.ID, e.b.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Accept(ctx, e.b.ID, p.ID); err != nil {
		t.Fatal(err)
	}
	got, err := e.svc.CountsByCreator(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[e.a.ID] != 2 {
		t.Fatalf("counts = %v, want anna 2 and ben absent", got)
	}
	if err := e.svc.Delete(ctx, e.a.ID, p.ID); err != nil {
		t.Fatal(err)
	}
	if got, err = e.svc.CountsByCreator(ctx); err != nil || got[e.a.ID] != 1 {
		t.Fatalf("after delete counts = %v, %v, want anna 1", got, err)
	}
}

func TestPersonalPlanCannotBeRenamedDeletedOrShared(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	plans, _, err := e.svc.Plans(ctx, e.a.ID)
	if err != nil {
		t.Fatal(err)
	}
	id := plans[0].ID
	if _, err := e.svc.Rename(ctx, e.a.ID, id, "Meins"); !errors.Is(err, mealplan.ErrPersonal) || !errors.Is(err, mealplan.ErrInvalid) {
		t.Fatalf("rename = %v, want ErrPersonal wrapping ErrInvalid", err)
	}
	if err := e.svc.Delete(ctx, e.a.ID, id); !errors.Is(err, mealplan.ErrPersonal) {
		t.Fatalf("delete = %v, want ErrPersonal", err)
	}
	p, err := e.svc.Get(ctx, e.a.ID, id, mon, sun)
	if err != nil || p.Name != nil || p.Revision != 0 {
		t.Fatalf("plan after refusals = %+v, %v; want unnamed, revision 0", p, err)
	}
	if _, err := e.svc.Invite(ctx, e.a.ID, id, e.b.ID); !errors.Is(err, mealplan.ErrPersonal) {
		t.Fatalf("invite = %v, want ErrPersonal", err)
	}
}

func TestForeignPlanIsNotFound(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	p := e.plan(t)
	if _, err := e.svc.Get(ctx, e.b.ID, p.ID, mon, sun); !errors.Is(err, mealplan.ErrNotFound) {
		t.Fatalf("ben get = %v, want ErrNotFound", err)
	}
	if _, err := e.svc.Add(ctx, e.b.ID, p.ID, mon, sun, mealplan.AddEntry{Title: ptr("Pizza")}); !errors.Is(err, mealplan.ErrNotFound) {
		t.Fatalf("ben add = %v, want ErrNotFound", err)
	}
	if _, err := e.svc.Rename(ctx, e.b.ID, p.ID, "Meins"); !errors.Is(err, mealplan.ErrNotFound) {
		t.Fatalf("ben rename = %v, want ErrNotFound", err)
	}
	if err := e.svc.Delete(ctx, e.b.ID, p.ID); !errors.Is(err, mealplan.ErrNotFound) {
		t.Fatalf("ben delete = %v, want ErrNotFound", err)
	}
	plans, _, err := e.svc.Plans(ctx, e.b.ID)
	if err != nil || len(plans) != 1 || !plans[0].Personal {
		t.Fatalf("ben's plans = %+v, %v; want only his personal plan", plans, err)
	}
}

func TestRenameAndDelete(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	p := e.plan(t)
	got, err := e.svc.Rename(ctx, e.a.ID, p.ID, "  Urlaub Sylt ")
	if err != nil || got.Name == nil || *got.Name != "Urlaub Sylt" {
		t.Fatalf("rename = %+v, %v", got, err)
	}
	if _, err := e.svc.Rename(ctx, e.a.ID, p.ID, " "); !errors.Is(err, mealplan.ErrInvalid) {
		t.Fatalf("blank rename = %v, want ErrInvalid", err)
	}
	if err := e.svc.Delete(ctx, e.a.ID, p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Get(ctx, e.a.ID, p.ID, mon, sun); !errors.Is(err, mealplan.ErrNotFound) {
		t.Fatalf("get after delete = %v, want ErrNotFound", err)
	}
}

func TestAddRecipeCopiesTitleAndServings(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	r := e.curry(t)
	p, err := e.svc.Add(ctx, e.a.ID, e.plan(t).ID, mon, sun, mealplan.AddEntry{Day: ptr("2026-11-18"), RecipeID: &r.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Entries) != 1 {
		t.Fatalf("entries = %+v", p.Entries)
	}
	en := p.Entries[0]
	if en.Title != "Curry" || en.Servings == nil || *en.Servings != 4 || en.RecipeID == nil || *en.RecipeID != r.ID ||
		en.RecipeSlug == nil || *en.RecipeSlug != r.Slug || en.Day == nil || *en.Day != "2026-11-18" ||
		en.CreatedBy == nil || en.CreatedBy.ID != e.a.ID {
		t.Fatalf("entry = %+v", en)
	}
	p, err = e.svc.Add(ctx, e.a.ID, p.ID, mon, sun, mealplan.AddEntry{RecipeID: &r.ID, Servings: ptr(2)})
	if err != nil || *p.Entries[0].Servings != 2 || p.Entries[0].Day != nil {
		t.Fatalf("tray entry with servings = %+v, %v", p.Entries, err)
	}
	if _, err := e.svc.Add(ctx, e.a.ID, p.ID, mon, sun, mealplan.AddEntry{RecipeID: ptr("nope")}); !errors.Is(err, mealplan.ErrRecipeNotFound) {
		t.Fatalf("unknown recipe = %v, want ErrRecipeNotFound", err)
	}
	for _, bad := range []mealplan.AddEntry{
		{},
		{RecipeID: &r.ID, Title: ptr("Curry")},
		{RecipeID: &r.ID, Servings: ptr(100)},
		{RecipeID: &r.ID, Day: ptr("2026-11-31")},
	} {
		if _, err := e.svc.Add(ctx, e.a.ID, p.ID, mon, sun, bad); !errors.Is(err, mealplan.ErrInvalid) {
			t.Fatalf("add %+v = %v, want ErrInvalid", bad, err)
		}
	}
}

func TestAddFreeTextHasNoServings(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	p, err := e.svc.Add(ctx, e.a.ID, e.plan(t).ID, mon, sun, mealplan.AddEntry{Day: ptr(mon), Title: ptr(" Pizza bestellen ")})
	if err != nil {
		t.Fatal(err)
	}
	if en := p.Entries[0]; en.Title != "Pizza bestellen" || en.Servings != nil || en.RecipeID != nil {
		t.Fatalf("entry = %+v", en)
	}
	if _, err := e.svc.SetServings(ctx, e.a.ID, p.ID, p.Entries[0].ID, mon, sun, 3); !errors.Is(err, mealplan.ErrNotFound) {
		t.Fatalf("servings on free text = %v, want ErrNotFound", err)
	}
	for _, bad := range []mealplan.AddEntry{
		{Title: ptr(" ")},
		{Title: ptr(string(make([]rune, 121)))},
		{Title: ptr("Pizza"), Servings: ptr(2)},
	} {
		if _, err := e.svc.Add(ctx, e.a.ID, p.ID, mon, sun, bad); !errors.Is(err, mealplan.ErrInvalid) {
			t.Fatalf("add %+v = %v, want ErrInvalid", bad, err)
		}
	}
}

func TestSetServings(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	r := e.curry(t)
	p, err := e.svc.Add(ctx, e.a.ID, e.plan(t).ID, mon, sun, mealplan.AddEntry{RecipeID: &r.ID})
	if err != nil {
		t.Fatal(err)
	}
	p, err = e.svc.SetServings(ctx, e.a.ID, p.ID, p.Entries[0].ID, mon, sun, 6)
	if err != nil || *p.Entries[0].Servings != 6 {
		t.Fatalf("set servings = %+v, %v", p.Entries, err)
	}
	if _, err := e.svc.SetServings(ctx, e.a.ID, p.ID, p.Entries[0].ID, mon, sun, 0); !errors.Is(err, mealplan.ErrInvalid) {
		t.Fatalf("servings 0 = %v, want ErrInvalid", err)
	}
}

func TestMoveRenumbersBothDays(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	id := e.plan(t).ID
	tue := ptr("2026-11-17")
	a1 := e.add(t, id, tue, "A1")
	e.add(t, id, tue, "A2")
	e.add(t, id, tue, "A3")
	e.add(t, id, nil, "T1")
	t2 := e.add(t, id, nil, "T2")

	p, err := e.svc.Move(ctx, e.a.ID, id, a1, mon, sun, nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got := titles(t, p, tue); !equal(got, []string{"A2", "A3"}) {
		t.Fatalf("tuesday = %v", got)
	}
	if got := titles(t, p, nil); !equal(got, []string{"T1", "A1", "T2"}) {
		t.Fatalf("tray = %v", got)
	}

	// From the tray onto the day, past its end: it lands last.
	p, err = e.svc.Move(ctx, e.a.ID, id, t2, mon, sun, tue, 9)
	if err != nil {
		t.Fatal(err)
	}
	if got := titles(t, p, tue); !equal(got, []string{"A2", "A3", "T2"}) {
		t.Fatalf("tuesday = %v", got)
	}
	if got := titles(t, p, nil); !equal(got, []string{"T1", "A1"}) {
		t.Fatalf("tray = %v", got)
	}

	// Within one day.
	p, err = e.svc.Move(ctx, e.a.ID, id, t2, mon, sun, tue, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := titles(t, p, tue); !equal(got, []string{"T2", "A2", "A3"}) {
		t.Fatalf("tuesday = %v", got)
	}
	if _, err := e.svc.Move(ctx, e.a.ID, id, "nope", mon, sun, tue, 0); !errors.Is(err, mealplan.ErrNotFound) {
		t.Fatalf("move unknown = %v, want ErrNotFound", err)
	}
	if _, err := e.svc.Move(ctx, e.a.ID, id, t2, mon, sun, tue, -1); !errors.Is(err, mealplan.ErrInvalid) {
		t.Fatalf("move to -1 = %v, want ErrInvalid", err)
	}
}

func TestRangeIncludesBothEnds(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	id := e.plan(t).ID
	e.add(t, id, ptr("2026-11-15"), "Before")
	e.add(t, id, ptr(mon), "Monday")
	e.add(t, id, ptr(sun), "Sunday")
	e.add(t, id, ptr("2026-11-23"), "After")
	e.add(t, id, nil, "Tray")
	p, err := e.svc.Get(ctx, e.a.ID, id, mon, sun)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, en := range p.Entries {
		got = append(got, en.Title)
	}
	if !equal(got, []string{"Tray", "Monday", "Sunday"}) {
		t.Fatalf("entries = %v, want tray first, then both ends", got)
	}
	if p.From != mon || p.To != sun {
		t.Fatalf("range = %s..%s", p.From, p.To)
	}
}

func TestHasEntriesLooksBeyondTheRange(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	id := e.plan(t).ID
	has := func(want bool) {
		t.Helper()
		p, err := e.svc.Get(ctx, e.a.ID, id, mon, sun)
		if err != nil {
			t.Fatal(err)
		}
		if p.HasEntries != want {
			t.Fatalf("hasEntries = %v, want %v", p.HasEntries, want)
		}
	}
	has(false)
	title := "Later"
	p, err := e.svc.Add(ctx, e.a.ID, id, "2026-12-07", "2026-12-07", mealplan.AddEntry{Day: ptr("2026-12-07"), Title: &title})
	if err != nil || len(p.Entries) != 1 {
		t.Fatalf("add = %+v, %v", p, err)
	}
	has(true)
	later := p.Entries[0].ID
	if _, err := e.svc.Remove(ctx, e.a.ID, id, later, mon, sun); err != nil {
		t.Fatal(err)
	}
	has(false)
	// Undone within the removal's window, it counts again.
	if _, err := e.svc.Restore(ctx, e.a.ID, id, later, mon, sun); err != nil {
		t.Fatal(err)
	}
	has(true)
	if _, err := e.svc.Remove(ctx, e.a.ID, id, later, mon, sun); err != nil {
		t.Fatal(err)
	}
	// The tray counts, in any span.
	tray := e.add(t, id, nil, "Tray")
	if p, err := e.svc.Get(ctx, e.a.ID, id, "2027-01-04", "2027-01-10"); err != nil || !p.HasEntries {
		t.Fatalf("tray only: hasEntries = %v, %v", p.HasEntries, err)
	}
	if _, err := e.svc.Remove(ctx, e.a.ID, id, tray, mon, sun); err != nil {
		t.Fatal(err)
	}
	has(false)
}

func TestRangeLimit(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	id := e.plan(t).ID
	if _, err := e.svc.Get(ctx, e.a.ID, id, "2026-11-01", "2027-01-02"); err != nil {
		t.Fatalf("62 days apart = %v, want ok", err)
	}
	for _, r := range [][2]string{
		{"2026-11-01", "2027-01-03"}, // 63 days apart
		{sun, mon},
		{"2026-11-16", "16.11.2026"},
		{"", sun},
	} {
		if _, err := e.svc.Get(ctx, e.a.ID, id, r[0], r[1]); !errors.Is(err, mealplan.ErrInvalid) {
			t.Fatalf("range %v = %v, want ErrInvalid", r, err)
		}
	}
}

func TestRemoveAndRestore(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	id := e.plan(t).ID
	e.add(t, id, ptr(mon), "A")
	b := e.add(t, id, ptr(mon), "B")
	e.add(t, id, ptr(mon), "C")
	p, err := e.svc.Remove(ctx, e.a.ID, id, b, mon, sun)
	if err != nil {
		t.Fatal(err)
	}
	if got := titles(t, p, ptr(mon)); !equal(got, []string{"A", "C"}) {
		t.Fatalf("monday after remove = %v", got)
	}
	if _, err := e.svc.Remove(ctx, e.a.ID, id, b, mon, sun); !errors.Is(err, mealplan.ErrNotFound) {
		t.Fatalf("second remove = %v, want ErrNotFound", err)
	}
	p, err = e.svc.Restore(ctx, e.a.ID, id, b, mon, sun)
	if err != nil {
		t.Fatal(err)
	}
	if got := titles(t, p, ptr(mon)); !equal(got, []string{"A", "B", "C"}) {
		t.Fatalf("monday after restore = %v", got)
	}
	if _, err := e.svc.Restore(ctx, e.a.ID, id, b, mon, sun); !errors.Is(err, mealplan.ErrNotFound) {
		t.Fatalf("second restore = %v, want ErrNotFound", err)
	}
}

func TestPurgeDropsOldRemovals(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	id := e.plan(t).ID
	old := e.add(t, id, ptr(mon), "Old")
	fresh := e.add(t, id, ptr(mon), "Fresh")
	if _, err := e.svc.Remove(ctx, e.a.ID, id, old, mon, sun); err != nil {
		t.Fatal(err)
	}
	e.now = e.now.Add(25 * time.Hour)
	if _, err := e.svc.Remove(ctx, e.a.ID, id, fresh, mon, sun); err != nil {
		t.Fatal(err)
	}
	rows := func(entryID string) int {
		var n int
		if err := e.conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM meal_plan_entries WHERE id = ?`, entryID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if rows(old) != 0 {
		t.Fatal("entry removed 25 h ago is still stored")
	}
	if rows(fresh) != 1 {
		t.Fatal("entry removed just now was purged")
	}
	if _, err := e.svc.Restore(ctx, e.a.ID, id, old, mon, sun); !errors.Is(err, mealplan.ErrNotFound) {
		t.Fatalf("restore purged = %v, want ErrNotFound", err)
	}
}

func TestDeletedRecipeKeepsEntry(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	r := e.curry(t)
	p, err := e.svc.Add(ctx, e.a.ID, e.plan(t).ID, mon, sun, mealplan.AddEntry{Day: ptr(mon), RecipeID: &r.ID})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.recipes.Delete(ctx, r.ID, e.a); err != nil {
		t.Fatal(err)
	}
	p, err = e.svc.Get(ctx, e.a.ID, p.ID, mon, sun)
	if err != nil {
		t.Fatal(err)
	}
	if en := p.Entries[0]; en.Title != "Curry" || en.RecipeID != nil || en.RecipeSlug != nil || en.Servings == nil || *en.Servings != 4 {
		t.Fatalf("entry after recipe delete = %+v", en)
	}
}

func TestChangesPublish(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	r := e.curry(t)
	id := e.plan(t).ID
	events, cancel := e.svc.Hub().Subscribe(id)
	defer cancel()
	signaled := func(what string) {
		t.Helper()
		select {
		case <-events:
		default:
			t.Fatalf("%s did not signal", what)
		}
	}
	p, err := e.svc.Add(ctx, e.a.ID, id, mon, sun, mealplan.AddEntry{Day: ptr(mon), RecipeID: &r.ID})
	if err != nil {
		t.Fatal(err)
	}
	signaled("add")
	entry := p.Entries[0].ID
	steps := []struct {
		name string
		run  func() error
	}{
		{"move", func() error { _, err := e.svc.Move(ctx, e.a.ID, id, entry, mon, sun, nil, 0); return err }},
		{"servings", func() error { _, err := e.svc.SetServings(ctx, e.a.ID, id, entry, mon, sun, 2); return err }},
		{"remove", func() error { _, err := e.svc.Remove(ctx, e.a.ID, id, entry, mon, sun); return err }},
		{"restore", func() error { _, err := e.svc.Restore(ctx, e.a.ID, id, entry, mon, sun); return err }},
		{"rename", func() error { _, err := e.svc.Rename(ctx, e.a.ID, id, "Neu"); return err }},
		{"delete", func() error { return e.svc.Delete(ctx, e.a.ID, id) }},
	}
	for _, s := range steps {
		if err := s.run(); err != nil {
			t.Fatalf("%s: %v", s.name, err)
		}
		signaled(s.name)
	}
	if _, err := e.svc.Add(ctx, e.b.ID, id, mon, sun, mealplan.AddEntry{Title: ptr("x")}); err == nil {
		t.Fatal("stranger add on deleted plan succeeded")
	}
	select {
	case <-events:
		t.Fatal("a refused change signaled")
	default:
	}
}

type invited struct{ plan, inviter, invitee string }

type recorder struct{ got []invited }

func (r *recorder) PlanInvited(_ context.Context, planID, inviterID, inviteeID string) {
	r.got = append(r.got, invited{planID, inviterID, inviteeID})
}

// shared returns anna's plan with ben as an accepted member.
func (e *env) shared(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	id := e.plan(t).ID
	if _, err := e.svc.Invite(ctx, e.a.ID, id, e.b.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Accept(ctx, e.b.ID, id); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestInviteAcceptFlow(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	rec := &recorder{}
	e.svc.SetInviteNotifier(rec)
	id := e.plan(t).ID
	p, err := e.svc.Invite(ctx, e.a.ID, id, e.b.ID)
	if err != nil || len(p.Pending) != 1 || len(p.Members) != 0 {
		t.Fatalf("invite = %+v, %v; want ben pending", p, err)
	}
	if want := (invited{id, e.a.ID, e.b.ID}); len(rec.got) != 1 || rec.got[0] != want {
		t.Fatalf("notified %+v, want %+v", rec.got, want)
	}
	_, inv, err := e.svc.Plans(ctx, e.b.ID)
	if err != nil || len(inv) != 1 || inv[0].PlanID != id {
		t.Fatalf("ben's invitations = %+v, %v", inv, err)
	}
	p, err = e.svc.Accept(ctx, e.b.ID, id)
	if err != nil || len(p.Members) != 1 || p.Members[0].ID != e.b.ID {
		t.Fatalf("accept = %+v, %v; want ben a member", p, err)
	}
	if _, err := e.svc.Add(ctx, e.b.ID, id, mon, sun, mealplan.AddEntry{Day: ptr(mon), Title: ptr("Pizza")}); err != nil {
		t.Fatalf("ben add: %v", err)
	}
	if _, err := e.svc.Accept(ctx, e.b.ID, id); !errors.Is(err, mealplan.ErrNotFound) {
		t.Fatalf("second accept = %v, want ErrNotFound", err)
	}
	if _, err := e.svc.Invite(ctx, e.a.ID, id, e.a.ID); !errors.Is(err, mealplan.ErrInvalid) {
		t.Fatalf("invite self = %v, want ErrInvalid", err)
	}
	if _, err := e.svc.Invite(ctx, e.a.ID, id, "nobody"); !errors.Is(err, mealplan.ErrInvalid) {
		t.Fatalf("invite stranger = %v, want ErrInvalid", err)
	}
	if err := e.svc.Leave(ctx, e.b.ID, id); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Get(ctx, e.b.ID, id, mon, sun); !errors.Is(err, mealplan.ErrNotFound) {
		t.Fatalf("ben get after leave = %v, want ErrNotFound", err)
	}
}

func TestInviteIsIdempotent(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	rec := &recorder{}
	e.svc.SetInviteNotifier(rec)
	id := e.plan(t).ID
	for range 2 {
		if _, err := e.svc.Invite(ctx, e.a.ID, id, e.b.ID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := e.svc.Accept(ctx, e.b.ID, id); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Invite(ctx, e.a.ID, id, e.b.ID); err != nil {
		t.Fatal(err)
	}
	if len(rec.got) != 1 {
		t.Fatalf("notified %d times, want once", len(rec.got))
	}
}

func TestPendingInviteeCannotRead(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	id := e.plan(t).ID
	if _, err := e.svc.Invite(ctx, e.a.ID, id, e.b.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Get(ctx, e.b.ID, id, mon, sun); !errors.Is(err, mealplan.ErrNotFound) {
		t.Fatalf("pending get = %v, want ErrNotFound", err)
	}
	if _, err := e.svc.Add(ctx, e.b.ID, id, mon, sun, mealplan.AddEntry{Title: ptr("Pizza")}); !errors.Is(err, mealplan.ErrNotFound) {
		t.Fatalf("pending add = %v, want ErrNotFound", err)
	}
	if err := e.svc.Leave(ctx, e.b.ID, id); err != nil {
		t.Fatalf("decline: %v", err)
	}
	if _, inv, _ := e.svc.Plans(ctx, e.b.ID); len(inv) != 0 {
		t.Fatalf("invitations after decline = %+v", inv)
	}
}

func TestMemberCannotShareOrDelete(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	id := e.shared(t)
	if _, err := e.svc.Invite(ctx, e.b.ID, id, e.a.ID); !errors.Is(err, mealplan.ErrNotFound) {
		t.Fatalf("member invite = %v, want ErrNotFound", err)
	}
	if _, err := e.svc.RemoveMember(ctx, e.b.ID, id, e.b.ID); !errors.Is(err, mealplan.ErrNotFound) {
		t.Fatalf("member remove = %v, want ErrNotFound", err)
	}
	if err := e.svc.Delete(ctx, e.b.ID, id); !errors.Is(err, mealplan.ErrNotFound) {
		t.Fatalf("member delete = %v, want ErrNotFound", err)
	}
}

func TestCreatorCannotLeave(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	id := e.shared(t)
	if err := e.svc.Leave(ctx, e.a.ID, id); !errors.Is(err, mealplan.ErrInvalid) {
		t.Fatalf("creator leave = %v, want ErrInvalid", err)
	}
	if err := e.svc.Leave(ctx, e.a.ID, "missing"); !errors.Is(err, mealplan.ErrNotFound) {
		t.Fatalf("leave unknown = %v, want ErrNotFound", err)
	}
}

func TestRemovedMemberStreamsEnd(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	id := e.shared(t)
	events, cancel := e.svc.Hub().Subscribe(id)
	defer cancel()
	p, err := e.svc.RemoveMember(ctx, e.a.ID, id, e.b.ID)
	if err != nil || len(p.Members) != 0 {
		t.Fatalf("remove = %+v, %v", p, err)
	}
	select {
	case <-events:
	default:
		t.Fatal("removal did not signal, so the removed member's stream would stay open")
	}
	if _, err := e.svc.Get(ctx, e.b.ID, id, mon, sun); !errors.Is(err, mealplan.ErrNotFound) {
		t.Fatalf("removed get = %v, want ErrNotFound", err)
	}
	if _, err := e.svc.RemoveMember(ctx, e.a.ID, id, e.b.ID); !errors.Is(err, mealplan.ErrNotFound) {
		t.Fatalf("remove again = %v, want ErrNotFound", err)
	}
}

func TestAccountDeletionDropsOwnPlans(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	// Anna (owner) deletes ben: his plan goes, his entry in her plan stays.
	annas := e.shared(t)
	bens, err := e.svc.Create(ctx, e.b.ID, "Bens")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Add(ctx, e.b.ID, annas, mon, sun, mealplan.AddEntry{Day: ptr(mon), Title: ptr("Pizza")}); err != nil {
		t.Fatal(err)
	}
	if err := user.NewService(e.conn, "").Delete(ctx, e.a, e.b.ID); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := e.conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM meal_plans WHERE created_by = ? OR id = ?", e.b.ID, bens.ID).Scan(&n); err != nil || n != 0 {
		t.Fatalf("ben's plans left = %d, %v", n, err)
	}
	p, err := e.svc.Get(ctx, e.a.ID, annas, mon, sun)
	if err != nil || len(p.Entries) != 1 || p.Entries[0].CreatedBy != nil || len(p.Members) != 0 {
		t.Fatalf("anna's plan = %+v, %v; want ben's entry kept with no author and no members", p, err)
	}
}

// cover gives r a 1200x1600 cover focused at 0.5,0.66.
func (e *env) cover(t *testing.T, r recipe.Recipe) {
	t.Helper()
	ctx := context.Background()
	q := sqlc.New(e.conn)
	id := "img-" + r.ID
	if _, err := q.InsertImage(ctx, sqlc.InsertImageParams{ID: id, RecipeID: r.ID, Filename: id + ".jpg", Width: 1200, Height: 1600, SizeBytes: 1000, CreatedAt: db.FormatTime(e.now)}); err != nil {
		t.Fatal(err)
	}
	if err := q.SetRecipeCover(ctx, sqlc.SetRecipeCoverParams{CoverImageID: &id, UpdatedBy: e.a.ID, UpdatedAt: db.FormatTime(e.now), ID: r.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := q.UpdateImageFocus(ctx, sqlc.UpdateImageFocusParams{FocusX: 0.5, FocusY: 0.66, RecipeID: r.ID, ID: id}); err != nil {
		t.Fatal(err)
	}
}

func TestEntriesCarryThePhotoShapeAndFocus(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	r := e.curry(t)
	e.cover(t, r)
	p := e.plan(t)
	if _, err := e.svc.Add(ctx, e.a.ID, p.ID, mon, sun, mealplan.AddEntry{Day: ptr(mon), RecipeID: &r.ID}); err != nil {
		t.Fatal(err)
	}
	e.add(t, p.ID, ptr(mon), "Pizza")
	want := recipe.ImageFocus{X: 0.5, Y: 0.66}
	check := func(what string, w, h *int, f recipe.OrNull[recipe.ImageFocus], photo bool) {
		t.Helper()
		if !photo {
			if w != nil || h != nil || f.V != nil {
				t.Errorf("%s without photo: %v x %v at %+v", what, w, h, f.V)
			}
			return
		}
		if w == nil || *w != 1200 || h == nil || *h != 1600 || f.V == nil || *f.V != want {
			t.Errorf("%s: %v x %v at %+v, want 1200 x 1600 at %+v", what, w, h, f.V, want)
		}
	}

	got, err := e.svc.Get(ctx, e.a.ID, p.ID, mon, sun)
	if err != nil {
		t.Fatal(err)
	}
	for _, en := range got.Entries {
		check("plan entry "+en.Title, en.PhotoWidth, en.PhotoHeight, en.PhotoFocus, en.Photo != nil)
	}
	weeks, _, err := e.svc.PlansWeek(ctx, e.a.ID, mon, sun)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, w := range weeks {
		for _, en := range w.Entries {
			n++
			check("week entry "+en.Title, en.PhotoWidth, en.PhotoHeight, en.PhotoFocus, en.Photo != nil)
		}
	}
	if n != 2 {
		t.Fatalf("week entries = %d, want 2", n)
	}
}
