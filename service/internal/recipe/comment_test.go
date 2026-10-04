package recipe_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/s-frei/rezepte/service/internal/recipe"
	"github.com/s-frei/rezepte/service/internal/user"
)

// member is a plain user actor for comment tests: createUserWithProfile
// creates admins, which would hide the admin-only delete rule.
func member(t *testing.T, username string) user.User {
	t.Helper()
	conn, ok := testConns[t]
	if !ok {
		t.Fatal("member: call setup(t) first")
	}
	u, err := user.NewService(conn, "").Create(context.Background(), user.CreateParams{
		Username: username, Password: "pw", Role: user.RoleUser, DisplayName: strings.ToUpper(username[:1]) + username[1:],
	})
	if err != nil {
		t.Fatal(err)
	}
	return u
}

// must checks a setup call: must(f())(t) fails t on f's error and returns
// its value otherwise.
func must[T any](v T, err error) func(*testing.T) T {
	return func(t *testing.T) T {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
}

func TestNormalizeCommentBody(t *testing.T) {
	long := strings.Repeat("ä", 2000)
	for _, tc := range []struct {
		name, in, want string
		ok             bool
	}{
		{"plain", "Kapern zum Schluss", "Kapern zum Schluss", true},
		{"trimmed", "  hi \n", "hi", true},
		{"crlf", "a\r\nb\rc", "a\nb\nc", true},
		{"tab kept", "a\tb", "a\tb", true},
		{"empty", "", "", false},
		{"whitespace only", " \n\t ", "", false},
		{"2000 runes", long, long, true},
		{"2000 runes padded", "  " + long + "  ", long, true},
		{"2001 runes", long + "ä", "", false},
		{"control char", "a\x00b", "", false},
		{"escape", "a\x1bb", "", false},
		{"zero-width space only", "\u200B", "", false},
		{"bom only", "\uFEFF", "", false},
		{"soft hyphen only", "\u00AD", "", false},
		{"bidi override", "a\u202Eb", "", false},
		{"joiners only", "\u200D\u200C", "", false},
		{"emoji zwj sequence", "👩\u200D🍳 lecker", "👩\u200D🍳 lecker", true},
		{"zwnj kept", "Auf\u200Clage", "Auf\u200Clage", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := recipe.NormalizeCommentBody(tc.in)
			if tc.ok {
				if err != nil || got != tc.want {
					t.Fatalf("got %q, %v; want %q", got, err, tc.want)
				}
				return
			}
			if !errors.Is(err, recipe.ErrInvalidComment) {
				t.Fatalf("err = %v, want ErrInvalidComment", err)
			}
		})
	}
}

func TestCommentRoundTrip(t *testing.T) {
	ctx := context.Background()
	svc, authorID := setup(t)
	author := adminActor(authorID)
	jana := member(t, "jana")
	r := must(svc.Create(ctx, authorID, loadFixtures(t)[0]))(t)

	c := must(svc.AddComment(ctx, jana, r.ID, "  Kapern zum Schluss  "))(t)
	if c.Body != "Kapern zum Schluss" || c.Author == nil || c.Author.ID != jana.ID || !c.CanEdit || !c.CanDelete || c.New {
		t.Fatalf("added: %+v", c)
	}

	// The author sees Jana's entry as new, Jana does not see her own as new.
	list := must(svc.Comments(ctx, author, r.ID))(t)
	if len(list) != 1 || !list[0].New || list[0].CanEdit || !list[0].CanDelete {
		t.Fatalf("author's view: %+v", list)
	}
	list = must(svc.Comments(ctx, jana, r.ID))(t)
	if list[0].New {
		t.Fatal("own entry must never be new")
	}

	if err := svc.MarkCommentsSeen(ctx, author.ID, r.ID, c.ID); err != nil {
		t.Fatal(err)
	}
	list = must(svc.Comments(ctx, author, r.ID))(t)
	if list[0].New {
		t.Fatal("entry still new after MarkCommentsSeen")
	}

	edited := must(svc.EditComment(ctx, jana, c.ID, "Kapern ganz zum Schluss"))(t)
	if edited.EditedAt == nil || edited.ID != c.ID {
		t.Fatalf("edited: %+v", edited)
	}
	list = must(svc.Comments(ctx, author, r.ID))(t)
	if list[0].New {
		t.Fatal("an edit must not make an entry new again")
	}
}

