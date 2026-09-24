package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/s-frei/rezepte/service/internal/db"
	"github.com/s-frei/rezepte/service/internal/db/sqlc"
	"github.com/s-frei/rezepte/service/internal/user"
)

// TokenPrefix marks a raw API token. Secret scanners key on it, and it lets
// a session cookie pasted as a bearer token be told apart from a real one.
const TokenPrefix = "rzp_"

// prefixLen is how much of the raw token is stored in clear for display:
// enough to match a row against an entry in a client configuration, four
// characters out of a 256-bit secret.
const prefixLen = 8

// touchAfter throttles last_used_at writes, mirroring renewAfter for
// sliding sessions: a busy integration must not write on every request.
const touchAfter = time.Hour

// Scopes an API token can carry. Write implies read, and the client sends
// both, so the middleware compares lists instead of applying an implication
// rule.
const (
	ScopeRecipesRead  = "recipes:read"
	ScopeRecipesWrite = "recipes:write"
	ScopeUsersRead    = "users:read"
	ScopeUsersWrite   = "users:write"
)

// knownScopes is the closed set Create accepts.
var knownScopes = []string{ScopeRecipesRead, ScopeRecipesWrite, ScopeUsersRead, ScopeUsersWrite}

// Errors returned by TokenService.
var (
	// ErrNoToken covers every reason a token does not authenticate: unknown,
	// expired, revoked, or owned by someone who is no longer an admin. The
	// caller must not be told which, so all four collapse into one error.
	ErrNoToken = errors.New("api token not found or expired")
	// ErrNotAPIToken means the presented value does not carry TokenPrefix at
	// all - most often a session cookie pasted as a bearer token by mistake.
	// It is kept distinct from ErrNoToken so the caller can be told what
	// actually went wrong instead of a bare "invalid or expired".
	ErrNotAPIToken = errors.New("not an api token")
	// ErrInvalidScope means a scope outside knownScopes was requested.
	ErrInvalidScope = errors.New("unknown scope")
)

// Token is an API token as it is listed: metadata only, never the secret.
type Token struct {
	ID            string
	Name          string
	Prefix        string
	Scopes        []string
	OwnerID       string
	OwnerUsername string
	CreatedAt     time.Time
	ExpiresAt     *time.Time
	LastUsedAt    *time.Time
}

// VerifiedToken is what a successful TokenService.Authenticate reports.
type VerifiedToken struct {
	User   user.User
	Scopes []string
}

// TokenService issues and verifies API tokens.
type TokenService struct {
	q     *sqlc.Queries
	users *user.Service
	now   func() time.Time
}

// NewTokenService returns a TokenService backed by conn.
func NewTokenService(conn *sql.DB, users *user.Service) *TokenService {
	return &TokenService{q: sqlc.New(conn), users: users, now: time.Now}
}

// SetClock overrides the time source. Intended for tests.
func (s *TokenService) SetClock(now func() time.Time) { s.now = now }

// Create issues a token for ownerID. The raw token is returned once and is
// unrecoverable afterwards: only its digest is stored. A nil expiresAt means
// the token never expires.
func (s *TokenService) Create(ctx context.Context, ownerID, name string, scopes []string, expiresAt *time.Time) (string, Token, error) {
	for _, scope := range scopes {
		if !slices.Contains(knownScopes, scope) {
			return "", Token{}, fmt.Errorf("%w: %s", ErrInvalidScope, scope)
		}
	}
	if len(scopes) == 0 {
		return "", Token{}, fmt.Errorf("%w: at least one scope is required", ErrInvalidScope)
	}
	owner, err := s.users.ByID(ctx, ownerID)
	if err != nil {
		return "", Token{}, err
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", Token{}, fmt.Errorf("generate api token: %w", err)
	}
	raw := TokenPrefix + base64.RawURLEncoding.EncodeToString(buf)
	now := s.now()
	tok := Token{
		ID:            uuid.Must(uuid.NewV7()).String(),
		Name:          name,
		Prefix:        raw[:prefixLen],
		Scopes:        scopes,
		OwnerID:       owner.ID,
		OwnerUsername: owner.Username,
		CreatedAt:     now,
		ExpiresAt:     expiresAt,
	}
	err = s.q.CreateAPIToken(ctx, sqlc.CreateAPITokenParams{
		ID:          tok.ID,
		UserID:      tok.OwnerID,
		Name:        tok.Name,
		TokenHash:   hashToken(raw),
		TokenPrefix: tok.Prefix,
		Scopes:      strings.Join(scopes, " "),
		ExpiresAt:   formatNullable(expiresAt),
		CreatedAt:   db.FormatTime(now),
	})
	if err != nil {
		return "", Token{}, fmt.Errorf("insert api token: %w", err)
	}
	return raw, tok, nil
}

