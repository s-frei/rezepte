package demo

import (
	"context"
	"io/fs"
	"log/slog"
	"path"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/s-frei/rezepte/service/internal/db/dbtest"

	"github.com/s-frei/rezepte/service/internal/recipe"
	"github.com/s-frei/rezepte/service/internal/user"
)

// A photo set is found by the sample's slug, so a renamed sample would lose
// its photos without a word. Every embedded set must belong to a sample of
// its locale and be numbered 1..n without gaps, or photosFor would stop
// early and drop the rest.
func TestEveryPhotoSetBelongsToASample(t *testing.T) {
	locales, err := fs.ReadDir(photos, "photos")
	if err != nil {
		t.Fatal(err)
	}
	if len(locales) == 0 {
		t.Fatal("no embedded photos")
	}
	for _, loc := range locales {
		samples, err := recipe.Samples(user.Locale(loc.Name()))
		if err != nil {
			t.Fatal(err)
		}
		slugs := map[string]bool{}
		for _, s := range samples {
			slugs[recipe.Slugify(s.Title)] = true
		}
		sets, err := fs.ReadDir(photos, path.Join("photos", loc.Name()))
		if err != nil {
			t.Fatal(err)
		}
		for _, set := range sets {
			if !slugs[set.Name()] {
				t.Errorf("photos/%s/%s matches no %s sample", loc.Name(), set.Name(), loc.Name())
				continue
			}
			files, err := fs.ReadDir(photos, path.Join("photos", loc.Name(), set.Name()))
			if err != nil {
				t.Fatal(err)
			}
			got, err := photosFor(set.Name())
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != len(files) {
				t.Errorf("photos/%s/%s: %d files, %d read in order; name them 1.jpg..n.jpg", loc.Name(), set.Name(), len(files), len(got))
			}
		}
	}
}

// A set whose samples have no photos at all falls back to placeholders for
// the first imagedRecipes samples, as a language that brings its own sample
// recipes but no photos does.
func TestSeedFallsBackToPlaceholders(t *testing.T) {
	saved := photos
	photos = fstest.MapFS{}
	t.Cleanup(func() { photos = saved })

	ctx := context.Background()
	conn := dbtest.Open(t)
	if _, err := user.NewService(conn).Create(ctx, user.CreateParams{Username: "demo", Password: "demo1234", Role: user.RoleAdmin}); err != nil {
		t.Fatal(err)
	}
	sum, err := Seed(ctx, conn, filepath.Join(t.TempDir(), "images"), "demo", "de", slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}
	if sum.Recipes != 12 || sum.Images != imagedRecipes {
		t.Fatalf("summary = %+v, want 12 recipes and %d placeholders", sum, imagedRecipes)
	}
}
