package mealplan

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/s-frei/rezepte/service/internal/db/sqlc"
	"github.com/s-frei/rezepte/service/internal/ingredient"
)

// Staple is a name a plan's people always have at home ("Haben wir immer"),
// keyed by ingredient.Key so "Zwiebeln" and "zwiebel" are one staple.
type Staple struct {
	NameKey string `json:"nameKey"`
	Name    string `json:"name"`
}

// Staples returns a plan's staples by key.
func (s *Service) Staples(ctx context.Context, userID, planID string) ([]Staple, error) {
	if err := s.canAccess(ctx, userID, planID); err != nil {
		return nil, err
	}
	return s.staples(ctx, planID)
}

// SetStaple marks name as a staple of the plan or unmarks it, and returns the
// plan's staples. Marking a key that is a staple already keeps its first name.
func (s *Service) SetStaple(ctx context.Context, userID, planID, name string, staple bool) ([]Staple, error) {
	name, err := clean("name", name, 200)
	if err != nil {
		return nil, err
	}
	key := ingredient.Key(name)
	if err := s.canAccess(ctx, userID, planID); err != nil {
		return nil, err
	}
	if staple {
		err = s.q.InsertMealPlanStaple(ctx, sqlc.InsertMealPlanStapleParams{PlanID: planID, NameKey: key, Name: name, CreatedAt: s.stamp()})
	} else {
		err = s.q.DeleteMealPlanStaple(ctx, sqlc.DeleteMealPlanStapleParams{PlanID: planID, NameKey: key})
	}
	if err != nil {
		return nil, fmt.Errorf("set meal plan staple: %w", err)
	}
	return s.staples(ctx, planID)
}

func (s *Service) staples(ctx context.Context, planID string) ([]Staple, error) {
	rows, err := s.q.ListMealPlanStaples(ctx, planID)
	if err != nil {
		return nil, fmt.Errorf("list meal plan staples: %w", err)
	}
	out := make([]Staple, 0, len(rows))
	for _, r := range rows {
		out = append(out, Staple{NameKey: r.NameKey, Name: r.Name})
	}
	return out, nil
}

// canAccess reports ErrNotFound unless the person created or accepted the plan.
func (s *Service) canAccess(ctx context.Context, userID, planID string) error {
	_, err := s.q.GetMealPlanRevision(ctx, sqlc.GetMealPlanRevisionParams{ID: planID, UserID: userID})
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("get meal plan revision: %w", err)
	}
	return nil
}