// Authenticate resolves a raw token to its owner and scopes, and records the
// use at most once per touchAfter. An owner who is no longer an admin
// invalidates the token: a credential must not outlive the privilege that
// justified it.
func (s *TokenService) Authenticate(ctx context.Context, raw string) (VerifiedToken, error) {
	if !strings.HasPrefix(raw, TokenPrefix) {
		return VerifiedToken{}, ErrNotAPIToken
	}
	row, err := s.q.GetAPITokenByHash(ctx, hashToken(raw))
	if errors.Is(err, sql.ErrNoRows) {
		return VerifiedToken{}, ErrNoToken
	}
	if err != nil {
		return VerifiedToken{}, fmt.Errorf("get api token: %w", err)
	}
	now := s.now()
	expires, err := parseNullable(row.ExpiresAt)
	if err != nil {
		return VerifiedToken{}, err
	}
	if expires != nil && !now.Before(*expires) {
		return VerifiedToken{}, ErrNoToken
	}
	u, err := s.users.ByID(ctx, row.UserID)
	if errors.Is(err, user.ErrNotFound) {
		return VerifiedToken{}, ErrNoToken
	}
	if err != nil {
		return VerifiedToken{}, err
	}
	// IsAdmin, not a comparison against RoleAdmin: the instance owner is an
	// admin, and a bare comparison would kill every token they hold.
	if !u.Role.IsAdmin() {
		return VerifiedToken{}, ErrNoToken
	}
	used, err := parseNullable(row.LastUsedAt)
	if err != nil {
		return VerifiedToken{}, err
	}
	if used == nil || now.Sub(*used) >= touchAfter {
		if err := s.q.TouchAPIToken(ctx, sqlc.TouchAPITokenParams{
			LastUsedAt: ptr(db.FormatTime(now)),
			ID:         row.ID,
		}); err != nil {
			return VerifiedToken{}, fmt.Errorf("touch api token: %w", err)
		}
	}
	return VerifiedToken{User: u, Scopes: strings.Fields(row.Scopes)}, nil
}

// List returns every token in the instance, newest first, with its owner's
// name. Expired tokens are included: an admin has to be able to see why an
// integration stopped working.
func (s *TokenService) List(ctx context.Context) ([]Token, error) {
	rows, err := s.q.ListAPITokens(ctx)
	if err != nil {
		return nil, fmt.Errorf("list api tokens: %w", err)
	}
	out := make([]Token, 0, len(rows))
	for _, row := range rows {
		created, err := db.ParseTime(row.CreatedAt)
		if err != nil {
			return nil, err
		}
		expires, err := parseNullable(row.ExpiresAt)
		if err != nil {
			return nil, err
		}
		used, err := parseNullable(row.LastUsedAt)
		if err != nil {
			return nil, err
		}
		out = append(out, Token{
			ID:            row.ID,
			Name:          row.Name,
			Prefix:        row.TokenPrefix,
			Scopes:        strings.Fields(row.Scopes),
			OwnerID:       row.UserID,
			OwnerUsername: row.OwnerUsername,
			CreatedAt:     created,
			ExpiresAt:     expires,
			LastUsedAt:    used,
		})
	}
	return out, nil
}

// Delete revokes a token. An unknown id yields ErrNoToken so the caller can
// answer 404 rather than pretending the revoke happened.
func (s *TokenService) Delete(ctx context.Context, id string) error {
	n, err := s.q.DeleteAPIToken(ctx, id)
	if err != nil {
		return fmt.Errorf("delete api token %s: %w", id, err)
	}
	if n == 0 {
		return ErrNoToken
	}
	return nil
}

// bearerAuthFailureMessage turns a TokenService.Authenticate error into what
// a bearer request is told. Both bearer paths - Middleware and requireAuth -
// call this so a session cookie pasted as a bearer token gets the same
// comprehensible message wherever it lands, instead of a bare 401.
func bearerAuthFailureMessage(err error) string {
	if errors.Is(err, ErrNotAPIToken) {
		return fmt.Sprintf("this is not an API token; API tokens begin with %q", TokenPrefix)
	}
	return "api token invalid or expired"
}

// formatNullable renders an optional timestamp for a nullable TEXT column.
func formatNullable(t *time.Time) *string {
	if t == nil {
		return nil
	}
	return ptr(db.FormatTime(*t))
}

// parseNullable reads a nullable TEXT timestamp back.
func parseNullable(s *string) (*time.Time, error) {
	if s == nil {
		return nil, nil
	}
	t, err := db.ParseTime(*s)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func ptr[T any](v T) *T { return &v }