func TestCommentPermissions(t *testing.T) {
	ctx := context.Background()
	svc, adminID := setup(t)
	admin := adminActor(adminID)
	jana, tom := member(t, "jana"), member(t, "tom")
	r := must(svc.Create(ctx, adminID, loadFixtures(t)[0]))(t)
	c := must(svc.AddComment(ctx, jana, r.ID, "hi"))(t)
	if _, err := svc.EditComment(ctx, tom, c.ID, "x"); !errors.Is(err, recipe.ErrNotCommentAuthor) {
		t.Fatalf("tom edits: %v", err)
	}
	if _, err := svc.EditComment(ctx, admin, c.ID, "x"); !errors.Is(err, recipe.ErrNotCommentAuthor) {
		t.Fatalf("admin edits: %v", err)
	}
	if err := svc.DeleteComment(ctx, tom, c.ID); !errors.Is(err, recipe.ErrCommentDeleteForbidden) {
		t.Fatalf("tom deletes: %v", err)
	}
	if err := svc.DeleteComment(ctx, admin, c.ID); err != nil {
		t.Fatalf("admin deletes: %v", err)
	}
	if err := svc.DeleteComment(ctx, jana, c.ID); !errors.Is(err, recipe.ErrCommentNotFound) {
		t.Fatalf("delete twice: %v", err)
	}
	if _, err := svc.AddComment(ctx, jana, "nope", "hi"); !errors.Is(err, recipe.ErrNotFound) {
		t.Fatalf("missing recipe: %v", err)
	}
	if _, err := svc.Comments(ctx, jana, "nope"); !errors.Is(err, recipe.ErrNotFound) {
		t.Fatalf("list missing recipe: %v", err)
	}
	if err := svc.MarkCommentsSeen(ctx, jana.ID, "nope", 1); !errors.Is(err, recipe.ErrNotFound) {
		t.Fatalf("seen missing recipe: %v", err)
	}
}

