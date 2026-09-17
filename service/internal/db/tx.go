package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/s-frei/rezepte/service/internal/db/sqlc"
)

// Tx runs fn inside a transaction. The transaction is committed when fn
// returns nil and rolled back otherwise (including panics).
func Tx(ctx context.Context, conn *sql.DB, fn func(q *sqlc.Queries) error) (err error) {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	if err = fn(sqlc.New(conn).WithTx(tx)); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}
