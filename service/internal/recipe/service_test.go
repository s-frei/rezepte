package recipe_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/s-frei/rezepte/service/internal/db/dbtest"
	"github.com/s-frei/rezepte/service/internal/recipe"
	"github.com/s-frei/rezepte/service/internal/user"
)

func loadFixtures(t *testing.T) []recipe.Input {
	t.Helper()
	data, err := os.ReadFile("testdata/recipes.json")
	if err != nil {
		t.Fatal(err)
	}
	var in []recipe.Input
	if err := json.Unmarshal(data, &in); err != nil {
		t.Fatalf("parse fixtures: %v", err)
	}
	if len(in) != 12 {
		t.Fatalf("fixtures = %d, want 12", len(in))
	}
	return in
}

func setup(t *testing.T) (*recipe.Service, string) {
	t.Helper()
	conn := dbtest.Open(t)
	u, err := user.NewService(conn).Create(context.Background(), "sam", "pw", user.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	return recipe.NewService(conn), u.ID
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
