// Package demo fills an empty Rezepte instance with the sample recipes and
// their photos, so documentation screenshots and manual testing have
// realistic data. A sample set with embedded photos gets those; a set
// without any gets deterministic placeholders instead. It writes only
// through the domain services and only when the recipes table is empty.
package demo

import (
	"bytes"
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"path"

	"github.com/s-frei/rezepte/service/internal/image"
	"github.com/s-frei/rezepte/service/internal/recipe"
	"github.com/s-frei/rezepte/service/internal/user"
)

// Credentials of the admin --demo creates when REZEPTE_ADMIN_PASSWORD is
// unset and the users table is empty.
const (
	AdminUser     = "demo"
	AdminPassword = "demo1234"
)

// imagedRecipes is how many samples, counted from the top of the list, get
// a placeholder image when their set has no photos. The rest show the
// overview's placeholder tiles.
const imagedRecipes = 9

// photos holds the demo photos as photos/<locale>/<slug>/<n>.jpg, numbered
// from 1 in gallery order. They are derived from the AI-generated originals
// in assets/demo-photos by `mise run demo-photos`; do not edit them here.
//
//go:embed photos
var embedded embed.FS

// photos is what photosFor reads; a variable so tests can seed a set that
// has no photos.
var photos fs.FS = embedded

// ErrNoUsers is returned when no user exists to own the sample recipes.
var ErrNoUsers = errors.New("demo: no user to own the sample recipes")

// Summary reports what Seed did.
type Summary struct {
	Recipes int
	Images  int
	// Skipped is true when the recipes table was not empty; nothing was written.
	Skipped bool
}

// Seed creates the sample recipes in locale's language, owned by the user
// named owner (or the first user by name when no such user exists), and
// uploads their images below imageDir. When any sample of the set has
// embedded photos, every sample gets its own photos, the first as cover, and
// a sample without photos stays without an image. A set with no photos at
// all gets a placeholder for each of its first imagedRecipes samples. It is
// idempotent: a database that already holds recipes is left untouched.
func Seed(ctx context.Context, conn *sql.DB, imageDir, owner string, locale user.Locale, logger *slog.Logger) (Summary, error) {
	recipes := recipe.NewService(conn, recipe.WithImageDir(imageDir))
	n, err := recipes.Count(ctx)
	if err != nil {
		return Summary{}, err
	}
	if n > 0 {
		logger.Info("demo: recipes present, not seeding", "count", n)
		return Summary{Skipped: true}, nil
	}
	o, err := findOwner(ctx, user.NewService(conn), owner)
	if err != nil {
		return Summary{}, err
	}
	samples, err := recipe.Samples(locale)
	if err != nil {
		return Summary{}, err
	}
	sets := make([][][]byte, len(samples))
	withPhotos := false
	for i, s := range samples {
		if sets[i], err = photosFor(recipe.Slugify(s.Title)); err != nil {
			return Summary{}, err
		}
		withPhotos = withPhotos || len(sets[i]) > 0
	}
	images := image.NewService(conn, imageDir)
	var sum Summary
	// The overview sorts by updated_at desc: seeding back to front puts the
	// first sample on top.
	for i := len(samples) - 1; i >= 0; i-- {
		r, err := recipes.Create(ctx, o.ID, samples[i])
		if err != nil {
			return sum, fmt.Errorf("create sample %q: %w", samples[i].Title, err)
		}
		sum.Recipes++
		if withPhotos {
			for n, data := range sets[i] {
				if _, err := images.Upload(ctx, r.ID, o, bytes.NewReader(data)); err != nil {
					return sum, fmt.Errorf("upload photo %d for %q: %w", n+1, r.Title, err)
				}
				sum.Images++
			}
			continue
		}
		if i >= imagedRecipes {
			continue
		}
		data, err := Placeholder(i, r.Title)
		if err != nil {
			return sum, err
		}
		if _, err := images.Upload(ctx, r.ID, o, bytes.NewReader(data)); err != nil {
			return sum, fmt.Errorf("upload placeholder for %q: %w", r.Title, err)
		}
		sum.Images++
	}
	logger.Info("demo: sample data seeded", "recipes", sum.Recipes, "images", sum.Images)
	return sum, nil
}

// photosFor returns the embedded photos of the sample whose slug is slug,
// 1.jpg first, stopping at the first missing number; nil when it has none.
// The slug alone identifies the sample: sample sets are different dishes,
// and a language falling back to another's set finds that set's photos.
func photosFor(slug string) ([][]byte, error) {
	dirs, err := fs.Glob(photos, path.Join("photos", "*", slug))
	if err != nil || len(dirs) == 0 {
		return nil, err
	}
	var set [][]byte
	for n := 1; ; n++ {
		data, err := fs.ReadFile(photos, path.Join(dirs[0], fmt.Sprintf("%d.jpg", n)))
		if errors.Is(err, fs.ErrNotExist) {
			return set, nil
		}
		if err != nil {
			return nil, fmt.Errorf("read photo %d of %s: %w", n, slug, err)
		}
		set = append(set, data)
	}
}

// findOwner resolves the user named username, falling back to the first
// user (List orders by username) so seeding works whatever the instance
// owner is called. The recipes it creates are that user's own, so the edit
// rule lets them add the placeholder images whatever their role.
func findOwner(ctx context.Context, users *user.Service, username string) (user.User, error) {
	list, err := users.List(ctx)
	if err != nil {
		return user.User{}, err
	}
	if len(list) == 0 {
		return user.User{}, ErrNoUsers
	}
	for _, u := range list {
		if u.Username == username {
			return u, nil
		}
	}
	return list[0], nil
}
