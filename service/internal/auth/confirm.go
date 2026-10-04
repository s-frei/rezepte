package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/s-frei/rezepte/service/internal/db"
	"github.com/s-frei/rezepte/service/internal/db/sqlc"
	"github.com/s-frei/rezepte/service/internal/user"
)

// ConfirmationTTL is how long a confirmation link stays usable.
const ConfirmationTTL = 7 * 24 * time.Hour

// ErrNoConfirmation means the token is unknown, used, expired, or was mailed
// to an address the account no longer has. One error for all, so a probe
// learns nothing.
var ErrNoConfirmation = errors.New("confirmation link not found or expired")

// ConfirmPath is the SPA path a confirmation link opens; the token sits in
// the fragment for the reason SetupPath gives.
func ConfirmPath(token string) string { return "/confirm-email#" + token }

// IssueConfirmation replaces userID's open confirmation with one for address
// and returns its token, shown once in the mail and never stored.
func (s *Service) IssueConfirmation(ctx context.Context, userID, address string) (string, error) {
	token := newToken()
	now := s.now()
	if err := s.q.ReplaceEmailConfirmation(ctx, sqlc.ReplaceEmailConfirmationParams{
		ID: hashToken(token), UserID: userID, Address: address,
		ExpiresAt: db.FormatTime(now.Add(ConfirmationTTL)), CreatedAt: db.FormatTime(now),
	}); err != nil {
		return "", fmt.Errorf("store confirmation for %s: %w", userID, err)
	}
	return token, nil
}

// Confirm uses the token up and verifies the address it was mailed to, in one
// transaction, only while that is still the account's address; a token whose
// address changed is used up all the same and verifies nothing. It never
// signs anybody in: the link proves a mailbox, not who holds the browser.
func (s *Service) Confirm(ctx context.Context, token string) (string, error) {
	var address string
	var stale bool
	err := db.Tx(ctx, s.conn, func(q *sqlc.Queries) error {
		row, err := q.ConsumeEmailConfirmation(ctx, sqlc.ConsumeEmailConfirmationParams{
			ID: hashToken(token), ExpiresAt: db.FormatTime(s.now()),
		})
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNoConfirmation
		}
		if err != nil {
			return fmt.Errorf("consume confirmation: %w", err)
		}
		n, err := q.VerifyEmailIfMatches(ctx, sqlc.VerifyEmailIfMatchesParams{
			UpdatedAt: db.FormatTime(s.now()), ID: row.UserID, Email: row.Address,
		})
		if err != nil {
			return fmt.Errorf("verify email of %s: %w", row.UserID, err)
		}
		if n == 0 {
			// Commit the delete: a stale token is used up, not left to linger.
			stale = true
			return nil
		}
		address = row.Address
		return nil
	})
	if err == nil && stale {
		err = ErrNoConfirmation
	}
	return address, err
}

// ConfirmationPending reports whether u's current address has an open
// confirmation link: one that was mailed, is unexpired and unused. Only then
// may the profile say a confirmation mail went out.
func (s *Service) ConfirmationPending(ctx context.Context, u user.User) (bool, error) {
	if u.Email == "" || u.EmailVerified {
		return false, nil
	}
	open, err := s.q.HasOpenEmailConfirmation(ctx, sqlc.HasOpenEmailConfirmationParams{
		UserID: u.ID, Address: u.Email, ExpiresAt: db.FormatTime(s.now()),
	})
	if err != nil {
		return false, fmt.Errorf("look up confirmation of %s: %w", u.ID, err)
	}
	return open, nil
}
