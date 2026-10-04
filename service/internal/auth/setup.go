package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/s-frei/rezepte/service/internal/db"
	"github.com/s-frei/rezepte/service/internal/db/sqlc"
	"github.com/s-frei/rezepte/service/internal/user"
)

// ErrNoSetupLink means the token is unknown, used or expired. One error for
// all three, so a probe learns nothing.
var ErrNoSetupLink = errors.New("setup link not found or expired")

// errSetupLinkOpen means a reset was refused because an admin's setup link is
// open: that link already lets the person in.
var errSetupLinkOpen = errors.New("an open setup link stands")

// SetupLink is a freshly issued link. Token is shown once and never stored.
type SetupLink struct {
	Token     string
	ExpiresAt time.Time
	// Target is the account the link was issued for, as read under the rank
	// check, so a caller needs no second lookup.
	Target user.User
}

// SetupPath is the SPA path a setup link opens. The token sits in the
// fragment, so it never reaches a server or proxy log or a Referer header.
func SetupPath(token string) string { return "/welcome#" + token }

// SetupLinkTTL is how long a setup link stays usable.
const SetupLinkTTL = 7 * 24 * time.Hour

// ResetLinkTTL is how long a mailed password reset link stays usable: it
// lies in an inbox that may not be the person's alone.
const ResetLinkTTL = time.Hour

// Purpose says what a setup link may do. A reset link only sets a password.
type Purpose string

// The values of Purpose, as stored in setup_links.purpose.
const (
	PurposeSetup Purpose = "setup"
	PurposeReset Purpose = "reset"
)

// OpenLink is an open setup link as PeekSetupLink reports it.
type OpenLink struct {
	User    user.User
	Purpose Purpose
}

// newToken is a fresh 256-bit token in URL-safe base64.
func newToken() string {
	raw := make([]byte, 32)
	_, _ = rand.Read(raw) // never fails since Go 1.24
	return base64.RawURLEncoding.EncodeToString(raw)
}

// IssueSetupLink replaces any open link of userID with a new one. It is the
// same act as a password reset and follows the same rank rule.
func (s *Service) IssueSetupLink(ctx context.Context, actor user.User, userID string) (SetupLink, error) {
	target, err := s.users.ByID(ctx, userID)
	if err != nil {
		return SetupLink{}, err
	}
	if err := user.CanManage(actor.Role, target.Role); err != nil {
		return SetupLink{}, err
	}
	return s.issueLink(ctx, target, actor.ID, PurposeSetup, SetupLinkTTL)
}

// IssueResetLink replaces an older reset link of userID, or an expired one,
// with a new reset link; an open setup link stays (errSetupLinkOpen). Nobody
// acts: the person asked for it without a session, and ForgotPassword checked
// they may have one, so the account itself is recorded as its creator. The
// owner is refused here too, not only by the caller.
func (s *Service) IssueResetLink(ctx context.Context, userID string) (SetupLink, error) {
	target, err := s.users.ByID(ctx, userID)
	if err != nil {
		return SetupLink{}, err
	}
	if target.Role.IsSuperadmin() {
		return SetupLink{}, user.ErrSuperadminProtected
	}
	return s.issueLink(ctx, target, target.ID, PurposeReset, ResetLinkTTL)
}

func (s *Service) issueLink(ctx context.Context, target user.User, createdBy string, purpose Purpose, ttl time.Duration) (SetupLink, error) {
	token := newToken()
	now := s.now()
	expires := now.Add(ttl)
	params := sqlc.ReplaceSetupLinkParams{
		ID: hashToken(token), UserID: target.ID, CreatedBy: createdBy,
		ExpiresAt: db.FormatTime(expires), CreatedAt: db.FormatTime(now), SentTo: "",
		Purpose: string(purpose),
	}
	n, err := s.q.ReplaceSetupLink(ctx, params)
	if err != nil {
		return SetupLink{}, fmt.Errorf("store setup link for %s: %w", target.ID, err)
	}
	if n == 0 {
		return SetupLink{}, errSetupLinkOpen
	}
	return SetupLink{Token: token, ExpiresAt: expires, Target: target}, nil
}

// SetupLinkHash is the digest a setup link is stored and looked up under.
// A caller that must carry a link across requests (the OIDC flow cookie)
// keeps this instead of the token.
func SetupLinkHash(token string) string { return hashToken(token) }

// PeekSetupLink returns the account an open link belongs to and what the
// link may do, without using it.
func (s *Service) PeekSetupLink(ctx context.Context, token string) (OpenLink, error) {
	return s.peek(ctx, hashToken(token))
}

// PeekSetupLinkByHash is the OIDC flow's peek, for a link known by
// SetupLinkHash. A reset link answers ErrNoSetupLink: it sets a password and
// nothing else, so a reset can never attach an identity.
func (s *Service) PeekSetupLinkByHash(ctx context.Context, hash string) (user.User, error) {
	l, err := s.peek(ctx, hash)
	if err != nil {
		return user.User{}, err
	}
	if l.Purpose != PurposeSetup {
		return user.User{}, ErrNoSetupLink
	}
	return l.User, nil
}

