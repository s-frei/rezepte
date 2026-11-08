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

// MailWindow is when a mail that confirms an address went out and until when
// its link works.
type MailWindow struct {
	SentAt    time.Time `json:"sentAt" doc:"When the mail went out"`
	ExpiresAt time.Time `json:"expiresAt" doc:"When its link stops working"`
}

// MailStatus is what the email row shows about an unconfirmed address: the
// confirmation mailed to it, open or expired, and the open setup link mailed
// to it, whose redeeming confirms it too. Both are nil for an empty or
// confirmed address.
type MailStatus struct {
	Confirmation *MailWindow
	SetupLink    *MailWindow
}

// MailStatus is MailStatuses for one account.
func (s *Service) MailStatus(ctx context.Context, u user.User) (MailStatus, error) {
	all, err := s.MailStatuses(ctx, []user.User{u})
	return all[u.ID], err
}

// MailStatuses reads every account's mail windows in two queries; both
// tables hold at most one row per account. Both count only for the address
// exactly as stored, the way Confirm and the setup link's redeem
// (VerifyEmailIfMatches) compare it: one that would not confirm it is no
// mail for it.
func (s *Service) MailStatuses(ctx context.Context, users []user.User) (map[string]MailStatus, error) {
	confirmations, err := s.q.ListEmailConfirmations(ctx)
	if err != nil {
		return nil, fmt.Errorf("list confirmations: %w", err)
	}
	links, err := s.q.ListMailedSetupLinks(ctx, db.FormatTime(s.now()))
	if err != nil {
		return nil, fmt.Errorf("list mailed setup links: %w", err)
	}
	byID := make(map[string]user.User, len(users))
	for _, u := range users {
		if u.Email != "" && !u.EmailVerified {
			byID[u.ID] = u
		}
	}
	out := make(map[string]MailStatus, len(byID))
	for _, c := range confirmations {
		u, ok := byID[c.UserID]
		if !ok || c.Address != u.Email {
			continue
		}
		w, err := mailWindow(c.CreatedAt, c.ExpiresAt)
		if err != nil {
			return nil, err
		}
		st := out[u.ID]
		st.Confirmation = w
		out[u.ID] = st
	}
	for _, l := range links {
		u, ok := byID[l.UserID]
		if !ok || l.SentTo != u.Email {
			continue
		}
		w, err := mailWindow(l.CreatedAt, l.ExpiresAt)
		if err != nil {
			return nil, err
		}
		st := out[u.ID]
		st.SetupLink = w
		out[u.ID] = st
	}
	return out, nil
}

func mailWindow(sent, expires string) (*MailWindow, error) {
	s, err := db.ParseTime(sent)
	if err != nil {
		return nil, fmt.Errorf("parse sent time: %w", err)
	}
	e, err := db.ParseTime(expires)
	if err != nil {
		return nil, fmt.Errorf("parse expiry: %w", err)
	}
	return &MailWindow{SentAt: s, ExpiresAt: e}, nil
}
