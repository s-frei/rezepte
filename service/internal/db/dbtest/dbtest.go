// Package dbtest provides a migrated throwaway database for tests.
package dbtest

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/s-frei/rezepte/service/internal/db"
)

// Open returns a migrated SQLite database in a temp directory that is
// closed and removed when the test ends.
func Open(t testing.TB) *sql.DB {
	t.Helper()
	ctx := context.Background()
	conn, err := db.Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("dbtest: open: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.Migrate(ctx, conn); err != nil {
		t.Fatalf("dbtest: migrate: %v", err)
	}
	return conn
}
