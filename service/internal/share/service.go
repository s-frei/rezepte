package share

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"
	"uuid"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"

	"github.com/s-frei/rezepte/service/internal/db"
	"github.com/s-frei/rezepte/service/internal/db/sqlc"
	"github.com/s-frei/rezepte/service/internal/settings"
	"github.com/s-frei/rezepte/service/internal/user"
)

// Service creates, lists, revokes, resolves and sweeps public recipe links.
type Service struct {
	q        *sqlc.Queries
	settings *settings.Service
	users    *user.Service
	now      func() time.Time
	// logger takes the failures the public routes answer with a 404 or the
	// app's own link preview, which would otherwise go unseen.
	logger *slog.Logger
}

// NewService returns a Service backed by conn. It logs to slog.Default(),
// which main.go sets before building it.
func NewService(conn *sql.DB, st *settings.Service, users *user.Service) *Service {
	return &Service{q: sqlc.New(conn), settings: st, users: users, now: time.Now, logger: slog.Default()}
}

// SetClock overrides the time source. Intended for tests.
func (s *Service) SetClock(now func() time.Time) { s.now = now }

// Create opens a public link to recipeID for actor. actor's CanSharePublicly
// is re-read from the database rather than trusted from the argument, since
// this is the write that authorization actually gates.
//
// An actor who already has a live link for this recipe gets ErrExists, with
// that existing Share returned alongside it - before days is validated at
// all, so a member who already shares gets their link back rather than a
// lifetime error for a request whose days no longer fits a since-lowered
// maximum. A share whose own expiry has passed does not count as existing:
// it is terminal (unlike paused or limited, nothing revives it), so it is
// cleared here rather than left to block a fresh link until the next Sweep.
//
// Only once there is no live existing share does days matter: it is one of
// settings.ShareLifetimes, or nil for permanent, and is refused with
// ErrLifetime when it is neither, or when it exceeds the instance maximum
// (a nil days exceeds any non-nil maximum).
func (s *Service) Create(ctx context.Context, actor user.User, recipeID string, days *int) (Share, error) {
	st, err := s.settings.Get(ctx)
	if err != nil {
		return Share{}, fmt.Errorf("get settings: %w", err)
	}
	if !st.PublicShares {
		return Share{}, ErrSharingOff
	}
	creator, err := s.users.ByID(ctx, actor.ID)
	if err != nil {
		return Share{}, fmt.Errorf("get creator %s: %w", actor.ID, err)
	}
	if !creator.CanSharePublicly {
		return Share{}, ErrNotAllowed
	}

	now := s.now()

	buildExisting := func(row sqlc.Share) (Share, error) {
		recipeRow, err := s.q.GetRecipe(ctx, recipeID)
		if err != nil {
			return Share{}, fmt.Errorf("get recipe %s: %w", recipeID, err)
		}
		return s.buildShare(row, recipeRow.Slug, recipeRow.Title, st, creator.CanSharePublicly, nil, now)
	}

	if existing, ok, err := s.liveOwnShare(ctx, recipeID, actor.ID, now); err != nil {
		return Share{}, err
	} else if ok {
		share, err := buildExisting(existing)
		if err != nil {
			return Share{}, err
		}
		return share, ErrExists
	}

	if days != nil && !slices.Contains(settings.ShareLifetimes, *days) {
		return Share{}, ErrLifetime
	}
	if st.PublicShareMaxDays != nil && (days == nil || *days > *st.PublicShareMaxDays) {
		return Share{}, ErrLifetime
	}
	recipeRow, err := s.q.GetRecipe(ctx, recipeID)
	if errors.Is(err, sql.ErrNoRows) {
		return Share{}, ErrRecipeNotFound
	}
	if err != nil {
		return Share{}, fmt.Errorf("get recipe %s: %w", recipeID, err)
	}
	token := newToken()
	var expiresAt *string
	if days != nil {
		e := db.FormatTime(now.AddDate(0, 0, *days))
		expiresAt = &e
	}
	params := sqlc.CreateShareParams{
		ID:        uuid.NewV7().String(),
		Token:     token,
		RecipeID:  recipeID,
		CreatedBy: actor.ID,
		CreatedAt: db.FormatTime(now),
		ExpiresAt: expiresAt,
	}
	row, err := s.q.CreateShare(ctx, params)
	if err != nil {
		if !isUniqueViolation(err) {
			return Share{}, fmt.Errorf("insert share: %w", err)
		}
		// Raced with a concurrent Create for the same recipe and actor
		// between the check above and this insert. The same rule applies:
		// a live row wins as ErrExists, an own-expired one is cleared and
		// the insert retried once.
		existing, ok, err := s.liveOwnShare(ctx, recipeID, actor.ID, now)
		if err != nil {
			return Share{}, err
		}
		if ok {
			share, err := buildExisting(existing)
			if err != nil {
				return Share{}, err
			}
			return share, ErrExists
		}
		row, err = s.q.CreateShare(ctx, params)
		if err != nil {
			return Share{}, fmt.Errorf("insert share after clearing an expired one: %w", err)
		}
	}
	return s.buildShare(row, recipeRow.Slug, recipeRow.Title, st, creator.CanSharePublicly, nil, now)
}

