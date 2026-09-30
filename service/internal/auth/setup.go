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

// SetupLinkTTL is how long a setup link stays usable.
const SetupLinkTTL = 7 * 24 * time.Hour

// ErrNoSetupLink means the token is unknown, used or expired. One error for
// all three, so a probe learns nothing.
var ErrNoSetupLink = errors.New("setup link not found or expired")

// SetupLink is a freshly issued link. Token is shown once and never stored.
type SetupLink struct {
	Token     string
	ExpiresAt time.Time
}

// SetupPath is the SPA path a setup link opens. The token sits in the
// fragment, so it never reaches a server or proxy log or a Referer header.
func SetupPath(token string) string { return "/welcome#" + token }

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
	raw := make([]byte, 32)
	_, _ = rand.Read(raw)
	token := base64.RawURLEncoding.EncodeToString(raw)
	now := s.now()
	expires := now.Add(SetupLinkTTL)
	if err := s.q.ReplaceSetupLink(ctx, sqlc.ReplaceSetupLinkParams{
		ID: hashToken(token), UserID: userID, CreatedBy: actor.ID,
		ExpiresAt: db.FormatTime(expires), CreatedAt: db.FormatTime(now),
	}); err != nil {
		return SetupLink{}, fmt.Errorf("store setup link for %s: %w", userID, err)
	}
	return SetupLink{Token: token, ExpiresAt: expires}, nil
}

// SetupLinkHash is the digest a setup link is stored and looked up under.
// A caller that must carry a link across requests (the OIDC flow cookie)
// keeps this instead of the token.
func SetupLinkHash(token string) string { return hashToken(token) }

// PeekSetupLink returns the account an open link belongs to, without using it.
func (s *Service) PeekSetupLink(ctx context.Context, token string) (user.User, error) {
	return s.PeekSetupLinkByHash(ctx, hashToken(token))
}

// PeekSetupLinkByHash is PeekSetupLink for a link known by SetupLinkHash.
func (s *Service) PeekSetupLinkByHash(ctx context.Context, hash string) (user.User, error) {
	row, err := s.q.GetOpenSetupLink(ctx, sqlc.GetOpenSetupLinkParams{
		ID: hash, ExpiresAt: db.FormatTime(s.now()),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return user.User{}, ErrNoSetupLink
	}
	if err != nil {
		return user.User{}, fmt.Errorf("get setup link: %w", err)
	}
	return s.users.ByID(ctx, row.UserID)
}

// ConsumeSetupLink uses the link up and returns its account. Of two callers
// with the same token, exactly one gets the id.
func (s *Service) ConsumeSetupLink(ctx context.Context, token string) (string, error) {
	return s.consumeWith(ctx, s.q, hashToken(token))
}

// ConsumeSetupLinkByHash is ConsumeSetupLink for a link known by
// SetupLinkHash.
func (s *Service) ConsumeSetupLinkByHash(ctx context.Context, hash string) (string, error) {
	return s.consumeWith(ctx, s.q, hash)
}

// consumeWith runs the delete-returning consume of the link stored under
// hash against q - s.q outside a transaction, or a transaction's own
// *sqlc.Queries - and maps sql.ErrNoRows to ErrNoSetupLink. The one place
// both consumes and RedeemWithPassword's transactional consume do this.
func (s *Service) consumeWith(ctx context.Context, q *sqlc.Queries, hash string) (string, error) {
	id, err := q.ConsumeSetupLink(ctx, sqlc.ConsumeSetupLinkParams{
		ID: hash, ExpiresAt: db.FormatTime(s.now()),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNoSetupLink
	}
	if err != nil {
		return "", fmt.Errorf("consume setup link: %w", err)
	}
	return id, nil
}

// RedeemWithPassword sets the account's password through its setup link,
// ends every session it had and opens a new one. The hash is computed before
// the link is used, so a full argon2 queue (user.ErrBusy) leaves the link
// open for a retry.
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

// OpenSetupLinks maps every account with an open link to the link's expiry,
// for the admin's people list.
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
