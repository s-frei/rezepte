package demo_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/db/dbtest"
	"github.com/s-frei/rezepte/service/internal/demo"
	"github.com/s-frei/rezepte/service/internal/recipe"
	"github.com/s-frei/rezepte/service/internal/settings"
	"github.com/s-frei/rezepte/service/internal/user"
)

var quiet = slog.New(slog.DiscardHandler)

func TestSeedUploadsEmbeddedPhotos(t *testing.T) {
	cases := []struct {
		locale   user.Locale
		first    string   // the overview's first title
		withSet  string   // a sample with three photos
		withNone []string // the samples left without a photo
	}{
		{"en", "Shepherd's Pie", "shepherd-s-pie", []string{"leek-and-potato-soup", "coronation-chicken-sandwiches", "bangers-and-mash"}},
		{"de", "Königsberger Klopse", "koenigsberger-klopse", []string{"linseneintopf", "kartoffelsalat", "frikadellen"}},
	}
	for _, c := range cases {
		t.Run(string(c.locale), func(t *testing.T) {
			seedPhotos(t, c.locale, c.first, c.withSet, c.withNone)
		})
	}
}

func seedPhotos(t *testing.T, locale user.Locale, first, withSet string, withNone []string) {
	ctx := context.Background()
	conn := dbtest.Open(t)
	if _, err := user.NewService(conn, "").Create(ctx, user.CreateParams{Username: "demo", Password: "demo1234", Role: user.RoleAdmin}); err != nil {
		t.Fatal(err)
	}
	imageDir := filepath.Join(t.TempDir(), "images")

	sum, err := demo.Seed(ctx, conn, imageDir, "demo", nil, locale, quiet)
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}
	if sum.Skipped || sum.Recipes != 12 || sum.Images != 27 {
		t.Fatalf("summary = %+v, want 12 recipes, 27 images, not skipped", sum)
	}

	recipes := recipe.NewService(conn, "")
	r, err := recipes.BySlug(ctx, withSet)
	if err != nil {
		t.Fatalf("%s: %v", withSet, err)
	}
	if len(r.Images) != 3 || r.CoverImageID == nil || *r.CoverImageID != r.Images[0].ID {
		t.Fatalf("%s: %d images, cover %v; want 3 with the first as cover", withSet, len(r.Images), r.CoverImageID)
	}
	if r.Images[0].Width != 1600 {
		t.Fatalf("%s cover is %d px wide, want the embedded 1600", withSet, r.Images[0].Width)
	}
	thumb := filepath.Join(imageDir, r.ID, r.Images[0].ID+"_thumb.jpg")
	if _, err := os.Stat(thumb); err != nil {
		t.Fatalf("thumb variant missing: %v", err)
	}
	for _, slug := range withNone {
		r, err := recipes.BySlug(ctx, slug)
		if err != nil {
			t.Fatalf("%s: %v", slug, err)
		}
		if r.CoverImageID != nil || len(r.Images) != 0 {
			t.Fatalf("%s should have no photo, got %d", slug, len(r.Images))
		}
	}

	// The photos are part of writing a sample, not an edit of it: a sample
	// whose uploads ended in a later second than its create would carry a
	// "Last edited" line in some seeds and not in others.
	for _, id := range sum.RecipeIDs {
		r, err := recipes.ByID(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if !r.UpdatedAt.Equal(r.CreatedAt) || r.UpdatedBy.ID != r.CreatedBy.ID {
			t.Errorf("%s: updated %v by %s, want its creation, %v by %s",
				r.Slug, r.UpdatedAt, r.UpdatedBy.ID, r.CreatedAt, r.CreatedBy.ID)
		}
	}

	page, err := recipes.List(ctx, recipe.ListParams{Page: 1, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 12 || len(page.Items) != 1 || page.Items[0].Title != first {
		t.Fatalf("overview: total %d, first %q; want 12 and %q first", page.Total, page.Items[0].Title, first)
	}
}

func TestSeedIsIdempotent(t *testing.T) {
	ctx := context.Background()
	conn := dbtest.Open(t)
	if _, err := user.NewService(conn, "").Create(ctx, user.CreateParams{Username: "demo", Password: "demo1234", Role: user.RoleAdmin}); err != nil {
		t.Fatal(err)
	}
	imageDir := filepath.Join(t.TempDir(), "images")
	if _, err := demo.Seed(ctx, conn, imageDir, "demo", nil, "de", quiet); err != nil {
		t.Fatal(err)
	}
	sum, err := demo.Seed(ctx, conn, imageDir, "demo", nil, "de", quiet) // "de": Count below expects the German fixture count
	if err != nil {
		t.Fatalf("second Seed: %v", err)
	}
	if !sum.Skipped || sum.Recipes != 0 || sum.Images != 0 {
		t.Fatalf("second summary = %+v, want skipped and zero counts", sum)
	}
	if n, _ := recipe.NewService(conn, "").Count(ctx); n != 12 {
		t.Fatalf("recipes after second seed = %d, want 12", n)
	}
}

func TestSeedNeedsAUserAndFallsBackToTheFirst(t *testing.T) {
	ctx := context.Background()
	conn := dbtest.Open(t)
	imageDir := filepath.Join(t.TempDir(), "images")
	if _, err := demo.Seed(ctx, conn, imageDir, "demo", nil, "de", quiet); !errors.Is(err, demo.ErrNoUsers) {
		t.Fatalf("Seed without users: err = %v, want ErrNoUsers", err)
	}
	sam, err := user.NewService(conn, "").Create(ctx, user.CreateParams{Username: "sam", Password: "sam-password", Role: user.RoleAdmin})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := demo.Seed(ctx, conn, imageDir, "demo", nil, "de", quiet); err != nil { // "de": the slug below is the German fixture's
		t.Fatalf("Seed with fallback owner: %v", err)
	}
	r, err := recipe.NewService(conn, "").BySlug(ctx, "flammkuchen")
	if err != nil {
		t.Fatal(err)
	}
	if r.CreatedBy.ID != sam.ID {
		t.Fatalf("owner = %s, want %s (first user)", r.CreatedBy.ID, sam.ID)
	}
}

// The demo's other members exist so that what one member shows another -
// tasty marks, for now - is on screen from the first start. Each signs in
// with the development-credential password, and the first sample, the
// overview's top card, carries two marks.
func TestSeedMembersMarkTheSamplesTasty(t *testing.T) {
	ctx := context.Background()
	conn := dbtest.Open(t)
	users := user.NewService(conn, "")
	if _, err := users.Create(ctx, user.CreateParams{Username: "demo", Password: "demo1234", Role: user.RoleAdmin}); err != nil {
		t.Fatal(err)
	}
	members, err := demo.AddMembers(ctx, conn, "de", quiet)
	if err != nil {
		t.Fatal(err)
	}
	sum, err := demo.Seed(ctx, conn, filepath.Join(t.TempDir(), "images"), "demo", members, "de", quiet)
	if err != nil {
		t.Fatal(err)
	}

	if err := demo.SeedMembers(ctx, conn, sum, "demo", ""); err != nil {
		t.Fatalf("SeedMembers: %v", err)
	}

	for _, name := range demo.Members {
		u, err := users.Authenticate(ctx, name, name+"1234")
		if err != nil {
			t.Fatalf("sign in as %s: %v", name, err)
		}
		if u.Role != user.RoleUser || u.Locale != "de" {
			t.Fatalf("%s: role %s, locale %s; want user, de", name, u.Role, u.Locale)
		}
	}
	recipes := recipe.NewService(conn, "")
	top, err := recipes.BySlug(ctx, "koenigsberger-klopse")
	if err != nil {
		t.Fatal(err)
	}
	if top.TastyCount != 2 || len(top.TastyBy) != 2 {
		t.Fatalf("top card: tastyCount %d, tastyBy %v; want 2 marks", top.TastyCount, top.TastyBy)
	}
	var marks int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM tasty`).Scan(&marks); err != nil {
		t.Fatal(err)
	}
	if marks != 6 {
		t.Fatalf("tasty marks = %d, want 6", marks)
	}
}

// With an issuer, jonas is pre-linked to the identity the test Dex issues
// for him (subject checked against a live Dex, see
// TestDexSubjectMatchesLiveDex), so the demo shows "Signs in with …" and
// signs him in through Dex without anyone connecting an account first.
// Without an issuer, OIDC is off and nothing is linked. Calling SeedMembers
// a second time must not error either way, the same idempotence LinkIdentity
// itself gives a repeated link.
func TestSeedMembersLinksJonasToAnIdentityWhenOIDCIsConfigured(t *testing.T) {
	const issuer = "http://localhost:5656/dex"
	const jonasSubject = "CgVqb25hcxIFbG9jYWw" // dexSubject("jonas", "local")

	ctx := context.Background()
	conn := dbtest.Open(t)
	users := user.NewService(conn, "")
	if _, err := users.Create(ctx, user.CreateParams{Username: "demo", Password: "demo1234", Role: user.RoleAdmin}); err != nil {
		t.Fatal(err)
	}
	members, err := demo.AddMembers(ctx, conn, "en", quiet)
	if err != nil {
		t.Fatal(err)
	}
	sum, err := demo.Seed(ctx, conn, filepath.Join(t.TempDir(), "images"), "demo", members, "en", quiet)
	if err != nil {
		t.Fatal(err)
	}

	if err := demo.SeedMembers(ctx, conn, sum, "demo", ""); err != nil {
		t.Fatalf("SeedMembers without an issuer: %v", err)
	}
	if _, err := users.ByIdentity(ctx, issuer, jonasSubject); !errors.Is(err, user.ErrNotFound) {
		t.Fatalf("ByIdentity without OIDC configured: err = %v, want ErrNotFound", err)
	}

	if err := demo.SeedMembers(ctx, conn, sum, "demo", issuer); err != nil {
		t.Fatalf("SeedMembers with an issuer: %v", err)
	}
	jonas, err := users.ByIdentity(ctx, issuer, jonasSubject)
	if err != nil {
		t.Fatalf("ByIdentity after linking: %v", err)
	}
	if jonas.Username != "jonas" {
		t.Fatalf("identity resolved to %q, want jonas", jonas.Username)
	}

	// A second run over the same identity is the no-op LinkIdentity itself
	// promises, not an error.
	if err := demo.SeedMembers(ctx, conn, sum, "demo", issuer); err != nil {
		t.Fatalf("second SeedMembers with an issuer: %v", err)
	}
}

// An instance that already holds recipes gets no members, and neither marks
// nor links: they belong to the sample data, not to an instance in use.
func TestMembersFollowTheSeed(t *testing.T) {
	ctx := context.Background()
	conn := dbtest.Open(t)
	users := user.NewService(conn, "")
	if _, err := users.Create(ctx, user.CreateParams{Username: "demo", Password: "demo1234", Role: user.RoleSuperadmin}); err != nil {
		t.Fatal(err)
	}
	if _, err := demo.Seed(ctx, conn, filepath.Join(t.TempDir(), "images"), "demo", nil, "de", quiet); err != nil {
		t.Fatal(err)
	}
	members, err := demo.AddMembers(ctx, conn, "de", quiet)
	if err != nil {
		t.Fatalf("AddMembers on a seeded instance: %v", err)
	}
	if len(members) != 0 {
		t.Fatalf("members on a seeded instance = %v, want none", members)
	}
	if err := demo.SeedMembers(ctx, conn, demo.Summary{Skipped: true}, "demo", ""); err != nil {
		t.Fatalf("SeedMembers after a skipped seed: %v", err)
	}
	if list, _ := users.List(ctx); len(list) != 1 {
		t.Fatalf("users after a skipped seed = %d, want 1", len(list))
	}
	var shares int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM shares`).Scan(&shares); err != nil {
		t.Fatal(err)
	}
	if shares != 0 {
		t.Fatalf("shares after a skipped seed = %d, want none", shares)
	}
}

// Everybody in the demo, the admin included, gets a pantry name in the
// demo's language that differs from their login name and keeps its initial.
func TestMembersAndAdminGetPantryNames(t *testing.T) {
	for locale, want := range map[user.Locale]map[string]string{
		"de": {"demo": "Dattel Dill", "mila": "Mila Majoran", "jonas": "Jonas Zimt"},
		"en": {"demo": "Damson Dill", "mila": "Mila Marjoram", "jonas": "Jonas Cinnamon"},
	} {
		t.Run(string(locale), func(t *testing.T) {
			ctx := context.Background()
			conn := dbtest.Open(t)
			users := user.NewService(conn, "")
			if err := users.EnsureSuperadmin(ctx, demo.AdminUser, demo.AdminPassword); err != nil {
				t.Fatal(err)
			}
			if _, err := demo.AddMembers(ctx, conn, locale, quiet); err != nil {
				t.Fatal(err)
			}
			list, err := users.List(ctx)
			if err != nil {
				t.Fatal(err)
			}
			got := map[string]string{}
			for _, u := range list {
				got[u.Username] = u.DisplayName
			}
			for username, name := range want {
				if got[username] != name {
					t.Errorf("display name of %s = %q, want %q", username, got[username], name)
				}
			}
		})
	}
}

// A member name somebody already took is left to them: that account gets no
// password, recipes or marks from the demo.
func TestMembersLeaveATakenNameAlone(t *testing.T) {
	ctx := context.Background()
	conn := dbtest.Open(t)
	users := user.NewService(conn, "")
	if _, err := users.Create(ctx, user.CreateParams{Username: "demo", Password: "demo1234", Role: user.RoleAdmin}); err != nil {
		t.Fatal(err)
	}
	taken, err := users.Create(ctx, user.CreateParams{Username: demo.Members[0], Password: "their-own-pw", Role: user.RoleUser})
	if err != nil {
		t.Fatal(err)
	}
	members, err := demo.AddMembers(ctx, conn, "de", quiet)
	if err != nil {
		t.Fatal(err)
	}
	sum, err := demo.Seed(ctx, conn, filepath.Join(t.TempDir(), "images"), "demo", members, "de", quiet)
	if err != nil {
		t.Fatal(err)
	}
	if err := demo.SeedMembers(ctx, conn, sum, "demo", ""); err != nil {
		t.Fatalf("SeedMembers with a taken name: %v", err)
	}
	if _, err := users.Authenticate(ctx, taken.Username, "their-own-pw"); err != nil {
		t.Fatalf("the existing %s lost their password: %v", taken.Username, err)
	}
	for table, column := range map[string]string{"tasty": "user_id", "recipes": "created_by"} {
		var theirs int
		if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table+` WHERE `+column+` = ?`, taken.ID).Scan(&theirs); err != nil {
			t.Fatal(err)
		}
		if theirs != 0 {
			t.Fatalf("the existing %s got %d %s rows, want none", taken.Username, theirs, table)
		}
	}
}