func TestCommentIDsAreNotReused(t *testing.T) {
	ctx := context.Background()
	svc, adminID := setup(t)
	admin := adminActor(adminID)
	jana := member(t, "jana")
	r := must(svc.Create(ctx, adminID, loadFixtures(t)[0]))(t)

	first := must(svc.AddComment(ctx, jana, r.ID, "eins"))(t)
	if err := svc.MarkCommentsSeen(ctx, admin.ID, r.ID, first.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteComment(ctx, jana, first.ID); err != nil {
		t.Fatal(err)
	}
	second := must(svc.AddComment(ctx, jana, r.ID, "zwei"))(t)
	if second.ID <= first.ID {
		t.Fatalf("id reused: first %d, second %d", first.ID, second.ID)
	}
	list := must(svc.Comments(ctx, admin, r.ID))(t)
	if len(list) != 1 || !list[0].New {
		t.Fatalf("entry after a deleted newest one must be new: %+v", list)
	}
}

func TestWatermarkIsByIDNotTime(t *testing.T) {
	ctx := context.Background()
	svc, adminID := setup(t)
	admin := adminActor(adminID)
	jana := member(t, "jana")
	r := must(svc.Create(ctx, adminID, loadFixtures(t)[0]))(t)
	// Both entries and the page view share one timestamp.
	fixed := time.Date(2026, 10, 3, 10, 30, 0, 0, time.UTC)
	svc.SetClock(func() time.Time { return fixed })
	first := must(svc.AddComment(ctx, jana, r.ID, "eins"))(t)
	if err := svc.MarkCommentsSeen(ctx, admin.ID, r.ID, first.ID); err != nil {
		t.Fatal(err)
	}
	// Same second as the page view: still new, because it has a higher id.
	must(svc.AddComment(ctx, jana, r.ID, "zwei"))(t)
	list := must(svc.Comments(ctx, admin, r.ID))(t)
	if len(list) != 2 || list[0].New || !list[1].New {
		t.Fatalf("new flags: %v %v", list[0].New, list[1].New)
	}
}

func TestAddCommentLeavesWatermark(t *testing.T) {
	ctx := context.Background()
	svc, adminID := setup(t)
	admin := adminActor(adminID)
	jana := member(t, "jana")
	r := must(svc.Create(ctx, adminID, loadFixtures(t)[0]))(t)
	must(svc.AddComment(ctx, jana, r.ID, "Frage?"))(t)
	// The author writes before the page ever showed Jana's entry: writing
	// does not mark it seen, only opening the page does.
	must(svc.AddComment(ctx, admin, r.ID, "Antwort"))(t)
	list := must(svc.Comments(ctx, admin, r.ID))(t)
	if !list[0].New || list[1].New {
		t.Fatalf("Jana's entry stays new, the own one never is: %+v", list)
	}
	if !cardFor(t, svc, adminID, r.ID).NewComments {
		t.Fatal("the card still dots for the unseen entry")
	}
}

func TestMarkSeenOnlyWhatWasShown(t *testing.T) {
	ctx := context.Background()
	svc, adminID := setup(t)
	admin := adminActor(adminID)
	jana := member(t, "jana")
	r := must(svc.Create(ctx, adminID, loadFixtures(t)[0]))(t)
	first := must(svc.AddComment(ctx, jana, r.ID, "eins"))(t)
	// The page loaded #1; #2 arrives before the page reports it was seen.
	second := must(svc.AddComment(ctx, jana, r.ID, "zwei"))(t)
	if err := svc.MarkCommentsSeen(ctx, admin.ID, r.ID, first.ID); err != nil {
		t.Fatal(err)
	}
	list := must(svc.Comments(ctx, admin, r.ID))(t)
	if list[0].New || !list[1].New {
		t.Fatalf("only the shown entry is seen: %+v", list)
	}
	if !cardFor(t, svc, adminID, r.ID).NewComments {
		t.Fatal("the card still dots for the entry the page never showed")
	}

	// A later, older page view never lowers the watermark.
	if err := svc.MarkCommentsSeen(ctx, admin.ID, r.ID, second.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.MarkCommentsSeen(ctx, admin.ID, r.ID, first.ID); err != nil {
		t.Fatal(err)
	}
	if list := must(svc.Comments(ctx, admin, r.ID))(t); list[1].New {
		t.Fatal("watermark lowered")
	}
}

func TestMarkSeenIsCapped(t *testing.T) {
	ctx := context.Background()
	svc, adminID := setup(t)
	admin := adminActor(adminID)
	jana := member(t, "jana")
	r := must(svc.Create(ctx, adminID, loadFixtures(t)[0]))(t)
	// 0 and an empty diary mark nothing.
	if err := svc.MarkCommentsSeen(ctx, admin.ID, r.ID, 0); err != nil {
		t.Fatal(err)
	}
	if err := svc.MarkCommentsSeen(ctx, admin.ID, r.ID, 999); err != nil {
		t.Fatal(err)
	}
	must(svc.AddComment(ctx, jana, r.ID, "eins"))(t)
	if err := svc.MarkCommentsSeen(ctx, admin.ID, r.ID, 999); err != nil {
		t.Fatal(err)
	}
	must(svc.AddComment(ctx, jana, r.ID, "zwei"))(t)
	list := must(svc.Comments(ctx, admin, r.ID))(t)
	if list[0].New || !list[1].New {
		t.Fatalf("a too-high upTo is capped at the newest entry: %+v", list)
	}
}

func cardFor(t *testing.T, svc *recipe.Service, userID, recipeID string) recipe.Card {
	t.Helper()
	p := must(svc.List(context.Background(), recipe.ListParams{UserID: userID}))(t)
	for _, c := range p.Items {
		if c.ID == recipeID {
			return c
		}
	}
	t.Fatalf("no card %s", recipeID)
	return recipe.Card{}
}

func TestNewCommentsDot(t *testing.T) {
	ctx := context.Background()
	svc, authorID := setup(t)
	jana, tom, bystander := member(t, "jana"), member(t, "tom"), member(t, "olga")
	r := must(svc.Create(ctx, authorID, loadFixtures(t)[0]))(t)

	if cardFor(t, svc, authorID, r.ID).NewComments {
		t.Fatal("no entries, no dot")
	}
	must(svc.AddComment(ctx, jana, r.ID, "Frage"))(t)
	if !cardFor(t, svc, authorID, r.ID).NewComments {
		t.Fatal("author must see the dot")
	}
	if cardFor(t, svc, jana.ID, r.ID).NewComments {
		t.Fatal("own entry must not dot")
	}
	if cardFor(t, svc, bystander.ID, r.ID).NewComments {
		t.Fatal("a member who neither wrote the recipe nor an entry gets no dot")
	}
	if list := must(svc.Comments(ctx, bystander, r.ID))(t); list[0].New {
		t.Fatalf("nor sees an entry flagged on a first visit: %+v", list)
	}
	answer := must(svc.AddComment(ctx, tom, r.ID, "Antwort"))(t)
	if !cardFor(t, svc, jana.ID, r.ID).NewComments {
		t.Fatal("a participant must see the dot after somebody answers")
	}
	if err := svc.MarkCommentsSeen(ctx, authorID, r.ID, answer.ID); err != nil {
		t.Fatal(err)
	}
	if cardFor(t, svc, authorID, r.ID).NewComments {
		t.Fatal("dot must clear after opening")
	}
}

func TestCommentsSurviveTheirAuthor(t *testing.T) {
	ctx := context.Background()
	svc, adminID := setup(t)
	admin := adminActor(adminID)
	jana := member(t, "jana")
	r := must(svc.Create(ctx, adminID, loadFixtures(t)[0]))(t)
	must(svc.AddComment(ctx, jana, r.ID, "Tipp"))(t)
	if err := user.NewService(testConns[t], "").Delete(ctx, admin, jana.ID); err != nil {
		t.Fatal(err)
	}
	list := must(svc.Comments(ctx, admin, r.ID))(t)
	if len(list) != 1 || list[0].Author != nil || !list[0].New || list[0].CanEdit || !list[0].CanDelete {
		t.Fatalf("former member's entry: %+v", list)
	}
	if !cardFor(t, svc, adminID, r.ID).NewComments {
		t.Fatal("a former member's entry still dots the card")
	}
}

// Removing a member hands their recipes to the acting admin, who then
// follows the entries on them. What was written before the handover is not
// news to the admin; what is written after is.
func TestReassignedRecipeShowsOnlyLaterEntriesAsNew(t *testing.T) {
	ctx := context.Background()
	svc, adminID := setup(t)
	admin := adminActor(adminID)
	jana, tom := member(t, "jana"), member(t, "tom")
	r := must(svc.Create(ctx, jana.ID, loadFixtures(t)[0]))(t)
	must(svc.AddComment(ctx, tom, r.ID, "Vorher"))(t)
	if err := user.NewService(testConns[t], "").Delete(ctx, admin, jana.ID); err != nil {
		t.Fatal(err)
	}
	if cardFor(t, svc, adminID, r.ID).NewComments {
		t.Fatal("entries from before the handover must not dot the admin's card")
	}
	must(svc.AddComment(ctx, tom, r.ID, "Nachher"))(t)
	if !cardFor(t, svc, adminID, r.ID).NewComments {
		t.Fatal("an entry after the handover must dot the admin's card")
	}
	list := must(svc.Comments(ctx, admin, r.ID))(t)
	if len(list) != 2 || list[0].New || !list[1].New {
		t.Fatalf("new flags after handover: %+v", list)
	}
}

func TestDeletingARecipeRemovesItsComments(t *testing.T) {
	ctx := context.Background()
	svc, adminID := setup(t)
	admin := adminActor(adminID)
	jana := member(t, "jana")
	r := must(svc.Create(ctx, adminID, loadFixtures(t)[0]))(t)
	c := must(svc.AddComment(ctx, jana, r.ID, "Tipp"))(t)
	if err := svc.MarkCommentsSeen(ctx, adminID, r.ID, c.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(ctx, r.ID, admin); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"recipe_comments", "recipe_comment_reads"} {
		var n int
		if err := testConns[t].QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table+" WHERE recipe_id = ?", r.ID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Fatalf("%s keeps %d rows of the deleted recipe", table, n)
		}
	}
}
