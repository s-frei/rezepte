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
	if _, err := user.NewService(conn).Create(ctx, user.CreateParams{Username: "demo", Password: "demo1234", Role: user.RoleAdmin}); err != nil {
		t.Fatal(err)
	}
	imageDir := filepath.Join(t.TempDir(), "images")

	sum, err := demo.Seed(ctx, conn, imageDir, "demo", locale, quiet)
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}
	if sum.Skipped || sum.Recipes != 12 || sum.Images != 27 {
		t.Fatalf("summary = %+v, want 12 recipes, 27 images, not skipped", sum)
	}

	recipes := recipe.NewService(conn)
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
	if _, err := user.NewService(conn).Create(ctx, user.CreateParams{Username: "demo", Password: "demo1234", Role: user.RoleAdmin}); err != nil {
		t.Fatal(err)
	}
	imageDir := filepath.Join(t.TempDir(), "images")
	if _, err := demo.Seed(ctx, conn, imageDir, "demo", "de", quiet); err != nil {
		t.Fatal(err)
	}
	sum, err := demo.Seed(ctx, conn, imageDir, "demo", "de", quiet) // "de": Count below expects the German fixture count
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
	if _, err := demo.Seed(ctx, conn, imageDir, "demo", "de", quiet); !errors.Is(err, demo.ErrNoUsers) {
		t.Fatalf("Seed without users: err = %v, want ErrNoUsers", err)
	}
	sam, err := user.NewService(conn).Create(ctx, user.CreateParams{Username: "sam", Password: "sam-password", Role: user.RoleAdmin})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := demo.Seed(ctx, conn, imageDir, "demo", "de", quiet); err != nil { // "de": the slug below is the German fixture's
		t.Fatalf("Seed with fallback owner: %v", err)
	}
	r, err := recipe.NewService(conn).BySlug(ctx, "flammkuchen")
	if err != nil {
		t.Fatal(err)
	}
	if r.CreatedBy.ID != sam.ID {
		t.Fatalf("owner = %s, want %s (first user)", r.CreatedBy.ID, sam.ID)
	}
}
