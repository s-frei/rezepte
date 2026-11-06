package db

import (
	"database/sql"
	"io/fs"
	"path/filepath"
	"slices"
	"testing"

	"github.com/pressly/goose/v3"
)

// openAt opens a fresh database migrated up to version, and the provider
// that moves it further.
func openAt(t *testing.T, version int64) (*sql.DB, *goose.Provider) {
	t.Helper()
	conn, err := Open(t.Context(), filepath.Join(t.TempDir(), "test.db"))
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
	if _, err := provider.UpTo(t.Context(), version); err != nil {
		t.Fatalf("up to %d: %v", version, err)
	}
	return conn, provider
}

// TestRecipeCommentsMigrationDownAndUp rolls 0014 back and applies it again,
// so its Down section is known to undo exactly what its Up section creates.
func TestRecipeCommentsMigrationDownAndUp(t *testing.T) {
	ctx := t.Context()
	conn, provider := openAt(t, goose.MaxVersion)
	tables := func() int {
		t.Helper()
		var n int
		if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master
			WHERE type = 'table' AND name IN ('recipe_comments', 'recipe_comment_reads')`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
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
	ctx := t.Context()
	conn, provider := openAt(t, goose.MaxVersion)
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
	ctx := t.Context()
	conn, provider := openAt(t, goose.MaxVersion)
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

// TestNotificationChannelsMigration moves 0020's mail opt-outs into 0021's
// channels: a group with every kind off becomes one group row, the rest stay
// kind rows, push starts empty; Down expands the group rows back.
func TestNotificationChannelsMigration(t *testing.T) {
	ctx := t.Context()
	conn, provider := openAt(t, 20)
	for _, q := range []string{
		`INSERT INTO users (id, username, display_name, password_hash, role, color, locale, created_at, updated_at) VALUES
			('a', 'a', 'A', 'x', 'user', '#000', 'en', 't', 't'),
			('b', 'b', 'B', 'x', 'user', '#000', 'en', 't', 't'),
			('c', 'c', 'C', 'x', 'admin', '#000', 'en', 't', 't')`,
		`INSERT INTO notification_optouts (user_id, kind) VALUES
			('a', 'comment'), ('a', 'recipe'), ('b', 'comment'), ('c', 'invite_accepted')`,
		`INSERT INTO sessions (id, user_id, expires_at, created_at) VALUES ('sa', 'a', 't', 't')`,
		`INSERT INTO push_subscriptions (id, user_id, endpoint, p256dh, auth, created_at) VALUES ('old', 'a', 'https://x/old', x'00', x'00', 't')`,
		`INSERT INTO timer_alarms (id, subscription_id, fire_at, title, body, url, created_at) VALUES ('al', 'old', 't', 'T', '', '/recipes/x', 't')`,
	} {
		if _, err := conn.ExecContext(ctx, q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	rows := func(query string) []string {
		t.Helper()
		r, err := conn.QueryContext(ctx, query)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = r.Close() }()
		var got []string
		for r.Next() {
			var a, b, c string
			if err := r.Scan(&a, &b, &c); err != nil {
				t.Fatal(err)
			}
			got = append(got, a+":"+b+":"+c)
		}
		if err := r.Err(); err != nil {
			t.Fatal(err)
		}
		return got
	}

	if _, err := provider.UpTo(ctx, 21); err != nil {
		t.Fatalf("up to 21: %v", err)
	}
	if got := rows(`SELECT user_id, kind, channel FROM notification_optouts ORDER BY 1, 2`); !slices.Equal(got, []string{"b:comment:mail"}) {
		t.Errorf("kind rows %v", got)
	}
	if got := rows(`SELECT user_id, grp, channel FROM notification_group_optouts ORDER BY 1, 2`); !slices.Equal(got, []string{"a:household:mail", "c:admin:mail"}) {
		t.Errorf("group rows %v", got)
	}
	if _, err := conn.ExecContext(ctx, `INSERT INTO notification_optouts (user_id, kind, channel) VALUES ('a', 'comment', 'sms')`); err == nil {
		t.Error("channel sms accepted")
	}
	var alerts string
	if err := conn.QueryRowContext(ctx, `SELECT mail_alert_last_at FROM instance_settings WHERE id = 1`).Scan(&alerts); err != nil || alerts != "" {
		t.Errorf("mail alert state %q, %v", alerts, err)
	}

	count := func(query string) int {
		t.Helper()
		var n int
		if err := conn.QueryRowContext(ctx, query).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if n := count(`SELECT (SELECT COUNT(*) FROM push_subscriptions) + (SELECT COUNT(*) FROM timer_alarms)`); n != 0 {
		t.Errorf("%d subscriptions and alarms without a session kept", n)
	}
	for _, q := range []string{
		`INSERT INTO push_subscriptions (id, user_id, session_id, endpoint, p256dh, auth, created_at) VALUES
			('p1', 'a', 'sa', 'https://x/1', x'00', x'00', 't')`,
		`INSERT INTO timer_alarms (id, subscription_id, fire_at, title, body, url, created_at) VALUES ('a1', 'p1', 't', 'T', '', '/recipes/x', 't')`,
	} {
		if _, err := conn.ExecContext(ctx, q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	if _, err := conn.ExecContext(ctx, `INSERT INTO push_subscriptions (id, user_id, session_id, endpoint, p256dh, auth, created_at) VALUES
		('p2', 'a', 'gone', 'https://x/2', x'00', x'00', 't')`); err == nil {
		t.Error("subscription of an unknown session accepted")
	}

	if _, err := provider.DownTo(ctx, 20); err != nil {
		t.Fatalf("down to 20: %v", err)
	}
	if n := count(`SELECT (SELECT COUNT(*) FROM push_subscriptions WHERE id = 'p1') + (SELECT COUNT(*) FROM timer_alarms WHERE subscription_id = 'p1')
		+ (SELECT COUNT(*) FROM pragma_table_info('push_subscriptions') WHERE name = 'session_id')`); n != 2 {
		t.Errorf("after down: %d of subscription, alarm and no session column", n)
	}
	if got := rows(`SELECT user_id, kind, '' FROM notification_optouts ORDER BY 1, 2`); !slices.Equal(got, []string{"a:comment:", "a:recipe:", "b:comment:", "c:invite_accepted:"}) {
		t.Errorf("rows after down %v", got)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("up again: %v", err)
	}
	if _, err := conn.ExecContext(ctx, `INSERT INTO push_subscriptions (id, user_id, session_id, endpoint, p256dh, auth, created_at) VALUES
		('p3', 'a', 'sa', 'https://x/3', x'00', x'00', 't')`); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, `DELETE FROM sessions WHERE id = 'sa'`); err != nil {
		t.Fatal(err)
	}
	if n := count(`SELECT COUNT(*) FROM push_subscriptions`); n != 0 {
		t.Errorf("%d subscriptions outlive their session", n)
	}
}

// TestCoverFirstMigration moves a recipe's cover to position 0, leaves a
// recipe whose cover is already first alone and gives a recipe without one
// its first photo; Down is a no-op.
func TestCoverFirstMigration(t *testing.T) {
	ctx := t.Context()
	conn, provider := openAt(t, 21)
	for _, q := range []string{
		`INSERT INTO users (id, username, display_name, password_hash, role, color, locale, created_at, updated_at)
			VALUES ('u1', 'u', 'U', 'x', 'user', '#000', 'en', 't', 't')`,
		`INSERT INTO recipes (id, slug, title, created_by, created_at, updated_by, updated_at, cover_image_id) VALUES
			('r1', 'r1', 'R1', 'u1', 't', 'u1', 't', 'b'),
			('r2', 'r2', 'R2', 'u1', 't', 'u1', 't', 'x')`,
		`INSERT INTO recipes (id, slug, title, created_by, created_at, updated_by, updated_at) VALUES
			('r3', 'r3', 'R3', 'u1', 't', 'u1', 't'), ('r4', 'r4', 'R4', 'u1', 't', 'u1', 't')`,
		`INSERT INTO images (id, recipe_id, filename, width, height, size_bytes, position, created_at) VALUES
			('a', 'r1', 'a', 1, 1, 1, 0, 't'), ('b', 'r1', 'b', 1, 1, 1, 1, 't'), ('c', 'r1', 'c', 1, 1, 1, 2, 't'),
			('x', 'r2', 'x', 1, 1, 1, 0, 't'), ('y', 'r2', 'y', 1, 1, 1, 1, 't'),
			('p', 'r3', 'p', 1, 1, 1, 0, 't'), ('q', 'r3', 'q', 1, 1, 1, 1, 't'),
			('m', 'r4', 'm', 1, 1, 1, 2, 't'), ('n', 'r4', 'n', 1, 1, 1, 1, 't')`,
	} {
		if _, err := conn.ExecContext(ctx, q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	order := func() string {
		t.Helper()
		var s string
		if err := conn.QueryRowContext(ctx, `SELECT group_concat(id || position, ' ') FROM (SELECT * FROM images ORDER BY recipe_id, position)`).Scan(&s); err != nil {
			t.Fatal(err)
		}
		return s
	}
	if _, err := provider.UpTo(ctx, 22); err != nil {
		t.Fatalf("up to 22: %v", err)
	}
	if got, want := order(), "b0 a1 c2 x0 y1 p0 q1 n1 m2"; got != want {
		t.Fatalf("after up = %q, want %q", got, want)
	}
	// A recipe without a cover gets its first photo, wherever its positions start.
	var covers string
	if err := conn.QueryRowContext(ctx, `SELECT group_concat(cover_image_id, ' ') FROM (SELECT * FROM recipes WHERE id IN ('r3', 'r4') ORDER BY id)`).Scan(&covers); err != nil || covers != "p n" {
		t.Fatalf("covers of recipes without one = %q, %v; want p n", covers, err)
	}
	if _, err := provider.DownTo(ctx, 21); err != nil {
		t.Fatalf("down to 21: %v", err)
	}
	if got, want := order(), "b0 a1 c2 x0 y1 p0 q1 n1 m2"; got != want {
		t.Fatalf("after down = %q, want %q", got, want)
	}
}

// TestCommentImagePositionMigration numbers existing attached photos by
// insertion order; Down drops the column.
func TestCommentImagePositionMigration(t *testing.T) {
	ctx := t.Context()
	conn, provider := openAt(t, 22)
	for _, q := range []string{
		`INSERT INTO users (id, username, display_name, password_hash, role, color, locale, created_at, updated_at)
			VALUES ('u1', 'u', 'U', 'x', 'user', '#000', 'en', 't', 't')`,
		`INSERT INTO recipes (id, slug, title, created_by, created_at, updated_by, updated_at)
			VALUES ('r1', 'r', 'R', 'u1', 't', 'u1', 't')`,
		`INSERT INTO recipe_comments (recipe_id, author_id, body, created_at) VALUES ('r1', 'u1', 'hi', 't')`,
		`INSERT INTO comment_images (id, recipe_id, comment_id, uploader_id, width, height, size_bytes, created_at) VALUES
			('x', 'r1', 1, 'u1', 10, 10, 100, 't'), ('y', 'r1', 1, 'u1', 10, 10, 100, 't')`,
	} {
		if _, err := conn.ExecContext(ctx, q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	if _, err := provider.UpTo(ctx, 23); err != nil {
		t.Fatalf("up to 23: %v", err)
	}
	var got string
	if err := conn.QueryRowContext(ctx, `SELECT group_concat(id || position, ' ') FROM (SELECT * FROM comment_images ORDER BY position)`).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != "x0 y1" {
		t.Fatalf("positions = %q, want %q", got, "x0 y1")
	}
	if _, err := provider.DownTo(ctx, 22); err != nil {
		t.Fatalf("down to 22: %v", err)
	}
}

// TestUserLastActiveMigration backfills 0025's last_active_at from the later
// of a session's last renewal (expiry minus 30 days) and a token's last use;
// an account with neither stays NULL. Down drops the column.
func TestUserLastActiveMigration(t *testing.T) {
	ctx := t.Context()
	conn, provider := openAt(t, 24)
	for _, q := range []string{
		`INSERT INTO users (id, username, display_name, password_hash, role, color, locale, created_at, updated_at) VALUES
			('s', 's', 'S', 'x', 'user', '#000', 'en', 't', 't'),
			('a', 'a', 'A', 'x', 'admin', '#000', 'en', 't', 't'),
			('n', 'n', 'N', 'x', 'user', '#000', 'en', 't', 't')`,
		`INSERT INTO sessions (id, user_id, expires_at, created_at) VALUES
			('s1', 's', '2026-03-01T10:00:00Z', 't'), ('s2', 's', '2026-03-05T10:00:00Z', 't'),
			('a1', 'a', '2026-03-31T08:00:00Z', 't')`,
		`INSERT INTO api_tokens (id, user_id, name, token_hash, token_prefix, scopes, created_at, last_used_at) VALUES
			('t1', 'a', 'mcp', 'h1', 'p1', '', 't', '2026-03-02T09:00:00Z'),
			('t2', 'a', 'cli', 'h2', 'p2', '', 't', NULL)`,
	} {
		if _, err := conn.ExecContext(ctx, q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	if _, err := provider.UpTo(ctx, 25); err != nil {
		t.Fatalf("up to 25: %v", err)
	}
	var got string
	if err := conn.QueryRowContext(ctx, `SELECT group_concat(id || '=' || coalesce(last_active_at, 'null'), ' ')
		FROM (SELECT * FROM users ORDER BY id)`).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if want := "a=2026-03-02T09:00:00Z n=null s=2026-02-03T10:00:00Z"; got != want {
		t.Fatalf("backfill = %q, want %q", got, want)
	}
	if _, err := provider.DownTo(ctx, 24); err != nil {
		t.Fatalf("down to 24: %v", err)
	}
}
