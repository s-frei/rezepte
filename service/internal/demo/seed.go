// Package demo fills an empty Rezepte instance with the sample recipes and
// the deterministic placeholder photos that stand in for real pictures, so
// documentation screenshots and manual testing have realistic data. It
// writes only through the domain services and only when the recipes table
// is empty.
package demo

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

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
// a placeholder image. The rest show the overview's placeholder tiles.
const imagedRecipes = 9

// ErrNoUsers is returned when no user exists to own the sample recipes.
var ErrNoUsers = errors.New("demo: no user to own the sample recipes")

// Summary reports what Seed did.
type Summary struct {
	Recipes int
	Images  int
	// Skipped is true when the recipes table was not empty; nothing was written.
	Skipped bool
}

// Seed creates the sample recipes, owned by the user named owner (or the
// first user by name when no such user exists), and uploads a placeholder
// image for the first imagedRecipes of them below imageDir. It is
// idempotent: a database that already holds recipes is left untouched.
func Seed(ctx context.Context, conn *sql.DB, imageDir, owner string, logger *slog.Logger) (Summary, error) {
	recipes := recipe.NewService(conn, recipe.WithImageDir(imageDir))
	n, err := recipes.Count(ctx)
	if err != nil {
		return Summary{}, err
	}
	if n > 0 {
		logger.Info("demo: recipes present, not seeding", "count", n)
		return Summary{Skipped: true}, nil
	}
	ownerID, err := ownerID(ctx, user.NewService(conn), owner)
	if err != nil {
		return Summary{}, err
	}
	samples, err := recipe.Samples()
	if err != nil {
		return Summary{}, err
	}
	images := image.NewService(conn, imageDir)
	var sum Summary
	// The overview sorts by updated_at desc: seeding back to front puts the
	// first sample on top.
	for i := len(samples) - 1; i >= 0; i-- {
		r, err := recipes.Create(ctx, ownerID, samples[i])
		if err != nil {
			return sum, fmt.Errorf("create sample %q: %w", samples[i].Title, err)
		}
		sum.Recipes++
		if i >= imagedRecipes {
			continue
		}
		data, err := Placeholder(i, r.Title)
		if err != nil {
			return sum, err
		}
		if _, err := images.Upload(ctx, r.ID, ownerID, bytes.NewReader(data)); err != nil {
			return sum, fmt.Errorf("upload placeholder for %q: %w", r.Title, err)
		}
		sum.Images++
	}
	logger.Info("demo: sample data seeded", "recipes", sum.Recipes, "images", sum.Images)
	return sum, nil
}

// ownerID resolves the user named username, falling back to the first user
// (List orders by username) so seeding works whatever the instance owner is
// called.
func ownerID(ctx context.Context, users *user.Service, username string) (string, error) {
	list, err := users.List(ctx)
	if err != nil {
		return "", err
	}
	if len(list) == 0 {
		return "", ErrNoUsers
	}
	for _, u := range list {
		if u.Username == username {
			return u.ID, nil
		}
	}
	return list[0].ID, nil
}
