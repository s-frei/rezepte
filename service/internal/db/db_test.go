package db_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/s-frei/rezepte/service/internal/db"
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
