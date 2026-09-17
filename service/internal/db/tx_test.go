package db_test

import (
	"context"
	"errors"
	"testing"

	"github.com/s-frei/rezepte/service/internal/db"
	"github.com/s-frei/rezepte/service/internal/db/dbtest"
	"github.com/s-frei/rezepte/service/internal/db/sqlc"
)

func TestTxCommitsAndRollsBack(t *testing.T) {
	ctx := context.Background()
	conn := dbtest.Open(t)
	err := db.Tx(ctx, conn, func(q *sqlc.Queries) error {
		_, err := q.UpsertTag(ctx, sqlc.UpsertTagParams{ID: "t1", Name: "kept"})
		return err
	})
	if err != nil {
		t.Fatalf("commit tx: %v", err)
	}
	boom := errors.New("boom")
	err = db.Tx(ctx, conn, func(q *sqlc.Queries) error {
		if _, err := q.UpsertTag(ctx, sqlc.UpsertTagParams{ID: "t2", Name: "dropped"}); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}
	var n int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM tags`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("tags = %d (%v), want 1 (rollback)", n, err)
	}
}