// liveOwnShare returns creatorID's current share for recipeID. A share whose
// own expiry has already passed is not live - it is terminal, so this
// deletes it and reports ok=false, exactly as if it did not exist.
func (s *Service) liveOwnShare(ctx context.Context, recipeID, creatorID string, now time.Time) (sqlc.Share, bool, error) {
	row, err := s.q.GetShareByRecipeAndCreator(ctx, sqlc.GetShareByRecipeAndCreatorParams{
		RecipeID: recipeID, CreatedBy: creatorID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return sqlc.Share{}, false, nil
	}
	if err != nil {
		return sqlc.Share{}, false, fmt.Errorf("get existing share for recipe %s: %w", recipeID, err)
	}
	expired, err := ownExpired(row.ExpiresAt, now)
	if err != nil {
		return sqlc.Share{}, false, err
	}
	if !expired {
		return row, true, nil
	}
	if _, err := s.q.DeleteShare(ctx, row.ID); err != nil {
		return sqlc.Share{}, false, fmt.Errorf("delete expired share %s: %w", row.ID, err)
	}
	return sqlc.Share{}, false, nil
}

// Mine returns actor's own share for recipeID, or ErrNotFound. A share whose
// own expiry has passed is terminal and is reported as ErrNotFound too - it
// is gone in every way that matters, whether or not Sweep has removed the
// row yet.
func (s *Service) Mine(ctx context.Context, actor user.User, recipeID string) (Share, error) {
	row, err := s.q.GetShareByRecipeAndCreator(ctx, sqlc.GetShareByRecipeAndCreatorParams{
		RecipeID: recipeID, CreatedBy: actor.ID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return Share{}, ErrNotFound
	}
	if err != nil {
		return Share{}, fmt.Errorf("get share for recipe %s: %w", recipeID, err)
	}
	now := s.now()
	expired, err := ownExpired(row.ExpiresAt, now)
	if err != nil {
		return Share{}, err
	}
	if expired {
		return Share{}, ErrNotFound
	}
	recipeRow, err := s.q.GetRecipe(ctx, recipeID)
	if errors.Is(err, sql.ErrNoRows) {
		return Share{}, ErrNotFound
	}
	if err != nil {
		return Share{}, fmt.Errorf("get recipe %s: %w", recipeID, err)
	}
	st, err := s.settings.Get(ctx)
	if err != nil {
		return Share{}, fmt.Errorf("get settings: %w", err)
	}
	return s.buildShare(row, recipeRow.Slug, recipeRow.Title, st, actor.CanSharePublicly, nil, now)
}

// List returns actor's own shares, newest first. all lists every share in
// the household instead, with each one's creator - admin only, else
// ErrAdminRequired. A share whose own expiry has passed is terminal and is
// left out of both lists, whether or not Sweep has removed the row yet.
func (s *Service) List(ctx context.Context, actor user.User, all bool) ([]Share, error) {
	if all && !actor.Role.IsAdmin() {
		return nil, ErrAdminRequired
	}
	st, err := s.settings.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("get settings: %w", err)
	}
	var rows []sqlc.ListAllSharesRow
	if all {
		if rows, err = s.q.ListAllShares(ctx); err != nil {
			return nil, fmt.Errorf("list all shares: %w", err)
		}
	} else {
		mine, err := s.q.ListSharesByCreator(ctx, actor.ID)
		if err != nil {
			return nil, fmt.Errorf("list shares of %s: %w", actor.ID, err)
		}
		for _, row := range mine {
			rows = append(rows, sqlc.ListAllSharesRow(row))
		}
	}
	now := s.now()
	out := make([]Share, 0, len(rows))
	for _, row := range rows {
		expired, err := ownExpired(row.ExpiresAt, now)
		if err != nil {
			return nil, err
		}
		if expired {
			continue
		}
		var creator *Creator
		if all {
			creator = &Creator{ID: row.CreatedBy, DisplayName: row.CreatorDisplayName, Color: row.CreatorColor}
		}
		share, err := s.buildShare(
			sqlc.Share{ID: row.ID, Token: row.Token, RecipeID: row.RecipeID, CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt, ExpiresAt: row.ExpiresAt},
			row.RecipeSlug, row.RecipeTitle, st, row.CreatorCanSharePublicly, creator, now,
		)
		if err != nil {
			return nil, err
		}
		out = append(out, share)
	}
	return out, nil
}

// Revoke deletes a share: actor's own, or any as admin. An id that does
// not exist, or belongs to somebody else and actor is not an admin, is
// ErrNotFound.
func (s *Service) Revoke(ctx context.Context, actor user.User, id string) error {
	if actor.Role.IsAdmin() {
		n, err := s.q.DeleteShare(ctx, id)
		if err != nil {
			return fmt.Errorf("delete share %s: %w", id, err)
		}
		if n == 0 {
			return ErrNotFound
		}
		return nil
	}
	n, err := s.q.DeleteShareByCreator(ctx, sqlc.DeleteShareByCreatorParams{ID: id, CreatedBy: actor.ID})
	if err != nil {
		return fmt.Errorf("delete share %s: %w", id, err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// RevokeAll deletes every public link in the household. Admin only, else
// ErrAdminRequired.
func (s *Service) RevokeAll(ctx context.Context, actor user.User) error {
	if !actor.Role.IsAdmin() {
		return ErrAdminRequired
	}
	if err := s.q.DeleteAllShares(ctx); err != nil {
		return fmt.Errorf("delete all shares: %w", err)
	}
	return nil
}

// Resolve returns the recipe token points at, but only while its share
// serves: ok is false for an unknown, revoked, paused, limited or expired
// token.
func (s *Service) Resolve(ctx context.Context, token string) (string, bool, error) {
	row, err := s.q.GetShareByToken(ctx, token)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("get share by token: %w", err)
	}
	st, err := s.settings.Get(ctx)
	if err != nil {
		return "", false, fmt.Errorf("get settings: %w", err)
	}
	createdAt, expiresAt, err := parseTimes(row.CreatedAt, row.ExpiresAt)
	if err != nil {
		return "", false, err
	}
	_, _, serving := evaluate(createdAt, expiresAt, st.PublicShares, row.CreatorCanSharePublicly, st.PublicShareMaxDays, s.now())
	if !serving {
		return "", false, nil
	}
	return row.RecipeID, true, nil
}

// Sweep deletes shares whose own expiry has passed. A link dormant only
// because of the instance maximum stays: raising the maximum again must
// revive it, which a deleted row could not.
func (s *Service) Sweep(ctx context.Context, now time.Time) error {
	formatted := db.FormatTime(now)
	if err := s.q.DeleteExpiredShares(ctx, &formatted); err != nil {
		return fmt.Errorf("delete expired shares: %w", err)
	}
	return nil
}

// SweepLoop calls Sweep every d until ctx is canceled, logging failures at
// warn instead of returning them so a transient database error never stops
// the sweep for good. Intended to run in its own goroutine.
func (s *Service) SweepLoop(ctx context.Context, d time.Duration, logger *slog.Logger) {
	ticker := time.NewTicker(d)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.Sweep(ctx, s.now()); err != nil {
				logger.Warn("sweep expired shares", "err", err)
			}
		}
	}
}

// buildShare converts a stored row into a public Share, running the
// validity rule (evaluate) against st and creatorAllowed to fill Status and
// the effective ExpiresAt.
func (s *Service) buildShare(row sqlc.Share, recipeSlug, recipeTitle string, st settings.Settings, creatorAllowed bool, createdBy *Creator, now time.Time) (Share, error) {
	createdAt, expiresAt, err := parseTimes(row.CreatedAt, row.ExpiresAt)
	if err != nil {
		return Share{}, err
	}
	status, effective, _ := evaluate(createdAt, expiresAt, st.PublicShares, creatorAllowed, st.PublicShareMaxDays, now)
	return Share{
		ID:        row.ID,
		Recipe:    RecipeRef{ID: row.RecipeID, Slug: recipeSlug, Title: recipeTitle},
		Path:      "/s/" + row.Token,
		CreatedAt: createdAt,
		ExpiresAt: effective,
		Status:    status,
		CreatedBy: createdBy,
	}, nil
}

// parseTimes parses a share row's stored timestamps.
func parseTimes(createdAt string, expiresAt *string) (time.Time, *time.Time, error) {
	created, err := db.ParseTime(createdAt)
	if err != nil {
		return time.Time{}, nil, err
	}
	if expiresAt == nil {
		return created, nil, nil
	}
	expires, err := db.ParseTime(*expiresAt)
	if err != nil {
		return time.Time{}, nil, err
	}
	return created, &expires, nil
}

// ownExpired reports whether a share's own expiry has passed. Unlike being
// paused or limited, this is terminal: nothing reverses it, so callers that
// find it true treat the share as gone rather than merely dormant.
func ownExpired(expiresAt *string, now time.Time) (bool, error) {
	if expiresAt == nil {
		return false, nil
	}
	t, err := db.ParseTime(*expiresAt)
	if err != nil {
		return false, err
	}
	return !t.After(now), nil
}

// newToken returns 16 random bytes from crypto/rand, base64url encoded
// without padding - 22 characters, 128 bits of entropy.
func newToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b) // never fails since Go 1.24
	return base64.RawURLEncoding.EncodeToString(b)
}

// isUniqueViolation reports whether err is a SQLite UNIQUE constraint
// failure, such as the one an insert hits on shares' UNIQUE(recipe_id,
// created_by) when the actor already has a link for the recipe.
func isUniqueViolation(err error) bool {
	var serr *sqlite.Error
	return errors.As(err, &serr) && serr.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE
}
