// Package settings holds the household-wide settings the instance owner
// controls. There is exactly one row; the migration creates it.
package settings

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"

	"github.com/s-frei/rezepte/service/internal/db/sqlc"
	"github.com/s-frei/rezepte/service/internal/user"
)

// ErrOwnerRequired is returned when anyone but the superadmin writes a setting.
var ErrOwnerRequired = errors.New("only the superadmin can change instance settings")

// ErrUnknownLifetime is returned for a link preview lifetime other than
// PreviewLifetimes.
var ErrUnknownLifetime = errors.New("link preview lifetime must be 15, 60 or 1440 minutes")

// ErrNoLinkPreviewKey is returned when the stored signing key is missing or
// not 32 bytes - a database the link preview migration has not run on.
var ErrNoLinkPreviewKey = errors.New("link preview key missing")

// PreviewLifetimes are the minutes a link preview may last, shortest first.
var PreviewLifetimes = []int{15, 60, 1440}

// Settings is the household-wide configuration.
type Settings struct {
	// RecipesLockedByDefault makes every recipe whose edit policy is
	// "default" editable only by its author and admins.
	RecipesLockedByDefault bool `json:"recipesLockedByDefault"`
	// LinkPreviews lets a recipe link shared from the app show the recipe's
	// title, description and cover to a crawler that is not signed in.
	LinkPreviews bool `json:"linkPreviews"`
	// LinkPreviewMinutes is how long such a link shows the recipe.
	LinkPreviewMinutes int `json:"linkPreviewMinutes" enum:"15,60,1440"`
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
	return Settings{
		RecipesLockedByDefault: row.RecipesLockedByDefault,
		LinkPreviews:           row.LinkPreviews,
		LinkPreviewMinutes:     int(row.LinkPreviewMinutes),
	}, nil
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

// SetLinkPreviews switches link previews on or off. Only the superadmin
// may; everybody else gets ErrOwnerRequired.
func (s *Service) SetLinkPreviews(ctx context.Context, actor user.User, on bool) (Settings, error) {
	if !actor.Role.IsSuperadmin() {
		return Settings{}, ErrOwnerRequired
	}
	if err := s.q.SetLinkPreviews(ctx, on); err != nil {
		return Settings{}, fmt.Errorf("set link previews: %w", err)
	}
	return s.Get(ctx)
}

// SetLinkPreviewMinutes chooses how long a link preview lasts, one of
// PreviewLifetimes. Only the superadmin may; everybody else gets
// ErrOwnerRequired, any other value ErrUnknownLifetime.
func (s *Service) SetLinkPreviewMinutes(ctx context.Context, actor user.User, minutes int) (Settings, error) {
	if !actor.Role.IsSuperadmin() {
		return Settings{}, ErrOwnerRequired
	}
	if !slices.Contains(PreviewLifetimes, minutes) {
		return Settings{}, ErrUnknownLifetime
	}
	if err := s.q.SetLinkPreviewMinutes(ctx, int64(minutes)); err != nil {
		return Settings{}, fmt.Errorf("set link preview minutes: %w", err)
	}
	return s.Get(ctx)
}

// LinkPreviewKey returns the 32-byte key link preview tokens are signed
// with. It never leaves the service: Settings does not carry it.
func (s *Service) LinkPreviewKey(ctx context.Context) ([]byte, error) {
	key, err := s.q.GetLinkPreviewKey(ctx)
	if err != nil {
		return nil, fmt.Errorf("get link preview key: %w", err)
	}
	if len(key) != 32 {
		return nil, ErrNoLinkPreviewKey
	}
	return key, nil
}
