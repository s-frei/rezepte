package db_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/s-frei/rezepte/service/internal/db"
	"github.com/s-frei/rezepte/service/internal/db/dbtest"
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
	insert := `INSERT INTO users (id, username, password_hash, role, created_at, updated_at)
	           VALUES (?, ?, 'x', ?, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`
	if _, err := conn.ExecContext(ctx, insert, "owner", "owner", "superadmin"); err != nil {
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
	replace := `INSERT OR REPLACE INTO users (id, username, password_hash, role, created_at, updated_at)
	            VALUES ('owner', 'owner', 'PWNED', 'user', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`
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
	if _, err := conn.ExecContext(ctx, insert, "other", "other", "superadmin"); err == nil {
		t.Fatal("a second superadmin was inserted, want the unique index to refuse it")
	}
	if _, err := conn.ExecContext(ctx, insert, "jo", "jo", "admin"); err != nil {
		t.Fatalf("insert admin: %v", err)
	}
	if _, err := conn.ExecContext(ctx, `UPDATE users SET role = 'superadmin' WHERE id = 'jo'`); err == nil {
		t.Fatal("an admin was promoted to superadmin, want the unique index to refuse it")
	}
	// An unknown role is refused by the CHECK constraint.
	if _, err := conn.ExecContext(ctx, insert, "ghost", "ghost", "wizard"); err == nil {
		t.Fatal("role 'wizard' was accepted, want the CHECK to refuse it")
	}
	// Ordinary rows are untouched by any of it.
	if _, err := conn.ExecContext(ctx, insert, "kim", "kim", "user"); err != nil {
		t.Fatalf("insert member: %v", err)
	}
	if _, err := conn.ExecContext(ctx, `DELETE FROM users WHERE id = 'kim'`); err != nil {
		t.Fatalf("delete member: %v", err)
	}
}