func (s *Service) peek(ctx context.Context, hash string) (OpenLink, error) {
	row, err := s.q.GetOpenSetupLink(ctx, sqlc.GetOpenSetupLinkParams{
		ID: hash, ExpiresAt: db.FormatTime(s.now()),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return OpenLink{}, ErrNoSetupLink
	}
	if err != nil {
		return OpenLink{}, fmt.Errorf("get setup link: %w", err)
	}
	u, err := s.users.ByID(ctx, row.UserID)
	if err != nil {
		return OpenLink{}, err
	}
	return OpenLink{User: u, Purpose: Purpose(row.Purpose)}, nil
}

// ConsumeSetupLink uses the link up and returns its account. Of two callers
// with the same token, exactly one gets the id.
func (s *Service) ConsumeSetupLink(ctx context.Context, token string) (string, error) {
	return s.ConsumeSetupLinkByHash(ctx, hashToken(token))
}

// ConsumeSetupLinkByHash is ConsumeSetupLink for a link known by
// SetupLinkHash.
func (s *Service) ConsumeSetupLinkByHash(ctx context.Context, hash string) (string, error) {
	var id string
	err := db.Tx(ctx, s.conn, func(q *sqlc.Queries) error {
		var err error
		id, err = s.consumeWith(ctx, q, hash)
		return err
	})
	return id, err
}

// consumeWith runs the delete-returning consume of the link stored under
// hash against q and maps sql.ErrNoRows to ErrNoSetupLink. A link that was
// mailed verifies the account's address - only while that is still the
// address it went to - in the same transaction, so whoever proved they read
// that mailbox is the one the mark is for.
func (s *Service) consumeWith(ctx context.Context, q *sqlc.Queries, hash string) (string, error) {
	row, err := q.ConsumeSetupLink(ctx, sqlc.ConsumeSetupLinkParams{
		ID: hash, ExpiresAt: db.FormatTime(s.now()),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNoSetupLink
	}
	if err != nil {
		return "", fmt.Errorf("consume setup link: %w", err)
	}
	if row.SentTo != "" {
		if _, err := q.VerifyEmailIfMatches(ctx, sqlc.VerifyEmailIfMatchesParams{
			UpdatedAt: db.FormatTime(s.now()), ID: row.UserID, Email: row.SentTo,
		}); err != nil {
			return "", fmt.Errorf("verify email of %s: %w", row.UserID, err)
		}
	}
	return row.UserID, nil
}

// MarkSetupLinkSent records that the link under token was mailed to to, so
// redeeming it verifies that address. Called only after the mail went out.
// It is keyed by the link, not the account: if a newer link replaced this one
// while the mail was being sent, the newer link was never mailed and stays
// unmarked (this call then matches nothing).
func (s *Service) MarkSetupLinkSent(ctx context.Context, token, to string) error {
	if err := s.q.MarkSetupLinkSent(ctx, sqlc.MarkSetupLinkSentParams{SentTo: to, ID: hashToken(token)}); err != nil {
		return fmt.Errorf("mark setup link sent: %w", err)
	}
	return nil
}

// RedeemWithPassword sets the account's password through its setup link,
// ends every session it had and opens a new one. The hash is computed before
// the link is used, so a full argon2 queue (user.ErrBusy) leaves the link
// open for a retry. A reset link redeems the same way.
func (s *Service) RedeemWithPassword(ctx context.Context, token, password string) (Session, error) {
	if _, err := s.PeekSetupLink(ctx, token); err != nil {
		return Session{}, err
	}
	hash, err := user.HashPassword(ctx, password)
	if err != nil {
		return Session{}, err
	}
	var userID string
	err = db.Tx(ctx, s.conn, func(q *sqlc.Queries) error {
		id, err := s.consumeWith(ctx, q, hashToken(token))
		if err != nil {
			return err
		}
		userID = id
		if _, err := q.UpdateUserPasswordHash(ctx, sqlc.UpdateUserPasswordHashParams{
			PasswordHash: hash, UpdatedAt: db.FormatTime(s.now()), ID: id,
		}); err != nil {
			return fmt.Errorf("set password of %s: %w", id, err)
		}
		if err := q.DeleteUserSessionsExcept(ctx, sqlc.DeleteUserSessionsExceptParams{UserID: id, ID: ""}); err != nil {
			return fmt.Errorf("end sessions of %s: %w", id, err)
		}
		return nil
	})
	if err != nil {
		return Session{}, err
	}
	u, err := s.users.ByID(ctx, userID)
	if err != nil {
		return Session{}, err
	}
	return s.StartSession(ctx, u)
}

// RevokeSetupLink closes userID's open link, if there is one. It follows the
// rank rule issuing does: whoever may hand out a link may take it back.
func (s *Service) RevokeSetupLink(ctx context.Context, actor user.User, userID string) error {
	target, err := s.users.ByID(ctx, userID)
	if err != nil {
		return err
	}
	if err := user.CanManage(actor.Role, target.Role); err != nil {
		return err
	}
	if err := s.q.DeleteSetupLinkOfUser(ctx, userID); err != nil {
		return fmt.Errorf("revoke setup link of %s: %w", userID, err)
	}
	return nil
}

// OpenSetupLinks maps every account with an open setup link to the link's
// expiry, for the admin's people list. Reset links are left out: a member
// asking for one is not an open invitation.
func (s *Service) OpenSetupLinks(ctx context.Context) (map[string]time.Time, error) {
	rows, err := s.q.ListOpenSetupLinks(ctx, db.FormatTime(s.now()))
	if err != nil {
		return nil, fmt.Errorf("list open setup links: %w", err)
	}
	open := make(map[string]time.Time, len(rows))
	for _, r := range rows {
		t, err := db.ParseTime(r.ExpiresAt)
		if err != nil {
			return nil, fmt.Errorf("parse setup link expiry for %s: %w", r.UserID, err)
		}
		open[r.UserID] = t
	}
	return open, nil
}
