package recipe

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/s-frei/rezepte/service/internal/db"
	"github.com/s-frei/rezepte/service/internal/db/sqlc"
)

// SetTasty marks recipeID as tasty (on=true) or clears the mark (on=false)
// for userID. It follows SetFavorite: a state rather than an event, keyed
// on userID alone so a caller only ever changes their own mark, with the
// existence check on marking and none on clearing.
//
// Marking returns ErrOwnRecipe when userID wrote the recipe. Clearing is
// not checked that way, since there is nothing of the author's to clear.
func (s *Service) SetTasty(ctx context.Context, userID, recipeID string, on bool) error {
	if !on {
		if err := s.q.DeleteTasty(ctx, sqlc.DeleteTastyParams{UserID: userID, RecipeID: recipeID}); err != nil {
			return fmt.Errorf("delete tasty: %w", err)
		}
		return nil
	}
	row, err := s.q.GetRecipe(ctx, recipeID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("get recipe %s: %w", recipeID, err)
	}
	if row.CreatedBy == userID {
		return ErrOwnRecipe
	}
	if err := s.q.SetTasty(ctx, sqlc.SetTastyParams{
		UserID: userID, RecipeID: recipeID, CreatedAt: db.FormatTime(s.now()),
	}); err != nil {
		return fmt.Errorf("set tasty: %w", err)
	}
	return nil
}

// IsTasty reports whether userID marked recipeID tasty. An empty userID
// reports false without querying, the same guard IsFavorite applies.
func (s *Service) IsTasty(ctx context.Context, userID, recipeID string) (bool, error) {
	if userID == "" {
		return false, nil
	}
	mine, err := s.tastyOf(ctx, userID, []string{recipeID})
	if err != nil {
		return false, err
	}
	return mine[recipeID], nil
}

// tastyOf returns which of ids userID marked tasty, in one query.
func (s *Service) tastyOf(ctx context.Context, userID string, ids []string) (map[string]bool, error) {
	idsJSON, err := json.Marshal(ids)
	if err != nil {
		return nil, fmt.Errorf("marshal recipe ids: %w", err)
	}
	rows, err := s.q.ListTastyRecipeIDs(ctx, sqlc.ListTastyRecipeIDsParams{
		UserID: userID, RecipeIds: string(idsJSON),
	})
	if err != nil {
		return nil, fmt.Errorf("list tasty recipe ids: %w", err)
	}
	mine := make(map[string]bool, len(rows))
	for _, id := range rows {
		mine[id] = true
	}
	return mine, nil
}

// tastyCounts returns how many members marked each of ids, in one query.
// A recipe nobody marked is absent, which reads as zero.
func (s *Service) tastyCounts(ctx context.Context, ids []string) (map[string]int, error) {
	idsJSON, err := json.Marshal(ids)
	if err != nil {
		return nil, fmt.Errorf("marshal recipe ids: %w", err)
	}
	rows, err := s.q.ListTastyCounts(ctx, string(idsJSON))
	if err != nil {
		return nil, fmt.Errorf("list tasty counts: %w", err)
	}
	counts := make(map[string]int, len(rows))
	for _, r := range rows {
		counts[r.RecipeID] = int(r.TastyCount)
	}
	return counts, nil
}
