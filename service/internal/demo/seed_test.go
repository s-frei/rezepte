package demo_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/s-frei/rezepte/service/internal/db/dbtest"
	"github.com/s-frei/rezepte/service/internal/demo"
	"github.com/s-frei/rezepte/service/internal/recipe"
	"github.com/s-frei/rezepte/service/internal/user"
)

var quiet = slog.New(slog.DiscardHandler)

func TestSeedCreatesSamplesWithPlaceholders(t *testing.T) {
	ctx := context.Background()
	conn := dbtest.Open(t)
	if _, err := user.NewService(conn).Create(ctx, user.CreateParams{Username: "demo", Password: "demo1234", Role: user.RoleAdmin}); err != nil {
		t.Fatal(err)
	}
	imageDir := filepath.Join(t.TempDir(), "images")

	sum, err := demo.Seed(ctx, conn, imageDir, "demo", quiet)
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}
	if sum.Skipped || sum.Recipes != 12 || sum.Images != 9 {
		t.Fatalf("summary = %+v, want 12 recipes, 9 images, not skipped", sum)
	}

	recipes := recipe.NewService(conn)
	klopse, err := recipes.BySlug(ctx, "koenigsberger-klopse")
	if err != nil {
		t.Fatalf("klopse: %v", err)
	}
	if klopse.CoverImageID == nil || len(klopse.Images) != 1 || len(klopse.IngredientGroups) != 2 {
		t.Fatalf("klopse: cover %v, %d images, %d groups; want a cover, 1 image, 2 groups",
			klopse.CoverImageID, len(klopse.Images), len(klopse.IngredientGroups))
	}
	thumb := filepath.Join(imageDir, klopse.ID, klopse.Images[0].ID+"_thumb.jpg")
	if _, err := os.Stat(thumb); err != nil {
		t.Fatalf("thumb variant missing: %v", err)
	}
	salat, err := recipes.BySlug(ctx, "kartoffelsalat")
	if err != nil {
		t.Fatal(err)
	}
	if salat.CoverImageID != nil || len(salat.Images) != 0 {
		t.Fatalf("kartoffelsalat should have no image, got %+v", salat.Images)
	}

	page, err := recipes.List(ctx, recipe.ListParams{Page: 1, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 12 || len(page.Items) != 1 || page.Items[0].Title != "Königsberger Klopse" {
		t.Fatalf("overview: total %d, first %q; want 12 and Königsberger Klopse first", page.Total, page.Items[0].Title)
	}
}

func TestSeedIsIdempotent(t *testing.T) {
	ctx := context.Background()
	conn := dbtest.Open(t)
	if _, err := user.NewService(conn).Create(ctx, user.CreateParams{Username: "demo", Password: "demo1234", Role: user.RoleAdmin}); err != nil {
		t.Fatal(err)
	}
	imageDir := filepath.Join(t.TempDir(), "images")
	if _, err := demo.Seed(ctx, conn, imageDir, "demo", quiet); err != nil {
		t.Fatal(err)
	}
	sum, err := demo.Seed(ctx, conn, imageDir, "demo", quiet)
	if err != nil {
		t.Fatalf("second Seed: %v", err)
	}
	if !sum.Skipped || sum.Recipes != 0 || sum.Images != 0 {
		t.Fatalf("second summary = %+v, want skipped and zero counts", sum)
	}
	if n, _ := recipe.NewService(conn).Count(ctx); n != 12 {
		t.Fatalf("recipes after second seed = %d, want 12", n)
	}
}

func TestSeedNeedsAUserAndFallsBackToTheFirst(t *testing.T) {
	ctx := context.Background()
	conn := dbtest.Open(t)
	imageDir := filepath.Join(t.TempDir(), "images")
	if _, err := demo.Seed(ctx, conn, imageDir, "demo", quiet); !errors.Is(err, demo.ErrNoUsers) {
		t.Fatalf("Seed without users: err = %v, want ErrNoUsers", err)
	}
	sam, err := user.NewService(conn).Create(ctx, user.CreateParams{Username: "sam", Password: "sam-password", Role: user.RoleAdmin})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := demo.Seed(ctx, conn, imageDir, "demo", quiet); err != nil {
		t.Fatalf("Seed with fallback owner: %v", err)
	}
	r, err := recipe.NewService(conn).BySlug(ctx, "flammkuchen")
	if err != nil {
		t.Fatal(err)
	}
	if r.CreatedBy != sam.ID {
		t.Fatalf("owner = %s, want %s (first user)", r.CreatedBy, sam.ID)
	}
}
