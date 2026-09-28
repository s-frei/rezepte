package share

import (
	"context"
	"errors"
	"testing"
	"time"
	"uuid"

	"github.com/s-frei/rezepte/service/internal/db"
	"github.com/s-frei/rezepte/service/internal/db/dbtest"
	"github.com/s-frei/rezepte/service/internal/db/sqlc"
	"github.com/s-frei/rezepte/service/internal/recipe"
	"github.com/s-frei/rezepte/service/internal/user"
)

// TestIsUniqueViolationOnARealDuplicateInsert inserts two shares for the
// same recipe and creator directly through the sqlc queries, against a real
// migrated database, so the second insert fails on the UNIQUE(recipe_id,
// created_by) constraint SQLite enforces - unlike
// TestCreateFindsADirectlyInsertedRowAsExisting in service_test.go, where
// the row is already there before Create is ever called, so liveOwnShare's
// own SELECT finds it first and Create never reaches an INSERT at all. This
// is the one test that actually exercises the branch Create's comment
// describes as a race: the insert itself failing on the constraint.
func TestIsUniqueViolationOnARealDuplicateInsert(t *testing.T) {
	ctx := context.Background()
	conn := dbtest.Open(t)

	users := user.NewService(conn, "")
	member, err := users.Create(ctx, user.CreateParams{Username: "mira", Password: "pw", Role: user.RoleUser})
	if err != nil {
		t.Fatal(err)
	}
	recipes := recipe.NewService(conn, "")
	r, err := recipes.Create(ctx, member.ID, recipe.Input{
		Title: "Gurkensalat", Servings: 2,
		IngredientGroups: []recipe.IngredientGroup{{Ingredients: []recipe.Ingredient{{Name: "Gurke"}}}},
		Steps:            []recipe.Step{{Text: "Schneiden."}},
	})
	if err != nil {
		t.Fatal(err)
	}

	q := sqlc.New(conn)
	now := db.FormatTime(time.Now().UTC())
	if _, err := q.CreateShare(ctx, sqlc.CreateShareParams{
		ID: uuid.NewV7().String(), Token: "token-one-aaaaaaaaaaaa",
		RecipeID: r.ID, CreatedBy: member.ID, CreatedAt: now,
	}); err != nil {
		t.Fatalf("first insert: %v", err)
	}

	_, err = q.CreateShare(ctx, sqlc.CreateShareParams{
		// Same recipe_id and created_by as above, everything else distinct -
		// the composite UNIQUE(recipe_id, created_by) is what must fail,
		// not the primary key or token's own UNIQUE column.
		ID: uuid.NewV7().String(), Token: "token-two-aaaaaaaaaaaa",
		RecipeID: r.ID, CreatedBy: member.ID, CreatedAt: now,
	})
	if err == nil {
		t.Fatal("second insert for the same recipe and creator: want an error")
	}
	if !isUniqueViolation(err) {
		t.Errorf("isUniqueViolation(%v) = false, want true", err)
	}

	if isUniqueViolation(errors.New("boom")) {
		t.Error("isUniqueViolation(a plain error) = true, want false")
	}
	if isUniqueViolation(nil) {
		t.Error("isUniqueViolation(nil) = true, want false")
	}
}
