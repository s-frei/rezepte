// Package auth issues and validates login sessions and exposes the auth API.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/s-frei/rezepte/service/internal/db"
	"github.com/s-frei/rezepte/service/internal/db/sqlc"
	"github.com/s-frei/rezepte/service/internal/user"
)

// CookieName is the session cookie sent to browsers.
const CookieName = "rezepte_session"

// SessionTTL is how long a session lives without activity.
const SessionTTL = 30 * 24 * time.Hour

// renewAfter limits how often a sliding session is written back.
const renewAfter = 24 * time.Hour

// ErrNoSession means the token is unknown or expired.
var ErrNoSession = errors.New("session not found or expired")

// Session is what the client receives after login.
type Session struct {
	Token     string
	ExpiresAt time.Time
	User      user.User
}

// Service manages sessions.
type Service struct {
	q     *sqlc.Queries
	users *user.Service
	now   func() time.Time
}

// NewService returns a Service backed by conn.
func NewService(conn *sql.DB, users *user.Service) *Service {
	return &Service{q: sqlc.New(conn), users: users, now: time.Now}
}

// SetClock overrides the time source. Intended for tests.
func (s *Service) SetClock(now func() time.Time) { s.now = now }

// Login verifies the credentials and creates a session.
func (s *Service) Login(ctx context.Context, username, password string) (Session, error) {
	u, err := s.users.Authenticate(ctx, username, password)
	if err != nil {
		return Session{}, err
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return Session{}, fmt.Errorf("generate session token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	now := s.now()
	expires := now.Add(SessionTTL)
	err = s.q.CreateSession(ctx, sqlc.CreateSessionParams{
		ID:        hashToken(token),
		UserID:    u.ID,
		ExpiresAt: db.FormatTime(expires),
		CreatedAt: db.FormatTime(now),
	})
	if err != nil {
		return Session{}, fmt.Errorf("insert session: %w", err)
	}
	return Session{Token: token, ExpiresAt: expires, User: u}, nil
}

// Verified is what a successful Authenticate call reports: the resolved
// user, the session's current expiry, and whether that expiry was just
// extended by a sliding renewal. Callers that hold the session cookie (the
// auth middleware) use Renewed to decide whether to re-issue it with the
// fresh ExpiresAt.
type Verified struct {
	User      user.User
	ExpiresAt time.Time
	Renewed   bool
}

// Authenticate resolves a token to its user and extends the session
// (at most once per day) so active users stay logged in.
func (s *Service) Authenticate(ctx context.Context, token string) (Verified, error) {
	row, err := s.q.GetSession(ctx, hashToken(token))
	if errors.Is(err, sql.ErrNoRows) {
		return Verified{}, ErrNoSession
	}
	if err != nil {
		return Verified{}, fmt.Errorf("get session: %w", err)
	}
	expires, err := db.ParseTime(row.ExpiresAt)
	if err != nil {
		return Verified{}, err
	}
	now := s.now()
	if !now.Before(expires) {
		if err := s.q.DeleteSession(ctx, row.ID); err != nil {
			return Verified{}, fmt.Errorf("delete expired session: %w", err)
		}
		return Verified{}, ErrNoSession
	}
	renewed := false
	if expires.Sub(now) < SessionTTL-renewAfter {
		expires = now.Add(SessionTTL)
		if err := s.q.ExtendSession(ctx, sqlc.ExtendSessionParams{
			ExpiresAt: db.FormatTime(expires),
			ID:        row.ID,
		}); err != nil {
			return Verified{}, fmt.Errorf("extend session: %w", err)
		}
		renewed = true
	}
	u, err := s.users.ByID(ctx, row.UserID)
	if errors.Is(err, user.ErrNotFound) {
		return Verified{}, ErrNoSession
	}
	if err != nil {
		return Verified{}, err
	}
	return Verified{User: u, ExpiresAt: expires, Renewed: renewed}, nil
}

// Logout deletes the session. Unknown tokens are not an error.
func (s *Service) Logout(ctx context.Context, token string) error {
	if err := s.q.DeleteSession(ctx, hashToken(token)); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// DeleteExpired removes sessions past their expiry.
func (s *Service) DeleteExpired(ctx context.Context) error {
	if err := s.q.DeleteExpiredSessions(ctx, db.FormatTime(s.now())); err != nil {
		return fmt.Errorf("delete expired sessions: %w", err)
	}
	return nil
}

// SweepLoop calls DeleteExpired every d until ctx is cancelled, logging
// failures instead of returning them so a transient DB error never takes
// the sweep down permanently. Intended to run in its own goroutine.
func (s *Service) SweepLoop(ctx context.Context, d time.Duration, logger *slog.Logger) {
	ticker := time.NewTicker(d)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.DeleteExpired(ctx); err != nil {
				logger.Warn("sweep expired sessions", "err", err)
			}
		}
	}
}

// hashToken stores only a digest so a database leak does not leak sessions.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
