package db_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/s-frei/rezepte/service/internal/db"
	"github.com/s-frei/rezepte/service/internal/db/dbtest"
	"github.com/s-frei/rezepte/service/internal/db/sqlc"
)

func minutes(t *testing.T, ctx context.Context, q *sqlc.Queries) int64 {
	t.Helper()
	s, err := q.GetInstanceSettings(ctx)
	if err != nil {
		t.Fatalf("GetInstanceSettings: %v", err)
	}
	return s.LinkPreviewMinutes
}

func TestTx(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn := dbtest.Open(t)
	q := sqlc.New(conn)
	before := minutes(t, ctx, q)
	other := int64(15) // link_preview_minutes only takes 15, 60 or 1440
	if before == other {
		other = 1440
	}

	set := func(v int64) func(q *sqlc.Queries) error {
		return func(q *sqlc.Queries) error { return q.SetLinkPreviewMinutes(ctx, v) }
	}

	boom := errors.New("boom")
	if err := db.Tx(ctx, conn, func(q *sqlc.Queries) error {
		_ = set(other)(q)
		return boom
	}); !errors.Is(err, boom) {
		t.Fatalf("error = %v, want boom", err)
	}
	if got := minutes(t, ctx, q); got != before {
		t.Fatalf("after error = %d, want rollback to %d", got, before)
	}

	func() {
		defer func() {
			if p := recover(); p != "panic in fn" {
				t.Fatalf("recovered %v, want the panic re-raised", p)
			}
		}()
		_ = db.Tx(ctx, conn, func(q *sqlc.Queries) error {
			_ = set(other)(q)
			panic("panic in fn")
		})
	}()
	if got := minutes(t, ctx, q); got != before {
		t.Fatalf("after panic = %d, want rollback to %d", got, before)
	}

	if err := db.Tx(ctx, conn, set(other)); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if got := minutes(t, ctx, q); got != other {
		t.Fatalf("after commit = %d, want %d", got, other)
	}
}
