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
	// "de": every test built on this fixture (here, search_test.go and
	// handler_test.go) keys off the German sample titles and search terms.
	in, err := recipe.Samples("de")
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
	u, err := user.NewService(conn).Create(context.Background(), user.CreateParams{Username: "sam", Password: "pw", Role: user.RoleAdmin})
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
	u, err := user.NewService(conn).Create(context.Background(), user.CreateParams{Username: username, Password: "pw", Role: user.RoleAdmin})
	if err != nil {
		t.Fatal(err)
	}
	return u.ID
}

// createUserWithProfile is createUser plus a display name and colour,
// for tests that need to assert on what a card or facet shows for the
// author rather than just who they are.
func createUserWithProfile(t *testing.T, username, displayName string, color user.Color) string {
	t.Helper()
	conn, ok := testConns[t]
	if !ok {
		t.Fatal("createUserWithProfile: call setup(t) first")
	}
	u, err := user.NewService(conn).Create(context.Background(), user.CreateParams{
		Username: username, Password: "pw", Role: user.RoleAdmin,
		DisplayName: displayName, Color: color,
	})
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
	in.Steps = []recipe.Step{{Text: "Alles mischen."}}
	updated, err := svc.Update(ctx, created.ID, uid, in)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Slug != created.Slug || updated.Title != "Käsespätzle deluxe" {
		t.Fatalf("slug/title = %q/%q", updated.Slug, updated.Title)
	}
	if len(updated.IngredientGroups) != 1 || len(updated.Steps) != 1 || len(updated.Tags) != 2 || updated.Tags[1] != "vegetarisch" {
		t.Fatalf("children = %+v", updated)
	}
	if _, err := svc.Update(ctx, "missing", uid, in); !errors.Is(err, recipe.ErrNotFound) {
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
	u, err := user.NewService(conn).Create(ctx, user.CreateParams{Username: "sam", Password: "pw", Role: user.RoleAdmin})
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
	if err := q.SetRecipeCover(ctx, sqlc.SetRecipeCoverParams{CoverImageID: &cover, UpdatedBy: u.ID, UpdatedAt: db.FormatTime(time.Now()), ID: created.ID}); err != nil {
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
	u, err := user.NewService(conn).Create(ctx, user.CreateParams{Username: "sam", Password: "pw", Role: user.RoleAdmin})
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

func TestUpdateRecordsTheEditor(t *testing.T) {
	ctx := context.Background()
	svc, uid := setup(t)
	editor := createUser(t, "mara")
	in := loadFixtures(t)[3] // Käsespätzle
	created, err := svc.Create(ctx, uid, in)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	// A recipe nobody has edited yet names its author on both sides.
	if created.CreatedBy != uid || created.UpdatedBy != uid {
		t.Fatalf("after create: createdBy = %q, updatedBy = %q, want both %q", created.CreatedBy, created.UpdatedBy, uid)
	}

	in.Title = "Käsespätzle deluxe"
	updated, err := svc.Update(ctx, created.ID, editor, in)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.UpdatedBy != editor {
		t.Fatalf("updatedBy = %q, want the editor %q", updated.UpdatedBy, editor)
	}
	if updated.CreatedBy != uid {
		t.Fatalf("createdBy = %q, want the original author %q", updated.CreatedBy, uid)
	}
}

// The detail view shows names, and /api/v1/users is admin-only, so the
// recipe has to carry them itself.
func TestRecipeCarriesAuthorNames(t *testing.T) {
	ctx := context.Background()
	svc, uid := setup(t)
	editor := createUser(t, "mara")
	created, err := svc.Create(ctx, uid, loadFixtures(t)[3])
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.CreatedByName != "sam" || created.UpdatedByName != "sam" {
		t.Fatalf("after create: %q / %q, want both \"sam\"", created.CreatedByName, created.UpdatedByName)
	}

	updated, err := svc.Update(ctx, created.ID, editor, loadFixtures(t)[3])
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.CreatedByName != "sam" || updated.UpdatedByName != "mara" {
		t.Fatalf("after update: %q / %q, want \"sam\" / \"mara\"", updated.CreatedByName, updated.UpdatedByName)
	}

	bySlug, err := svc.BySlug(ctx, created.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if bySlug.UpdatedByName != "mara" {
		t.Fatalf("BySlug: updatedByName = %q, want \"mara\"", bySlug.UpdatedByName)
	}
}

func TestCardsCarryTheAuthorsDisplayNameAndColour(t *testing.T) {
	ctx := context.Background()
	svc, _ := setup(t)
	authorID := createUserWithProfile(t, "mia", "Sam der Koch", "teal")

	created, err := svc.Create(ctx, authorID, loadFixtures(t)[0])
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	page, err := svc.List(ctx, recipe.ListParams{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].ID != created.ID {
		t.Fatalf("page.Items = %+v, want just %q", page.Items, created.ID)
	}
	card := page.Items[0]
	if card.CreatedByName != "Sam der Koch" {
		t.Errorf("CreatedByName = %q; want the display name", card.CreatedByName)
	}
	if card.CreatedByColor != "teal" || card.UpdatedByColor != "teal" {
		t.Errorf("colours = %q / %q; want teal / teal", card.CreatedByColor, card.UpdatedByColor)
	}

	detail, err := svc.ByID(ctx, card.ID)
	if err != nil {
		t.Fatalf("by id: %v", err)
	}
	if detail.CreatedByName != "Sam der Koch" || detail.CreatedByColor != "teal" {
		t.Errorf("detail = %q / %q; want the display name and teal", detail.CreatedByName, detail.CreatedByColor)
	}

	authors, err := svc.Authors(ctx)
	if err != nil {
		t.Fatalf("authors: %v", err)
	}
	if len(authors) != 1 {
		t.Fatalf("authors = %+v, want exactly one", authors)
	}
	if authors[0].Username != "mia" {
		t.Errorf("Username = %q; want the login name, which the filter matches on", authors[0].Username)
	}
	if authors[0].DisplayName != "Sam der Koch" || authors[0].Color != "teal" {
		t.Errorf("facet = %q / %q; want the display name and teal", authors[0].DisplayName, authors[0].Color)
	}
}

func f64(v float64) *float64 { return &v }

func TestCreateAndLoadRoundTripsReferences(t *testing.T) {
	svc, userID := setup(t)
	ctx := context.Background()

	teig, fuellung := "Teig", "Füllung"
	in := recipe.Input{
		Title:    "Maultaschen",
		Servings: 4,
		IngredientGroups: []recipe.IngredientGroup{
			{Name: &teig, Ingredients: []recipe.Ingredient{{Name: "Ei", Quantity: f64(3)}}},
			{Name: &fuellung, Ingredients: []recipe.Ingredient{{Name: "Ei", Quantity: f64(1)}}},
		},
		Steps: []recipe.Step{
			{Text: "Mehl und Ei verkneten.", References: []recipe.IngredientRef{
				{Word: "Ei", GroupName: &teig, IngredientName: "Ei"},
			}},
			{Text: "Spinat mit Ei vermengen.", References: []recipe.IngredientRef{
				{Word: "Ei", GroupName: &fuellung, IngredientName: "Ei"},
			}},
		},
	}

	created, err := svc.Create(ctx, userID, in)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := svc.ByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(got.Steps) != 2 {
		t.Fatalf("steps = %d, want 2", len(got.Steps))
	}
	if n := len(got.Steps[0].References); n != 1 {
		t.Fatalf("step 0 references = %d, want 1", n)
	}
	if g := got.Steps[0].References[0].GroupName; g == nil || *g != "Teig" {
		t.Errorf("step 0 group = %v, want Teig", g)
	}
	if g := got.Steps[1].References[0].GroupName; g == nil || *g != "Füllung" {
		t.Errorf("step 1 group = %v, want Füllung", g)
	}
}

// TestCreateAndLoadRoundTripsReferencesWithinOneGroup pins the ingredient
// index within a single group. TestCreateAndLoadRoundTripsReferences above
// only discriminates by group (both ingredients there are named "Ei"), so
// it would catch a group/ingredient axis swap but not an off-by-one within
// one group's ingredient list. Here both ingredients live in the same
// (unnamed) group and are named differently, so a wrong index resolves to
// the wrong name.
func TestCreateAndLoadRoundTripsReferencesWithinOneGroup(t *testing.T) {
	svc, userID := setup(t)
	ctx := context.Background()

	in := recipe.Input{
		Title:    "Gurkensalat",
		Servings: 2,
		IngredientGroups: []recipe.IngredientGroup{
			{Ingredients: []recipe.Ingredient{
				{Name: "Gurke", Quantity: f64(1)},
				{Name: "Essig", Quantity: f64(2)},
			}},
		},
		Steps: []recipe.Step{
			{Text: "Gurke hobeln und mit Essig anmachen.", References: []recipe.IngredientRef{
				{Word: "Gurke", IngredientName: "Gurke"},
				{Word: "Essig", IngredientName: "Essig"},
			}},
		},
	}

	created, err := svc.Create(ctx, userID, in)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := svc.ByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if n := len(got.Steps[0].References); n != 2 {
		t.Fatalf("references = %d, want 2", n)
	}
	byWord := make(map[string]string, 2)
	for _, r := range got.Steps[0].References {
		byWord[r.Word] = r.IngredientName
	}
	if byWord["Gurke"] != "Gurke" {
		t.Errorf(`reference for word "Gurke" resolved to ingredient %q, want "Gurke"`, byWord["Gurke"])
	}
	if byWord["Essig"] != "Essig" {
		t.Errorf(`reference for word "Essig" resolved to ingredient %q, want "Essig"`, byWord["Essig"])
	}
}

func TestReferencesSerialiseAsEmptySliceNotNil(t *testing.T) {
	svc, userID := setup(t)
	ctx := context.Background()
	created, err := svc.Create(ctx, userID, recipe.Input{
		Title: "Ohne", Servings: 2,
		IngredientGroups: []recipe.IngredientGroup{{Ingredients: []recipe.Ingredient{{Name: "Salz"}}}},
		Steps:            []recipe.Step{{Text: "Salzen."}},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := svc.ByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Steps[0].References == nil {
		t.Fatal("References is nil; it must marshal as [] so GET->PUT round-trips")
	}
}

func TestUpdateRewritesReferences(t *testing.T) {
	svc, userID := setup(t)
	ctx := context.Background()
	created, err := svc.Create(ctx, userID, recipe.Input{
		Title: "Suppe", Servings: 2,
		IngredientGroups: []recipe.IngredientGroup{{Ingredients: []recipe.Ingredient{{Name: "Möhre", Quantity: f64(2)}}}},
		Steps: []recipe.Step{{Text: "Möhre würfeln.", References: []recipe.IngredientRef{
			{Word: "Möhre", IngredientName: "Möhre"},
		}}},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// Drop the reference; the row must not survive.
	in := created.Input
	in.Steps[0].References = nil
	if _, err := svc.Update(ctx, created.ID, userID, in); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, err := svc.ByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if n := len(got.Steps[0].References); n != 0 {
		t.Fatalf("references = %d after removal, want 0", n)
	}
}

// TestUpdateDroppingAReferencedIngredientStillLoads covers a document
// rewrite that drops a referenced ingredient: the recipe must still save
// and reload cleanly, keeping only the reference to the ingredient that
// survives. It does not exercise the ingredient_id cascade on
// step_references - Update always calls DeleteStepsByRecipe first, which
// already removes every reference row for this recipe via the step_id
// cascade, so this test would pass unchanged against a build with no
// ingredient-side cascade at all. That cascade is covered one layer down,
// in internal/db/db_test.go (TestStepReferencesCascadeOnIngredientDelete),
// where an ingredient can be deleted directly while its step row survives.
func TestUpdateDroppingAReferencedIngredientStillLoads(t *testing.T) {
	svc, userID := setup(t)
	ctx := context.Background()
	created, err := svc.Create(ctx, userID, recipe.Input{
		Title: "Salat", Servings: 2,
		IngredientGroups: []recipe.IngredientGroup{{Ingredients: []recipe.Ingredient{
			{Name: "Gurke", Quantity: f64(1)},
			{Name: "Tomate", Quantity: f64(2)},
		}}},
		Steps: []recipe.Step{{Text: "Gurke und Tomate schneiden.", References: []recipe.IngredientRef{
			{Word: "Gurke", IngredientName: "Gurke"},
			{Word: "Tomate", IngredientName: "Tomate"},
		}}},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Drop the "Gurke" ingredient entirely; its step still names it in the
	// text, but no longer carries a reference to it.
	in := created.Input
	in.IngredientGroups[0].Ingredients = in.IngredientGroups[0].Ingredients[1:]
	in.Steps[0].References = []recipe.IngredientRef{
		{Word: "Tomate", IngredientName: "Tomate"},
	}
	if _, err := svc.Update(ctx, created.ID, userID, in); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := svc.ByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if len(got.IngredientGroups[0].Ingredients) != 1 || got.IngredientGroups[0].Ingredients[0].Name != "Tomate" {
		t.Fatalf("ingredients = %+v, want only Tomate", got.IngredientGroups[0].Ingredients)
	}
	if n := len(got.Steps[0].References); n != 1 || got.Steps[0].References[0].IngredientName != "Tomate" {
		t.Fatalf("references = %+v, want a single Tomate reference (Gurke's row must not survive)", got.Steps[0].References)
	}
}
