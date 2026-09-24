// Package settings holds the household-wide settings the instance owner
// controls. There is exactly one row; the migration creates it.
package settings

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/s-frei/rezepte/service/internal/db/sqlc"
	"github.com/s-frei/rezepte/service/internal/user"
)

// ErrOwnerRequired is returned when anyone but the superadmin writes a setting.
var ErrOwnerRequired = errors.New("only the superadmin can change instance settings")

// Settings is the household-wide configuration.
type Settings struct {
	// RecipesLockedByDefault makes every recipe whose edit policy is
	// "default" editable only by its author and admins.
	RecipesLockedByDefault bool `json:"recipesLockedByDefault"`
}

// Service reads and writes the instance settings.
type Service struct {
	q *sqlc.Queries
}

// NewService returns a Service backed by conn.
func NewService(conn *sql.DB) *Service {
	return &Service{q: sqlc.New(conn)}
}

// Get returns the current settings.
func (s *Service) Get(ctx context.Context) (Settings, error) {
	row, err := s.q.GetInstanceSettings(ctx)
	if err != nil {
		return Settings{}, fmt.Errorf("get instance settings: %w", err)
	}
	return Settings{RecipesLockedByDefault: row.RecipesLockedByDefault}, nil
}

// SetRecipesLockedByDefault switches the household default. Only the
// superadmin may; everybody else gets ErrOwnerRequired.
func (s *Service) SetRecipesLockedByDefault(ctx context.Context, actor user.User, on bool) (Settings, error) {
	if !actor.Role.IsSuperadmin() {
		return Settings{}, ErrOwnerRequired
	}
	if err := s.q.SetRecipesLockedByDefault(ctx, on); err != nil {
		return Settings{}, fmt.Errorf("set recipes locked by default: %w", err)
	}
	return s.Get(ctx)
}
