// Package settings holds the household-wide settings the instance owner
// controls. There is exactly one row; the migration creates it.
package settings

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"

	"github.com/s-frei/rezepte/service/internal/db"
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

// ErrUnknownShareLifetime is returned for a public share lifetime other than
// ShareLifetimes.
var ErrUnknownShareLifetime = errors.New("public share lifetime must be 1, 7, 30 or 365 days")

// PreviewLifetimes are the minutes a link preview may last, shortest first.
var PreviewLifetimes = []int{15, 60, 1440}

// ShareLifetimes are the days a public share may last, shortest first. Absent
// from a request or nil in Settings means permanent.
var ShareLifetimes = []int{1, 7, 30, 365}

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
	// PublicShares lets members turn a recipe into a public, read-only link.
	PublicShares bool `json:"publicShares"`
	// PublicShareDefaultDays is the lifetime preselected when a member
	// creates a public link; nil is permanent.
	PublicShareDefaultDays *int `json:"publicShareDefaultDays" enum:"1,7,30,365"`
	// PublicShareMaxDays is the longest lifetime a public link may have; nil
	// is no maximum (permanent links allowed).
	PublicShareMaxDays *int `json:"publicShareMaxDays" enum:"1,7,30,365"`
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
		PublicShares:           row.PublicShares,
		PublicShareDefaultDays: db.Conv[int](row.PublicShareDefaultDays),
		PublicShareMaxDays:     db.Conv[int](row.PublicShareMaxDays),
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

// SetPublicShares switches whether members may create public recipe links.
// Only the superadmin may; everybody else gets ErrOwnerRequired.
func (s *Service) SetPublicShares(ctx context.Context, actor user.User, on bool) (Settings, error) {
	if !actor.Role.IsSuperadmin() {
		return Settings{}, ErrOwnerRequired
	}
	if err := s.q.SetPublicShares(ctx, on); err != nil {
		return Settings{}, fmt.Errorf("set public shares: %w", err)
	}
	return s.Get(ctx)
}

// SetShareLifetimes chooses the default and maximum lifetime, in days, a new
// public share gets; each is nil (permanent) or one of ShareLifetimes. Only
// the superadmin may; everybody else gets ErrOwnerRequired, any other day
// count ErrUnknownShareLifetime. When maxDays is set and defaultDays is nil
// or exceeds it, defaultDays is lowered to maxDays: the default may never
// promise a lifetime the maximum forbids. Both columns are written in one
// statement, so a reader never sees a default the new maximum already
// disallows.
func (s *Service) SetShareLifetimes(ctx context.Context, actor user.User, defaultDays, maxDays *int) (Settings, error) {
	if !actor.Role.IsSuperadmin() {
		return Settings{}, ErrOwnerRequired
	}
	if defaultDays != nil && !slices.Contains(ShareLifetimes, *defaultDays) {
		return Settings{}, ErrUnknownShareLifetime
	}
	if maxDays != nil && !slices.Contains(ShareLifetimes, *maxDays) {
		return Settings{}, ErrUnknownShareLifetime
	}
	if maxDays != nil && (defaultDays == nil || *defaultDays > *maxDays) {
		defaultDays = maxDays
	}
	if err := s.q.SetShareLifetimes(ctx, sqlc.SetShareLifetimesParams{
		PublicShareDefaultDays: db.Conv[int64](defaultDays),
		PublicShareMaxDays:     db.Conv[int64](maxDays),
	}); err != nil {
		return Settings{}, fmt.Errorf("set share lifetimes: %w", err)
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
