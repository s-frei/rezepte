package recipe_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/s-frei/rezepte/service/internal/db"
	"github.com/s-frei/rezepte/service/internal/db/dbtest"
	"github.com/s-frei/rezepte/service/internal/db/sqlc"
	"github.com/s-frei/rezepte/service/internal/recipe"
	"github.com/s-frei/rezepte/service/internal/user"
)

func loadFixtures(t *testing.T) []recipe.Input {
	t.Helper()
	in, err := recipe.Samples()
	if err != nil {
		t.Fatal(err)
	}
	if len(in) != 12 {
		t.Fatalf("fixtures = %d, want 12", len(in))
	}
	return in
}

func setup(t *testing.T) (*recipe.Service, string) {
	t.Helper()
	conn := dbtest.Open(t)
	testConns[t] = conn
	t.Cleanup(func() { delete(testConns, t) })
	u, err := user.NewService(conn).Create(context.Background(), "sam", "pw", user.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	return recipe.NewService(conn), u.ID
}

// testConns lets createUser find the database setup opened for the current
// test and add a second user to it, without reaching into the database
// directly from the test and without changing setup's signature - other
// test files in this package call setup(t) expecting exactly its current
// two return values. Keyed by *testing.T rather than a single shared
// variable purely so each test's entry doesn't clobber another's; this map
// is not synchronised and none of these tests call t.Parallel() - adding it
// to any test that uses setup/createUser would race on concurrent map
// access and needs a mutex added here first, not just the key change.
var testConns = map[*testing.T]*sql.DB{}

// createUser adds a second user to the database setup opened for the
// current test, mirroring how setup creates the first one, and returns
// their id.
func createUser(t *testing.T, username string) string { //nolint:unparam // helper mirrors brief signature; every current call site happens to use the same username
	t.Helper()
	conn, ok := testConns[t]
	if !ok {
		t.Fatal("createUser: call setup(t) first")
	}
	u, err := user.NewService(conn).Create(context.Background(), username, "pw", user.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	return u.ID
}

func TestCreateAndReadBack(t *testing.T) {
	ctx := context.Background()
	svc, uid := setup(t)
	in := loadFixtures(t)[0] // Königsberger Klopse
	created, err := svc.Create(ctx, uid, in)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.Slug != "koenigsberger-klopse" || created.ID == "" || created.CreatedBy != uid {
		t.Fatalf("unexpected: %+v", created)
	}
	got, err := svc.BySlug(ctx, "koenigsberger-klopse")
	if err != nil {
		t.Fatalf("BySlug: %v", err)
	}
	if len(got.IngredientGroups) != 2 || got.IngredientGroups[0].Name == nil || *got.IngredientGroups[0].Name != "Klopse" {
		t.Fatalf("groups = %+v", got.IngredientGroups)
	}
	if len(got.Steps) != len(in.Steps) || len(got.Tags) != 2 || got.Tags[0] != "fleisch" {
		t.Fatalf("steps/tags = %d/%v", len(got.Steps), got.Tags)
	}
	if got.Images == nil {
		t.Fatal("images must be an empty array, not null")
	}
}

func TestSlugCollisionGetsNumbered(t *testing.T) {
	ctx := context.Background()
	svc, uid := setup(t)
	in := loadFixtures(t)[7] // Frikadellen
	a, _ := svc.Create(ctx, uid, in)
	b, err := svc.Create(ctx, uid, in)
	if err != nil {
		t.Fatal(err)
	}
	if a.Slug != "frikadellen" || b.Slug != "frikadellen-2" {
		t.Fatalf("slugs = %q, %q", a.Slug, b.Slug)
	}
}

func TestUpdateReplacesChildrenAndKeepsSlug(t *testing.T) {
	ctx := context.Background()
	svc, uid := setup(t)
	fx := loadFixtures(t)
	created, _ := svc.Create(ctx, uid, fx[3]) // Käsespätzle
	in := fx[3]
	in.Title = "Käsespätzle deluxe"
	in.Tags = []string{"Vegetarisch", "neu"}
	in.IngredientGroups = in.IngredientGroups[:1]
	in.Steps = []string{"Alles mischen."}
	updated, err := svc.Update(ctx, created.ID, in)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Slug != created.Slug || updated.Title != "Käsespätzle deluxe" {
		t.Fatalf("slug/title = %q/%q", updated.Slug, updated.Title)
	}
	if len(updated.IngredientGroups) != 1 || len(updated.Steps) != 1 || len(updated.Tags) != 2 || updated.Tags[1] != "vegetarisch" {
		t.Fatalf("children = %+v", updated)
	}
	if _, err := svc.Update(ctx, "missing", in); !errors.Is(err, recipe.ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
}

func TestDeleteCascadesAndDropsOrphanTags(t *testing.T) {
	ctx := context.Background()
	svc, uid := setup(t)
	created, _ := svc.Create(ctx, uid, loadFixtures(t)[10]) // Rote Grütze: süß, dessert
	if err := svc.Delete(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ByID(ctx, created.ID); !errors.Is(err, recipe.ErrNotFound) {
		t.Fatalf("after delete: %v", err)
	}
	if err := svc.Delete(ctx, created.ID); !errors.Is(err, recipe.ErrNotFound) {
		t.Fatalf("second delete: %v", err)
	}
	tags, err := svc.Tags(ctx)
	if err != nil || len(tags) != 0 {
		t.Fatalf("tags after delete = %v, %v", tags, err)
	}
}

func TestAllFixturesCreate(t *testing.T) {
	ctx := context.Background()
	svc, uid := setup(t)
	for _, in := range loadFixtures(t) {
		if _, err := svc.Create(ctx, uid, in); err != nil {
			t.Fatalf("%s: %v", in.Title, err)
		}
	}
}

func TestReservedSlugIsSkipped(t *testing.T) {
	ctx := context.Background()
	svc, uid := setup(t)
	in := loadFixtures(t)[0]
	in.Title = "New"
	created, err := svc.Create(ctx, uid, in)
	if err != nil {
		t.Fatal(err)
	}
	// "new" belongs to the frontend's /recipes/new editor route, so the
	// recipe has to take the numbered slug instead of shadowing it.
	if created.Slug != "new-2" {
		t.Fatalf("slug = %q, want %q", created.Slug, "new-2")
	}
}

func TestLoadListsImagesInPositionOrderWithCover(t *testing.T) {
	ctx := context.Background()
	conn := dbtest.Open(t)
	u, err := user.NewService(conn).Create(ctx, "sam", "pw", user.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	svc := recipe.NewService(conn)
	created, err := svc.Create(ctx, u.ID, loadFixtures(t)[0])
	if err != nil {
		t.Fatal(err)
	}
	q := sqlc.New(conn)
	for i, id := range []string{"img-b", "img-a"} {
		if _, err := q.InsertImage(ctx, sqlc.InsertImageParams{
			ID: id, RecipeID: created.ID, Filename: id + ".jpg", Width: 2400, Height: 1600,
			SizeBytes: 1, Position: int64(1 - i), CreatedAt: db.FormatTime(time.Now()),
		}); err != nil {
			t.Fatal(err)
		}
	}
	cover := "img-a"
	if err := q.SetRecipeCover(ctx, sqlc.SetRecipeCoverParams{CoverImageID: &cover, UpdatedAt: db.FormatTime(time.Now()), ID: created.ID}); err != nil {
		t.Fatal(err)
	}
	got, err := svc.ByID(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Images) != 2 || got.Images[0].ID != "img-a" || got.Images[0].Position != 0 || got.Images[1].ID != "img-b" {
		t.Fatalf("images = %+v", got.Images)
	}
	if got.Images[0].Width != 2400 || got.Images[0].Height != 1600 {
		t.Fatalf("dims = %+v", got.Images[0])
	}
	if got.CoverImageID == nil || *got.CoverImageID != "img-a" {
		t.Fatalf("cover = %v", got.CoverImageID)
	}
}

func TestDeleteRemovesImageDirectory(t *testing.T) {
	ctx := context.Background()
	conn := dbtest.Open(t)
	u, err := user.NewService(conn).Create(ctx, "sam", "pw", user.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	svc := recipe.NewService(conn, recipe.WithImageDir(dir))
	created, err := svc.Create(ctx, u.ID, loadFixtures(t)[1])
	if err != nil {
		t.Fatal(err)
	}
	recipeDir := filepath.Join(dir, created.ID)
	if err := os.MkdirAll(recipeDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(recipeDir, "x.jpg"), []byte("jpg"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(recipeDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("image dir still there: %v", err)
	}
}

func TestFavouriteRoundTrip(t *testing.T) {
	ctx := context.Background()
	svc, uid := setup(t)
	created, err := svc.Create(ctx, uid, loadFixtures(t)[0])
	if err != nil {
		t.Fatal(err)
	}

	if err := svc.SetFavourite(ctx, uid, created.ID, true); err != nil {
		t.Fatal(err)
	}
	p, err := svc.List(ctx, recipe.ListParams{UserID: uid})
	if err != nil {
		t.Fatal(err)
	}
	if !p.Items[0].Favourite {
		t.Fatal("card must report the star")
	}

	// Setting it twice must not fail - the star is a state, not an event.
	if err := svc.SetFavourite(ctx, uid, created.ID, true); err != nil {
		t.Fatalf("second set: %v", err)
	}

	if err := svc.SetFavourite(ctx, uid, created.ID, false); err != nil {
		t.Fatal(err)
	}
	p, err = svc.List(ctx, recipe.ListParams{UserID: uid})
	if err != nil {
		t.Fatal(err)
	}
	if p.Items[0].Favourite {
		t.Fatal("star must be gone")
	}
}

func TestFavouritesAreNotSharedBetweenUsers(t *testing.T) {
	ctx := context.Background()
	svc, uid := setup(t)
	other := createUser(t, "zweite@example.com")
	created, err := svc.Create(ctx, uid, loadFixtures(t)[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.SetFavourite(ctx, uid, created.ID, true); err != nil {
		t.Fatal(err)
	}
	p, err := svc.List(ctx, recipe.ListParams{UserID: other})
	if err != nil {
		t.Fatal(err)
	}
	if p.Items[0].Favourite {
		t.Fatal("another user's star must not show")
	}
}

// TestDeletingFavouriteOnlyAffectsCaller is the write-side counterpart to
// TestFavouritesAreNotSharedBetweenUsers: it isn't enough that user B can't
// see user A's star, user B must also be unable to clear it. Both users
// favourite the same recipe, user B unfavourites it, and user A's star
// must remain.
func TestDeletingFavouriteOnlyAffectsCaller(t *testing.T) {
	ctx := context.Background()
	svc, uid := setup(t)
	other := createUser(t, "zweite@example.com")
	created, err := svc.Create(ctx, uid, loadFixtures(t)[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.SetFavourite(ctx, uid, created.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetFavourite(ctx, other, created.ID, true); err != nil {
		t.Fatal(err)
	}

	if err := svc.SetFavourite(ctx, other, created.ID, false); err != nil {
		t.Fatal(err)
	}

	p, err := svc.List(ctx, recipe.ListParams{UserID: uid})
	if err != nil {
		t.Fatal(err)
	}
	if !p.Items[0].Favourite {
		t.Fatal("user A's star must survive user B's delete")
	}
	p, err = svc.List(ctx, recipe.ListParams{UserID: other})
	if err != nil {
		t.Fatal(err)
	}
	if p.Items[0].Favourite {
		t.Fatal("user B's star must be gone")
	}
}

// TestIsFavouriteRespectsCaller is IsFavourite's isolation check, the
// method the handler uses to fill Recipe.Favourite on the detail
// lookups: only the user who starred a recipe sees it as a favourite
// through this path either.
func TestIsFavouriteRespectsCaller(t *testing.T) {
	ctx := context.Background()
	svc, uid := setup(t)
	other := createUser(t, "zweite@example.com")
	created, err := svc.Create(ctx, uid, loadFixtures(t)[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.SetFavourite(ctx, uid, created.ID, true); err != nil {
		t.Fatal(err)
	}

	fav, err := svc.IsFavourite(ctx, uid, created.ID)
	if err != nil || !fav {
		t.Fatalf("owner: fav=%v err=%v", fav, err)
	}
	fav, err = svc.IsFavourite(ctx, other, created.ID)
	if err != nil || fav {
		t.Fatalf("other user: fav=%v err=%v", fav, err)
	}
	fav, err = svc.IsFavourite(ctx, "", created.ID)
	if err != nil || fav {
		t.Fatalf("empty user id: fav=%v err=%v", fav, err)
	}
}

// TestSetFavouriteOnMissingRecipeReturnsErrNotFound pins SetFavourite's
// existence check: starring (on=true) a recipe id that doesn't exist must
// report ErrNotFound rather than surfacing the underlying foreign key
// violation INSERT OR IGNORE does not swallow. Unstarring (on=false) the
// same missing id must not error at all - removing a favourite for a
// recipe that's already gone is a no-op, not a failure.
func TestSetFavouriteOnMissingRecipeReturnsErrNotFound(t *testing.T) {
	ctx := context.Background()
	svc, uid := setup(t)

	if err := svc.SetFavourite(ctx, uid, "missing", true); !errors.Is(err, recipe.ErrNotFound) {
		t.Fatalf("star missing recipe: %v", err)
	}
	if err := svc.SetFavourite(ctx, uid, "missing", false); err != nil {
		t.Fatalf("unstar missing recipe must be a no-op: %v", err)
	}
}
