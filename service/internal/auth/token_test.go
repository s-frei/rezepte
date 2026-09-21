package auth_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/db/dbtest"
	"github.com/s-frei/rezepte/service/internal/user"
)

// newTokenService seeds an admin "sam" and returns the token service, the
// user service and sam's id.
func newTokenService(t *testing.T) (*auth.TokenService, *user.Service, string) {
	t.Helper()
	conn := dbtest.Open(t)
	users := user.NewService(conn)
	sam, err := users.Create(context.Background(), user.CreateParams{Username: "sam", Password: "pw", Role: user.RoleAdmin})
	if err != nil {
		t.Fatal(err)
	}
	return auth.NewTokenService(conn, users), users, sam.ID
}

func TestCreateAndAuthenticate(t *testing.T) {
	ctx := context.Background()
	svc, _, ownerID := newTokenService(t)

	raw, tok, err := svc.Create(ctx, ownerID, "mcp", []string{auth.ScopeRecipesRead}, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !strings.HasPrefix(raw, auth.TokenPrefix) || len(raw) != len(auth.TokenPrefix)+43 {
		t.Fatalf("raw token = %q, want %s + 43 chars", raw, auth.TokenPrefix)
	}
	if tok.Prefix != raw[:8] {
		t.Fatalf("Prefix = %q, want %q", tok.Prefix, raw[:8])
	}
	if tok.OwnerName != "sam" || tok.Name != "mcp" {
		t.Fatalf("token = %+v", tok)
	}
	if tok.ExpiresAt != nil {
		t.Fatalf("ExpiresAt = %v, want nil for a token without expiry", tok.ExpiresAt)
	}

	v, err := svc.Authenticate(ctx, raw)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if v.User.ID != ownerID {
		t.Fatalf("user = %+v, want %s", v.User, ownerID)
	}
	if len(v.Scopes) != 1 || v.Scopes[0] != auth.ScopeRecipesRead {
		t.Fatalf("scopes = %v", v.Scopes)
	}

	if _, err := svc.Authenticate(ctx, auth.TokenPrefix+"bogus"); !errors.Is(err, auth.ErrNoToken) {
		t.Fatalf("bogus token: err = %v, want ErrNoToken", err)
	}
}

// TestAuthenticateRejectsAValueWithoutThePrefix covers the case a session
// cookie gets pasted as a bearer token: it must be told apart from an
// unknown-but-shaped-like-a-token value, so the caller can give a
// comprehensible error instead of a bare "invalid or expired".
func TestAuthenticateRejectsAValueWithoutThePrefix(t *testing.T) {
	svc, _, _ := newTokenService(t)
	if _, err := svc.Authenticate(context.Background(), "not-a-token-at-all"); !errors.Is(err, auth.ErrNotAPIToken) {
		t.Fatalf("err = %v, want ErrNotAPIToken", err)
	}
}

func TestCreateRejectsUnknownScope(t *testing.T) {
	svc, _, ownerID := newTokenService(t)
	_, _, err := svc.Create(context.Background(), ownerID, "bad", []string{"recipes:delete"}, nil)
	if !errors.Is(err, auth.ErrInvalidScope) {
		t.Fatalf("err = %v, want ErrInvalidScope", err)
	}
}

func TestExpiredTokenIsRejectedButKept(t *testing.T) {
	ctx := context.Background()
	svc, _, ownerID := newTokenService(t)

	past := time.Now().Add(-time.Minute)
	raw, _, err := svc.Create(ctx, ownerID, "stale", []string{auth.ScopeRecipesRead}, &past)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Authenticate(ctx, raw); !errors.Is(err, auth.ErrNoToken) {
		t.Fatalf("expired token: err = %v, want ErrNoToken", err)
	}
	// The row survives so an admin can see why the integration stopped.
	list, err := svc.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("List returned %d tokens, want the expired one to remain", len(list))
	}
}

func TestTokenDiesWithTheAdminRole(t *testing.T) {
	ctx := context.Background()
	svc, users, ownerID := newTokenService(t)
	// Demoting an admin is the instance owner's act alone, so the demotion
	// below needs one to do it.
	boss, err := users.Create(ctx, user.CreateParams{Username: "boss", Password: "pw", Role: user.RoleSuperadmin})
	if err != nil {
		t.Fatal(err)
	}
	raw, _, err := svc.Create(ctx, ownerID, "mcp", []string{auth.ScopeRecipesRead}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := users.SetRole(ctx, boss, ownerID, user.RoleUser); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Authenticate(ctx, raw); !errors.Is(err, auth.ErrNoToken) {
		t.Fatalf("after demotion: err = %v, want ErrNoToken", err)
	}
}

func TestLastUsedIsThrottledToOneHour(t *testing.T) {
	ctx := context.Background()
	svc, _, ownerID := newTokenService(t)

	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	svc.SetClock(func() time.Time { return now })
	raw, _, err := svc.Create(ctx, ownerID, "mcp", []string{auth.ScopeRecipesRead}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := svc.Authenticate(ctx, raw); err != nil {
		t.Fatal(err)
	}
	first := lastUsed(t, svc)
	if first == nil || !first.Equal(now) {
		t.Fatalf("lastUsedAt = %v, want %v on first use", first, now)
	}

	now = now.Add(30 * time.Minute)
	if _, err := svc.Authenticate(ctx, raw); err != nil {
		t.Fatal(err)
	}
	if got := lastUsed(t, svc); got == nil || !got.Equal(*first) {
		t.Fatalf("lastUsedAt = %v after 30m, want it unchanged at %v", got, first)
	}

	now = now.Add(31 * time.Minute)
	if _, err := svc.Authenticate(ctx, raw); err != nil {
		t.Fatal(err)
	}
	if got := lastUsed(t, svc); got == nil || !got.Equal(now) {
		t.Fatalf("lastUsedAt = %v after 61m, want %v", got, now)
	}
}

func lastUsed(t *testing.T, svc *auth.TokenService) *time.Time {
	t.Helper()
	list, err := svc.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("List returned %d tokens, want 1", len(list))
	}
	return list[0].LastUsedAt
}

func TestDeleteRevokes(t *testing.T) {
	ctx := context.Background()
	svc, _, ownerID := newTokenService(t)
	raw, tok, err := svc.Create(ctx, ownerID, "mcp", []string{auth.ScopeRecipesRead}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(ctx, tok.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Authenticate(ctx, raw); !errors.Is(err, auth.ErrNoToken) {
		t.Fatalf("after delete: err = %v, want ErrNoToken", err)
	}
	if err := svc.Delete(ctx, tok.ID); !errors.Is(err, auth.ErrNoToken) {
		t.Fatalf("second delete: err = %v, want ErrNoToken", err)
	}
}
