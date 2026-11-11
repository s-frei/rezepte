// Package auth issues and validates login sessions and exposes the auth API.
package auth

import (
	"context"
	"crypto/sha256"
	"database/sql"
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

// ThrottledError means the username has used its free login attempts and
// is locked for RetryAfter. Login returns it before the password is checked.
type ThrottledError struct {
	RetryAfter time.Duration
}

func (e *ThrottledError) Error() string {
	return fmt.Sprintf("too many login attempts, retry in %v", e.RetryAfter)
}

// Session is what the client receives after login.
type Session struct {
	Token     string
	ExpiresAt time.Time
	User      user.User
}

const (
	// resetMailEvery and confirmMailEvery space out mails one account can
	// trigger: a forgotten-password request, and "Send again".
	resetMailEvery   = 5 * time.Minute
	confirmMailEvery = time.Minute
)

// Service manages sessions, setup links and address confirmations.
type Service struct {
	conn            *sql.DB
	q               *sqlc.Queries
	users           *user.Service
	throttle        *throttle
	resetCooldown   *cooldown
	confirmCooldown *cooldown
	mailer          Mailer
	provider        func(context.Context) Provider
	notifier        Notifier
	now             func() time.Time
}

// NewService returns a Service backed by conn.
func NewService(conn *sql.DB, users *user.Service) *Service {
	return &Service{
		conn: conn, q: sqlc.New(conn), users: users, throttle: newThrottle(throttleCapacity),
		resetCooldown: newCooldown(resetMailEvery), confirmCooldown: newCooldown(confirmMailEvery),
		now: time.Now,
	}
}

// SetClock overrides the time source. Intended for tests.
func (s *Service) SetClock(now func() time.Time) { s.now = now }

// Login verifies the credentials and creates a session. A username that
// has used its free attempts yields a ThrottledError without its password
// being checked, so a locked attempt costs neither a query nor an argon2
// evaluation.
func (s *Service) Login(ctx context.Context, username, password string) (Session, error) {
	if wait, ok := s.throttle.begin(username, s.now()); !ok {
		return Session{}, &ThrottledError{RetryAfter: wait}
	}
	u, err := s.users.Authenticate(ctx, username, password)
	if err != nil {
		return Session{}, err
	}
	s.throttle.succeed(username)
	return s.StartSession(ctx, u)
}

// StartSession opens a session for u. Login calls it after the password
// check; a setup link and an identity provider call it after theirs.
func (s *Service) StartSession(ctx context.Context, u user.User) (Session, error) {
	token := newToken()
	now := s.now()
	expires := now.Add(SessionTTL)
	if err := s.q.CreateSession(ctx, sqlc.CreateSessionParams{
		ID: hashToken(token), UserID: u.ID,
		ExpiresAt: db.FormatTime(expires), CreatedAt: db.FormatTime(now),
	}); err != nil {
		return Session{}, fmt.Errorf("insert session: %w", err)
	}
	if err := touchActive(ctx, s.q, u.ID, now); err != nil {
		return Session{}, err
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
	SessionID string // the sessions row's id, the token's digest
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
		return Verified{}, fmt.Errorf("parse session expiry: %w", err)
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
	// Activity is recorded at most once per touchAfter, like a token's use,
	// so someone who opens the app daily reads "active today".
	if u.LastActiveAt == nil || now.Sub(*u.LastActiveAt) >= touchAfter {
		if err := touchActive(ctx, s.q, u.ID, now); err != nil {
			return Verified{}, err
		}
	}
	return Verified{User: u, SessionID: row.ID, ExpiresAt: expires, Renewed: renewed}, nil
}

// Peeked is what Peek reads from a session row. UserID is empty and Valid
// false when there is no row; an expired row keeps its UserID and LastSeen.
type Peeked struct {
	UserID string
	// LastSeen is the session's last renewal or its start: expires_at minus
	// SessionTTL, so it lags real use by up to renewAfter.
	LastSeen time.Time
	Valid    bool
}

// Peek looks a session up without using it: it never renews it, never
// records activity and never deletes an expired row. The accounts on a
// device are checked this way, so listing them keeps nobody signed in and
// makes nobody look active.
func (s *Service) Peek(ctx context.Context, token string) (Peeked, error) {
	row, err := s.q.GetSession(ctx, hashToken(token))
	if errors.Is(err, sql.ErrNoRows) {
		return Peeked{}, nil
	}
	if err != nil {
		return Peeked{}, fmt.Errorf("get session: %w", err)
	}
	expires, err := db.ParseTime(row.ExpiresAt)
	if err != nil {
		return Peeked{}, fmt.Errorf("parse session expiry: %w", err)
	}
	return Peeked{UserID: row.UserID, LastSeen: expires.Add(-SessionTTL), Valid: s.now().Before(expires)}, nil
}

// Logout deletes the session. Unknown tokens are not an error.
func (s *Service) Logout(ctx context.Context, token string) error {
	if err := s.q.DeleteSession(ctx, hashToken(token)); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// DeleteUserSessionsExcept ends every session of userID except the one
// whose raw cookie token is keepToken, so a password change logs out
// other devices while the acting browser stays signed in. An empty
// keepToken ends all of the user's sessions (admin password reset).
func (s *Service) DeleteUserSessionsExcept(ctx context.Context, userID, keepToken string) error {
	keep := ""
	if keepToken != "" {
		keep = hashToken(keepToken)
	}
	err := s.q.DeleteUserSessionsExcept(ctx, sqlc.DeleteUserSessionsExceptParams{UserID: userID, ID: keep})
	if err != nil {
		return fmt.Errorf("delete sessions of %s: %w", userID, err)
	}
	return nil
}

// DeleteExpired removes sessions and setup links past their expiry. Address
// confirmations stay: one row per account, kept so the email row can say
// the link ran out; Confirm refuses an expired one anyway.
func (s *Service) DeleteExpired(ctx context.Context) error {
	if err := s.q.DeleteExpiredSessions(ctx, db.FormatTime(s.now())); err != nil {
		return fmt.Errorf("delete expired sessions: %w", err)
	}
	if err := s.q.DeleteExpiredSetupLinks(ctx, db.FormatTime(s.now())); err != nil {
		return fmt.Errorf("delete expired setup links: %w", err)
	}
	return nil
}

// SweepLoop calls DeleteExpired every d until ctx is canceled, logging
// failures instead of returning them so a transient DB error never takes
// the sweep down permanently. The same tick drops login throttle entries
// nobody has tried for a day, and the mail cooldowns' finished periods.
// Intended to run in its own goroutine.
func (s *Service) SweepLoop(ctx context.Context, d time.Duration, logger *slog.Logger) {
	ticker := time.NewTicker(d)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.throttle.sweep(s.now())
			s.resetCooldown.sweep(s.now())
			s.confirmCooldown.sweep(s.now())
			if err := s.DeleteExpired(ctx); err != nil {
				logger.Warn("auth: sweep expired sessions failed", "err", err)
			}
		}
	}
}

// touchActive records that userID was active at now: on every new session,
// and on a session's or an API token's use at most once per touchAfter.
// Activity is recorded where it is already detected and throttled, never per
// request, so it adds at most one write per person and hour and lags real
// activity by up to an hour.
func touchActive(ctx context.Context, q *sqlc.Queries, userID string, now time.Time) error {
	if err := q.TouchUserActive(ctx, sqlc.TouchUserActiveParams{
		LastActiveAt: new(db.FormatTime(now)), ID: userID,
	}); err != nil {
		return fmt.Errorf("touch user %s active: %w", userID, err)
	}
	return nil
}

// hashToken stores only a digest so a database leak does not leak sessions.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
