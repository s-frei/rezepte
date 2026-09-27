package recipe_test

import (
	"context"
	"errors"
	"testing"

	"github.com/s-frei/rezepte/service/internal/recipe"
)

func TestTastyRoundTrip(t *testing.T) {
	ctx := context.Background()
	svc, author := setup(t)
	mara := createUserWithProfile(t, "mara", "Mara", "plum")
	created, err := svc.Create(ctx, author, loadFixtures(t)[0])
	if err != nil {
		t.Fatal(err)
	}

	if err := svc.SetTasty(ctx, mara, created.ID, true); err != nil {
		t.Fatal(err)
	}
	// Marking twice must not fail and must not count twice.
	if err := svc.SetTasty(ctx, mara, created.ID, true); err != nil {
		t.Fatalf("second set: %v", err)
	}

	p, err := svc.List(ctx, recipe.ListParams{UserID: mara})
	if err != nil {
		t.Fatal(err)
	}
	if card := p.Items[0]; card.TastyCount != 1 || !card.Tasty {
		t.Fatalf("card for the marker: count=%d tasty=%v", card.TastyCount, card.Tasty)
	}
	p, err = svc.List(ctx, recipe.ListParams{UserID: author})
	if err != nil {
		t.Fatal(err)
	}
	if card := p.Items[0]; card.TastyCount != 1 || card.Tasty {
		t.Fatalf("card for the author: count=%d tasty=%v", card.TastyCount, card.Tasty)
	}

	got, err := svc.ByID(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.TastyCount != 1 || len(got.TastyBy) != 1 || got.TastyBy[0].ID != mara || got.TastyBy[0].DisplayName != "Mara" {
		t.Fatalf("detail: count=%d by=%+v", got.TastyCount, got.TastyBy)
	}

	if err := svc.SetTasty(ctx, mara, created.ID, false); err != nil {
		t.Fatal(err)
	}
	got, err = svc.ByID(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.TastyCount != 0 || got.TastyBy == nil || len(got.TastyBy) != 0 {
		t.Fatalf("after clearing: count=%d by=%#v", got.TastyCount, got.TastyBy)
	}
}

// TestTastyCountsEveryMember checks the count is per recipe, not per
// caller: two members mark the same recipe, and each card on the page
// carries its own count, zero included.
func TestTastyCountsEveryMember(t *testing.T) {
	ctx := context.Background()
	svc, author := setup(t)
	mara := createUser(t, "mara")
	jo := createUserWithProfile(t, "jo", "Jo", "sage")
	fixtures := loadFixtures(t)
	marked, err := svc.Create(ctx, author, fixtures[0])
	if err != nil {
		t.Fatal(err)
	}
	unmarked, err := svc.Create(ctx, author, fixtures[1])
	if err != nil {
		t.Fatal(err)
	}
	for _, uid := range []string{mara, jo} {
		if err := svc.SetTasty(ctx, uid, marked.ID, true); err != nil {
			t.Fatal(err)
		}
	}

	p, err := svc.List(ctx, recipe.ListParams{UserID: jo})
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, c := range p.Items {
		counts[c.ID] = c.TastyCount
	}
	if counts[marked.ID] != 2 || counts[unmarked.ID] != 0 {
		t.Fatalf("counts = %v", counts)
	}

	got, err := svc.ByID(ctx, marked.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.TastyBy) != 2 || got.TastyBy[0].ID != mara || got.TastyBy[1].ID != jo {
		t.Fatalf("tastyBy = %+v", got.TastyBy)
	}
}

// TestAuthorCannotMarkOwnRecipeTasty pins the rule that a tasty mark is
// feedback to somebody else: the author's own mark is refused, and
// clearing one they never could set is the usual no-op.
func TestAuthorCannotMarkOwnRecipeTasty(t *testing.T) {
	ctx := context.Background()
	svc, author := setup(t)
	created, err := svc.Create(ctx, author, loadFixtures(t)[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.SetTasty(ctx, author, created.ID, true); !errors.Is(err, recipe.ErrOwnRecipe) {
		t.Fatalf("author marking own recipe: %v", err)
	}
	if err := svc.SetTasty(ctx, author, created.ID, false); err != nil {
		t.Fatalf("clearing must stay a no-op: %v", err)
	}
	got, err := svc.ByID(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.TastyCount != 0 {
		t.Fatalf("count = %d", got.TastyCount)
	}
}

func TestSetTastyOnMissingRecipeReturnsErrNotFound(t *testing.T) {
	ctx := context.Background()
	svc, uid := setup(t)
	if err := svc.SetTasty(ctx, uid, "missing", true); !errors.Is(err, recipe.ErrNotFound) {
		t.Fatalf("mark missing recipe: %v", err)
	}
	if err := svc.SetTasty(ctx, uid, "missing", false); err != nil {
		t.Fatalf("clear on missing recipe must be a no-op: %v", err)
	}
}

func TestIsTastyRespectsCaller(t *testing.T) {
	ctx := context.Background()
	svc, author := setup(t)
	mara := createUser(t, "mara")
	created, err := svc.Create(ctx, author, loadFixtures(t)[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.SetTasty(ctx, mara, created.ID, true); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		uid  string
		want bool
	}{{mara, true}, {author, false}, {"", false}} {
		got, err := svc.IsTasty(ctx, tc.uid, created.ID)
		if err != nil || got != tc.want {
			t.Fatalf("IsTasty(%q) = %v, %v; want %v", tc.uid, got, err, tc.want)
		}
	}
}

// TestListSortsByTasty pins the "tasty" order: most marks first, a tie
// falls back to most recently updated, and a recipe nobody marked still
// shows up at the end.
func TestListSortsByTasty(t *testing.T) {
	ctx := context.Background()
	svc, author := setup(t)
	mara := createUser(t, "mara")
	jo := createUser(t, "jo")
	fixtures := loadFixtures(t)
	ids := make([]string, 4)
	for i := range ids {
		r, err := svc.Create(ctx, author, fixtures[i])
		if err != nil {
			t.Fatal(err)
		}
		ids[i] = r.ID
	}
	mark := func(uid, id string) {
		t.Helper()
		if err := svc.SetTasty(ctx, uid, id, true); err != nil {
			t.Fatal(err)
		}
	}
	mark(mara, ids[0])
	mark(jo, ids[0])
	mark(mara, ids[2])
	mark(jo, ids[1])

	p, err := svc.List(ctx, recipe.ListParams{Sort: "tasty"})
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, len(p.Items))
	for i, c := range p.Items {
		got[i] = c.ID
	}
	// ids[1] and ids[2] tie on one mark; ids[2] was created later, so it is
	// the more recently updated and comes first.
	want := []string{ids[0], ids[2], ids[1], ids[3]}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}
