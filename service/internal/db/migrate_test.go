package db

import (
	"context"
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/pressly/goose/v3"
)

// TestRecipeCommentsMigrationDownAndUp rolls 0014 back and applies it again,
// so its Down section is known to undo exactly what its Up section creates.
func TestRecipeCommentsMigrationDownAndUp(t *testing.T) {
	ctx := context.Background()
	conn, err := Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	fsys, err := fs.Sub(migrations, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, conn, fsys)
	if err != nil {
		t.Fatal(err)
	}
	tables := func() int {
		t.Helper()
		var n int
		if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master
			WHERE type = 'table' AND name IN ('recipe_comments', 'recipe_comment_reads')`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("up: %v", err)
	}
	if _, err := provider.DownTo(ctx, 13); err != nil {
		t.Fatalf("down to 13: %v", err)
	}
	if n := tables(); n != 0 {
		t.Fatalf("%d comment tables left after down", n)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("up again: %v", err)
	}
	if n := tables(); n != 2 {
		t.Fatalf("%d comment tables after up, want 2", n)
	}
}

// TestPushMigrationDownAndUp rolls 0019 back and applies it again, so its
// Down section undoes the push tables and the VAPID key column.
func TestPushMigrationDownAndUp(t *testing.T) {
	ctx := context.Background()
	conn, err := Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	fsys, err := fs.Sub(migrations, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, conn, fsys)
	if err != nil {
		t.Fatal(err)
	}
	objects := func() int {
		t.Helper()
		var n int
		if err := conn.QueryRowContext(ctx, `SELECT
			(SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name IN ('push_subscriptions', 'timer_alarms'))
			+ (SELECT COUNT(*) FROM pragma_table_info('instance_settings') WHERE name = 'vapid_key')`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("up: %v", err)
	}
	if n := objects(); n != 3 {
		t.Fatalf("%d push objects after up, want 3", n)
	}
	if _, err := provider.DownTo(ctx, 18); err != nil {
		t.Fatalf("down to 18: %v", err)
	}
	if n := objects(); n != 0 {
		t.Fatalf("%d push objects left after down", n)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("up again: %v", err)
	}
}

// TestCommentImagesMigrationDownAndUp rolls 0020 back and applies it again,
// with a row in the table, so its Down section is known to undo exactly what
// its Up section creates.
func TestCommentImagesMigrationDownAndUp(t *testing.T) {
	ctx := context.Background()
	conn, err := Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	fsys, err := fs.Sub(migrations, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, conn, fsys)
	if err != nil {
		t.Fatal(err)
	}
	objects := func() int {
		t.Helper()
		var n int
		if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master
			WHERE name IN ('comment_images', 'comment_images_comment_idx', 'comment_images_staged_idx',
				'comment_images_recipe_idx', 'comment_images_recipe_image_idx')`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("up: %v", err)
	}
	for _, q := range []string{
		`INSERT INTO users (id, username, display_name, password_hash, role, color, locale, created_at, updated_at)
			VALUES ('u1', 'u', 'U', 'x', 'user', '#000', 'en', 't', 't')`,
		`INSERT INTO recipes (id, slug, title, created_by, created_at, updated_by, updated_at)
			VALUES ('r1', 'r', 'R', 'u1', 't', 'u1', 't')`,
		`INSERT INTO recipe_comments (recipe_id, author_id, body, created_at) VALUES ('r1', 'u1', 'hi', 't')`,
		`INSERT INTO comment_images (id, recipe_id, comment_id, uploader_id, width, height, size_bytes, created_at)
			VALUES ('c1', 'r1', 1, 'u1', 10, 10, 100, 't')`,
	} {
		if _, err := conn.ExecContext(ctx, q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	if n := objects(); n != 5 {
		t.Fatalf("%d comment image objects after up, want 5", n)
	}
	if _, err := provider.DownTo(ctx, 19); err != nil {
		t.Fatalf("down to 19: %v", err)
	}
	if n := objects(); n != 0 {
		t.Fatalf("%d comment image objects left after down", n)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("up again: %v", err)
	}
	if n := objects(); n != 5 {
		t.Fatalf("%d comment image objects after up again, want 5", n)
	}
}
