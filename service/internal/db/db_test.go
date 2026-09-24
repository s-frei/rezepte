package db_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/s-frei/rezepte/service/internal/db"
	"github.com/s-frei/rezepte/service/internal/db/dbtest"
	"github.com/s-frei/rezepte/service/internal/db/sqlc"
	"github.com/s-frei/rezepte/service/internal/user"
)

func TestOpenAndMigrate(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer conn.Close()

	if err := db.Migrate(ctx, conn); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	// Running again must be a no-op.
	if err := db.Migrate(ctx, conn); err != nil {
		t.Fatalf("second Migrate: %v", err)
	}

	var name string
	err = conn.QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name='users'`).Scan(&name)
	if err != nil || name != "users" {
		t.Fatalf("users table missing: %v", err)
	}

	var fk int
	if err := conn.QueryRowContext(ctx, `PRAGMA foreign_keys`).Scan(&fk); err != nil || fk != 1 {
		t.Fatalf("foreign_keys = %d (%v), want 1", fk, err)
	}
	var mode string
	if err := conn.QueryRowContext(ctx, `PRAGMA journal_mode`).Scan(&mode); err != nil || mode != "wal" {
		t.Fatalf("journal_mode = %q (%v), want wal", mode, err)
	}
	// Without this one a REPLACE walks past the superadmin triggers; see
	// TestSuperadminTriggers.
	var recursive int
	if err := conn.QueryRowContext(ctx, `PRAGMA recursive_triggers`).Scan(&recursive); err != nil || recursive != 1 {
		t.Fatalf("recursive_triggers = %d (%v), want 1", recursive, err)
	}
}

func TestTimeRoundTrip(t *testing.T) {
	now := time.Date(2026, 9, 17, 10, 30, 0, 0, time.UTC)
	s := db.FormatTime(now)
	if s != "2026-09-17T10:30:00Z" {
		t.Fatalf("FormatTime = %q", s)
	}
	back, err := db.ParseTime(s)
	if err != nil || !back.Equal(now) {
		t.Fatalf("ParseTime = %v, %v", back, err)
	}
}

func TestSuperadminTriggers(t *testing.T) {
	ctx := context.Background()
	conn := dbtest.Open(t)
	insert := `INSERT INTO users (id, username, display_name, password_hash, role, color, locale, created_at, updated_at)
	           VALUES (?, ?, ?, 'x', ?, 'amber', 'en', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`
	if _, err := conn.ExecContext(ctx, insert, "owner", "owner", "owner", "superadmin"); err != nil {
		t.Fatalf("seed superadmin: %v", err)
	}

	if _, err := conn.ExecContext(ctx, `DELETE FROM users WHERE id = 'owner'`); err == nil {
		t.Fatal("DELETE of the superadmin succeeded, want the trigger to abort it")
	}
	if _, err := conn.ExecContext(ctx, `UPDATE users SET role = 'admin' WHERE id = 'owner'`); err == nil {
		t.Fatal("demotion of the superadmin succeeded, want the trigger to abort it")
	}

	// A REPLACE is a delete plus an insert, so it is not an UPDATE the second
	// trigger sees, and its delete only fires the first one because the DSN
	// turns recursive_triggers on. With that pragma off this statement demotes
	// the owner and overwrites their hash without an error.
	replace := `INSERT OR REPLACE INTO users (id, username, display_name, password_hash, role, color, locale, created_at, updated_at)
	            VALUES ('owner', 'owner', 'owner', 'PWNED', 'user', 'amber', 'en', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`
	if _, err := conn.ExecContext(ctx, replace); err == nil {
		t.Fatal("INSERT OR REPLACE over the superadmin succeeded, want the trigger to abort it")
	}
	var role, hash string
	if err := conn.QueryRowContext(ctx, `SELECT role, password_hash FROM users WHERE id = 'owner'`).Scan(&role, &hash); err != nil {
		t.Fatalf("read the owner row back: %v", err)
	}
	if role != "superadmin" || hash != "x" {
		t.Fatalf("owner row = (%q, %q), want it untouched", role, hash)
	}

	// A second superadmin is refused by the partial unique index, whether it
	// arrives as a fresh row or as a promotion.
	if _, err := conn.ExecContext(ctx, insert, "other", "other", "other", "superadmin"); err == nil {
		t.Fatal("a second superadmin was inserted, want the unique index to refuse it")
	}
	if _, err := conn.ExecContext(ctx, insert, "jo", "jo", "jo", "admin"); err != nil {
		t.Fatalf("insert admin: %v", err)
	}
	if _, err := conn.ExecContext(ctx, `UPDATE users SET role = 'superadmin' WHERE id = 'jo'`); err == nil {
		t.Fatal("an admin was promoted to superadmin, want the unique index to refuse it")
	}
	// An unknown role is refused by the CHECK constraint.
	if _, err := conn.ExecContext(ctx, insert, "ghost", "ghost", "ghost", "wizard"); err == nil {
		t.Fatal("role 'wizard' was accepted, want the CHECK to refuse it")
	}
	// Ordinary rows are untouched by any of it.
	if _, err := conn.ExecContext(ctx, insert, "kim", "kim", "kim", "user"); err != nil {
		t.Fatalf("insert member: %v", err)
	}
	if _, err := conn.ExecContext(ctx, `DELETE FROM users WHERE id = 'kim'`); err != nil {
		t.Fatalf("delete member: %v", err)
	}
}

// seedIDs holds the ids of the rows seedMinimalRecipe inserts.
type seedIDs struct {
	UserID       string
	RecipeID     string
	GroupID      string
	IngredientID string
	StepID       string
}

// seedMinimalRecipe inserts one user, one recipe, one ingredient group, one
// ingredient and one step directly, bypassing the recipe service so this
// package's tests can exercise the schema without importing it.
func seedMinimalRecipe(t *testing.T, conn *sql.DB) seedIDs {
	t.Helper()
	ctx := context.Background()
	q := sqlc.New(conn)

	u, err := user.NewService(conn).Create(ctx, user.CreateParams{Username: "sam", Password: "pw", Role: user.RoleAdmin})
	if err != nil {
		t.Fatalf("seedMinimalRecipe: create user: %v", err)
	}

	now := db.FormatTime(time.Now().UTC())
	recipeID := uuid.Must(uuid.NewV7()).String()
	if _, err := q.InsertRecipe(ctx, sqlc.InsertRecipeParams{
		ID:          recipeID,
		Slug:        "test-recipe-" + recipeID,
		Title:       "Test Recipe",
		Description: "",
		Servings:    4,
		CreatedBy:   u.ID,
		CreatedAt:   now,
		UpdatedBy:   u.ID,
		UpdatedAt:   now,
	}); err != nil {
		t.Fatalf("seedMinimalRecipe: insert recipe: %v", err)
	}

	groupID := uuid.Must(uuid.NewV7()).String()
	if err := q.InsertIngredientGroup(ctx, sqlc.InsertIngredientGroupParams{
		ID:       groupID,
		RecipeID: recipeID,
		Position: 0,
	}); err != nil {
		t.Fatalf("seedMinimalRecipe: insert ingredient group: %v", err)
	}

	ingredientID := uuid.Must(uuid.NewV7()).String()
	if err := q.InsertIngredient(ctx, sqlc.InsertIngredientParams{
		ID:       ingredientID,
		GroupID:  groupID,
		Name:     "Zwiebel",
		Position: 0,
	}); err != nil {
		t.Fatalf("seedMinimalRecipe: insert ingredient: %v", err)
	}

	stepID := uuid.Must(uuid.NewV7()).String()
	if err := q.InsertStep(ctx, sqlc.InsertStepParams{
		ID:       stepID,
		RecipeID: recipeID,
		Position: 0,
		Text:     "Die Zwiebel schneiden.",
	}); err != nil {
		t.Fatalf("seedMinimalRecipe: insert step: %v", err)
	}

	return seedIDs{
		UserID:       u.ID,
		RecipeID:     recipeID,
		GroupID:      groupID,
		IngredientID: ingredientID,
		StepID:       stepID,
	}
}

func TestStepReferencesCascade(t *testing.T) {
	conn := dbtest.Open(t)
	ctx := context.Background()
	q := sqlc.New(conn)

	ids := seedMinimalRecipe(t, conn)

	if err := q.InsertStepReference(ctx, sqlc.InsertStepReferenceParams{
		StepID:       ids.StepID,
		IngredientID: ids.IngredientID,
		Word:         "Zwiebel",
		Position:     0,
	}); err != nil {
		t.Fatalf("insert reference: %v", err)
	}

	if err := q.DeleteStepsByRecipe(ctx, ids.RecipeID); err != nil {
		t.Fatalf("delete steps: %v", err)
	}
	rows, err := q.ListStepReferencesByRecipe(ctx, ids.RecipeID)
	if err != nil {
		t.Fatalf("list references: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("references survived the step delete: %d", len(rows))
	}
}

// TestStepReferencesCascadeOnIngredientDelete is TestStepReferencesCascade's
// sibling for the other foreign key on step_references: ingredient_id. The
// service never exercises this path on its own - Update always deletes
// every step (and with it, via the step_id cascade, every reference) before
// it deletes ingredient groups, so there is no way to reach it through the
// recipe service API. This goes one layer down and deletes the ingredient
// groups directly, leaving the step row untouched.
//
// The row count is read with a raw query against step_references rather
// than through ListStepReferencesByRecipe: that query INNER JOINs through
// ingredients, so once the ingredient is gone it reports zero rows whether
// or not the step_references row itself was actually deleted - it would
// pass just as well against a build with no ingredient-side cascade. Only a
// direct count, plus the survival of the step row (asserted via
// ListStepsByRecipe, ruling out that this is just the step_id cascade
// already covered above), pins this as the ingredient-side cascade
// specifically.
func TestStepReferencesCascadeOnIngredientDelete(t *testing.T) {
	conn := dbtest.Open(t)
	ctx := context.Background()
	q := sqlc.New(conn)

	ids := seedMinimalRecipe(t, conn)

	if err := q.InsertStepReference(ctx, sqlc.InsertStepReferenceParams{
		StepID:       ids.StepID,
		IngredientID: ids.IngredientID,
		Word:         "Zwiebel",
		Position:     0,
	}); err != nil {
		t.Fatalf("insert reference: %v", err)
	}

	if err := q.DeleteIngredientGroupsByRecipe(ctx, ids.RecipeID); err != nil {
		t.Fatalf("delete ingredient groups: %v", err)
	}

	var n int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM step_references WHERE step_id = ?`, ids.StepID).Scan(&n); err != nil {
		t.Fatalf("count step_references: %v", err)
	}
	if n != 0 {
		t.Fatalf("reference rows survived the ingredient delete: %d", n)
	}

	steps, err := q.ListStepsByRecipe(ctx, ids.RecipeID)
	if err != nil {
		t.Fatalf("list steps: %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("step row = %d, want 1 (it must survive - only the reference should be gone)", len(steps))
	}
}