// TestMembersWriteSomeSamples: Jonas and Mila each wrote two samples, none
// of them one they mark tasty, and Mila locked one of hers; the first sample
// stays the admin's.
func TestMembersWriteSomeSamples(t *testing.T) {
	ctx := context.Background()
	conn := dbtest.Open(t)
	if _, err := user.NewService(conn, "").Create(ctx, user.CreateParams{Username: "demo", Password: "demo1234", Role: user.RoleSuperadmin}); err != nil {
		t.Fatal(err)
	}
	members, err := demo.AddMembers(ctx, conn, "en", quiet)
	if err != nil {
		t.Fatal(err)
	}
	sum, err := demo.Seed(ctx, conn, filepath.Join(t.TempDir(), "images"), "demo", members, "en", quiet)
	if err != nil {
		t.Fatal(err)
	}
	if err := demo.SeedMembers(ctx, conn, sum, "demo", ""); err != nil {
		t.Fatal(err)
	}

	perAuthor := map[string]int{}
	rows, err := conn.QueryContext(ctx, `SELECT u.username, COUNT(*) FROM recipes r JOIN users u ON u.id = r.created_by GROUP BY u.username`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var name string
		var n int
		if err := rows.Scan(&name, &n); err != nil {
			t.Fatal(err)
		}
		perAuthor[name] = n
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if perAuthor["demo"] != 8 || perAuthor["jonas"] != 2 || perAuthor["mila"] != 2 {
		t.Fatalf("recipes per author = %v, want demo 8, jonas 2, mila 2", perAuthor)
	}
	var ownMarks int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM tasty t JOIN recipes r ON r.id = t.recipe_id WHERE r.created_by = t.user_id`).Scan(&ownMarks); err != nil {
		t.Fatal(err)
	}
	if ownMarks != 0 {
		t.Fatalf("members marked %d of their own recipes tasty, want none", ownMarks)
	}

	recipes := recipe.NewService(conn, "")
	first, err := recipes.BySlug(ctx, "shepherd-s-pie")
	if err != nil {
		t.Fatal(err)
	}
	if first.CreatedBy.Username != "demo" {
		t.Fatalf("first sample written by %q, want demo", first.CreatedBy.Username)
	}
	locked, err := recipes.BySlug(ctx, "bangers-and-mash")
	if err != nil {
		t.Fatal(err)
	}
	if locked.CreatedBy.Username != "mila" || locked.EditPolicy != recipe.PolicyLocked {
		t.Fatalf("Bangers and Mash: by %q, policy %q; want mila, locked", locked.CreatedBy.Username, locked.EditPolicy)
	}
}

// TestSeedMembersShareSamples: with the instance owner as its admin, the demo
// gives the admin and each member links of their own, none to the first
// sample, which the documentation photographs - and leaves public sharing
// off, as every instance starts, so the links start out paused.
func TestSeedMembersShareSamples(t *testing.T) {
	ctx := context.Background()
	conn := dbtest.Open(t)
	users := user.NewService(conn, "")
	if _, err := users.Create(ctx, user.CreateParams{Username: "demo", Password: "demo1234", Role: user.RoleSuperadmin}); err != nil {
		t.Fatal(err)
	}
	members, err := demo.AddMembers(ctx, conn, "en", quiet)
	if err != nil {
		t.Fatal(err)
	}
	sum, err := demo.Seed(ctx, conn, filepath.Join(t.TempDir(), "images"), "demo", members, "en", quiet)
	if err != nil {
		t.Fatal(err)
	}
	if err := demo.SeedMembers(ctx, conn, sum, "demo", ""); err != nil {
		t.Fatalf("SeedMembers: %v", err)
	}

	st, err := settings.NewService(conn).Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.PublicShares || st.PublicShareDefaultDays == nil || *st.PublicShareDefaultDays != 7 ||
		st.PublicShareMaxDays == nil || *st.PublicShareMaxDays != 30 {
		t.Fatalf("settings = %+v, want sharing off, 7 days by default, 30 at most", st)
	}
	perUser := map[string]int{}
	rows, err := conn.QueryContext(ctx, `SELECT u.username, s.recipe_id FROM shares s JOIN users u ON u.id = s.created_by`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var name, recipeID string
		if err := rows.Scan(&name, &recipeID); err != nil {
			t.Fatal(err)
		}
		if recipeID == sum.RecipeIDs[0] {
			t.Errorf("%s shares the first sample, which the documentation photographs", name)
		}
		perUser[name]++
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if perUser["demo"] != 1 || perUser["mila"] != 2 || perUser["jonas"] != 2 {
		t.Fatalf("links per user = %v, want demo 1, mila 2, jonas 2", perUser)
	}
}

// TestAddMembersInvitesOneWithoutAPassword: the demo's extra member has no
// password, no email and an open setup link, so the people list shows an
// account someone can still claim; the admin and the other members each get
// an unverified sample email.
func TestAddMembersInvitesOneWithoutAPassword(t *testing.T) {
	ctx := context.Background()
	conn := dbtest.Open(t)
	users := user.NewService(conn, "")
	admin, err := users.Create(ctx, user.CreateParams{Username: "demo", Password: "demo1234", Role: user.RoleSuperadmin})
	if err != nil {
		t.Fatal(err)
	}
	members, err := demo.AddMembers(ctx, conn, "en", quiet)
	if err != nil {
		t.Fatal(err)
	}
	sum, err := demo.Seed(ctx, conn, filepath.Join(t.TempDir(), "images"), "demo", members, "en", quiet)
	if err != nil {
		t.Fatal(err)
	}
	if err := demo.SeedMembers(ctx, conn, sum, "demo", ""); err != nil {
		t.Fatalf("SeedMembers: %v", err)
	}

	var invited user.User
	for _, m := range members {
		if m.Username == demo.Invited {
			invited = m
			continue
		}
		if m.Email != m.Username+"@example.com" || m.EmailVerified {
			t.Fatalf("%s: email %q verified %v; want an unverified %s@example.com", m.Username, m.Email, m.EmailVerified, m.Username)
		}
	}
	if invited.ID == "" {
		t.Fatalf("members = %v, want %s among them", members, demo.Invited)
	}
	got, err := users.ByID(ctx, invited.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.HasPassword {
		t.Fatalf("%s has a password, want none", demo.Invited)
	}
	if got.Email != "" {
		t.Fatalf("%s: email %q, want none", demo.Invited, got.Email)
	}
	owner, err := users.ByID(ctx, admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	if owner.Email != "demo@example.com" || owner.EmailVerified {
		t.Fatalf("admin: email %q verified %v; want an unverified demo@example.com", owner.Email, owner.EmailVerified)
	}

	// Issuing the same user's link a second time stays a no-op rather than
	// an error: ReplaceSetupLink upserts on the invited user's id. A real
	// second --demo run never reaches this - Seed's own idempotence
	// (TestSeedIsIdempotent) stops seedDemo short of a second SeedMembers -
	// but SeedMembers's own call to IssueSetupLink should not depend on that
	// guard to stay safe.
	if _, err := auth.NewService(conn, users).IssueSetupLink(ctx, admin, invited.ID); err != nil {
		t.Fatalf("second IssueSetupLink: %v", err)
	}
}

// TestSeedMembersLeaveSharingOffUnderAnotherAdmin: sharing is the owner's
// switch, so an admin who is not the owner gets members but no links.
func TestSeedMembersLeaveSharingOffUnderAnotherAdmin(t *testing.T) {
	ctx := context.Background()
	conn := dbtest.Open(t)
	if _, err := user.NewService(conn, "").Create(ctx, user.CreateParams{Username: "demo", Password: "demo1234", Role: user.RoleAdmin}); err != nil {
		t.Fatal(err)
	}
	members, err := demo.AddMembers(ctx, conn, "en", quiet)
	if err != nil {
		t.Fatal(err)
	}
	sum, err := demo.Seed(ctx, conn, filepath.Join(t.TempDir(), "images"), "demo", members, "en", quiet)
	if err != nil {
		t.Fatal(err)
	}
	if err := demo.SeedMembers(ctx, conn, sum, "demo", ""); err != nil {
		t.Fatalf("SeedMembers: %v", err)
	}
	var n int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM shares`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("shares = %d, want none", n)
	}
}
