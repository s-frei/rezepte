package user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/s-frei/rezepte/service/internal/db"
	"github.com/s-frei/rezepte/service/internal/db/sqlc"
)

// ErrIdentityTaken means the provider identity already belongs to another
// account, or this account already has an identity at that provider.
var ErrIdentityTaken = errors.New("identity already linked")

// ErrLastCredential means removing the identity would leave the account with
// no way to sign in.
var ErrLastCredential = errors.New("account has no password; set one before disconnecting")

// ByIdentity returns the account linked to (issuer, subject).
func (s *Service) ByIdentity(ctx context.Context, issuer, subject string) (User, error) {
	row, err := s.q.GetUserByIdentity(ctx, sqlc.GetUserByIdentityParams{Issuer: issuer, Subject: subject})
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("get user by identity: %w", err)
	}
	return fromRow(row)
}

// LinkIdentity links (issuer, subject) to userID. Linking the identity the
// account already has is a no-op.
func (s *Service) LinkIdentity(ctx context.Context, userID, issuer, subject string) error {
	u, err := s.ByIdentity(ctx, issuer, subject)
	switch {
	case err == nil && u.ID == userID:
		return nil
	case err == nil:
		return ErrIdentityTaken
	case !errors.Is(err, ErrNotFound):
		return err
	}
	err = s.q.InsertIdentity(ctx, sqlc.InsertIdentityParams{
		UserID: userID, Issuer: issuer, Subject: subject, CreatedAt: db.FormatTime(s.now()),
	})
	if isUniqueViolation(err) || isPrimaryKeyViolation(err) {
		return ErrIdentityTaken
	}
	if err != nil {
		return fmt.Errorf("link identity to %s: %w", userID, err)
	}
	return nil
}

// IdentityLinkedAt reports when userID linked its identity at issuer.
func (s *Service) IdentityLinkedAt(ctx context.Context, userID, issuer string) (time.Time, error) {
	row, err := s.q.GetIdentityOfUser(ctx, sqlc.GetIdentityOfUserParams{UserID: userID, Issuer: issuer})
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, ErrNotFound
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("get identity of %s: %w", userID, err)
	}
	return db.ParseTime(row.CreatedAt)
}

// UnlinkIdentity removes userID's identity at issuer, unless it is the
// account's only way in.
func (s *Service) UnlinkIdentity(ctx context.Context, userID, issuer string) error {
	u, err := s.ByID(ctx, userID)
	if err != nil {
		return err
	}
	if !u.HasPassword {
		return ErrLastCredential
	}
	n, err := s.q.DeleteIdentityOfUser(ctx, sqlc.DeleteIdentityOfUserParams{UserID: userID, Issuer: issuer})
	if err != nil {
		return fmt.Errorf("unlink identity of %s: %w", userID, err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// LinkedUserIDs is the set of accounts with an identity at any provider, for
// the admin's people list.
func (s *Service) LinkedUserIDs(ctx context.Context) (map[string]bool, error) {
	ids, err := s.q.ListLinkedUserIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("list linked users: %w", err)
	}
	linked := make(map[string]bool, len(ids))
	for _, id := range ids {
		linked[id] = true
	}
	return linked, nil
}
