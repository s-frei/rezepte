package auth_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/db/dbtest"
	"github.com/s-frei/rezepte/service/internal/user"
)

type setupFixture struct {
	users    *user.Service
	sessions *auth.Service
	owner    user.User
	admin    user.User
	member   user.User
}

func newSetupFixture(t *testing.T) setupFixture {
	t.Helper()
	ctx := context.Background()
	conn := dbtest.Open(t)
	users := user.NewService(conn, "")
	mk := func(name string, role user.Role, pw string) user.User {
		u, err := users.Create(ctx, user.CreateParams{Username: name, Password: pw, Role: role})
		if err != nil {
			t.Fatal(err)
		}
		return u
	}
	return setupFixture{
		users: users, sessions: auth.NewService(conn, users),
		owner: mk("owner", user.RoleSuperadmin, "pw"), admin: mk("sam", user.RoleAdmin, "pw"),
		member: mk("anna", user.RoleUser, ""),
	}
}

func TestRedeemSetupLinkSetsPasswordAndSignsIn(t *testing.T) {
	ctx := context.Background()
	f := newSetupFixture(t)
	link, err := f.sessions.IssueSetupLink(ctx, f.admin, f.member.ID)
	if err != nil {
		t.Fatal(err)
	}
	sess, err := f.sessions.RedeemWithPassword(ctx, link.Token, "anna1234")
	if err != nil {
		t.Fatal(err)
	}
	if sess.User.ID != f.member.ID || !sess.User.HasPassword {
		t.Fatalf("session for %+v", sess.User)
	}
	if _, err := f.users.Authenticate(ctx, "anna", "anna1234"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.sessions.PeekSetupLink(ctx, link.Token); !errors.Is(err, auth.ErrNoSetupLink) {
		t.Fatalf("link still open: %v", err)
	}
}

func TestRedeemSetupLinkOnlyOnce(t *testing.T) {
	ctx := context.Background()
	f := newSetupFixture(t)
	link, _ := f.sessions.IssueSetupLink(ctx, f.admin, f.member.ID)
	var wg sync.WaitGroup
	results := make([]error, 2)
	for i := range results {
		wg.Go(func() { _, results[i] = f.sessions.RedeemWithPassword(ctx, link.Token, "anna1234") })
	}
	wg.Wait()
	ok := 0
	for _, err := range results {
		if err == nil {
			ok++
		} else if !errors.Is(err, auth.ErrNoSetupLink) {
			t.Fatalf("unexpected: %v", err)
		}
	}
	if ok != 1 {
		t.Fatalf("%d redemptions succeeded, want 1", ok)
	}
}

func TestSetupLinkExpires(t *testing.T) {
	ctx := context.Background()
	f := newSetupFixture(t)
	now := time.Now()
	f.sessions.SetClock(func() time.Time { return now })
	link, _ := f.sessions.IssueSetupLink(ctx, f.admin, f.member.ID)
	f.sessions.SetClock(func() time.Time { return now.Add(auth.SetupLinkTTL + time.Second) })
	if _, err := f.sessions.RedeemWithPassword(ctx, link.Token, "anna1234"); !errors.Is(err, auth.ErrNoSetupLink) {
		t.Fatalf("got %v", err)
	}
}

func TestNewSetupLinkReplacesTheOld(t *testing.T) {
	ctx := context.Background()
	f := newSetupFixture(t)
	first, _ := f.sessions.IssueSetupLink(ctx, f.admin, f.member.ID)
	if _, err := f.sessions.IssueSetupLink(ctx, f.admin, f.member.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.sessions.PeekSetupLink(ctx, first.Token); !errors.Is(err, auth.ErrNoSetupLink) {
		t.Fatalf("old link still open: %v", err)
	}
}

func TestSetupLinkFollowsRank(t *testing.T) {
	ctx := context.Background()
	f := newSetupFixture(t)
	if _, err := f.sessions.IssueSetupLink(ctx, f.admin, f.owner.ID); !errors.Is(err, user.ErrSuperadminProtected) {
		t.Fatalf("admin → owner: %v", err)
	}
	if _, err := f.sessions.IssueSetupLink(ctx, f.admin, f.admin.ID); !errors.Is(err, user.ErrSuperadminRequired) {
		t.Fatalf("admin → self: %v", err)
	}
	if _, err := f.sessions.IssueSetupLink(ctx, f.owner, f.owner.ID); !errors.Is(err, user.ErrSuperadminProtected) {
		t.Fatalf("owner → owner: %v", err)
	}
}

func TestRedeemSetupLinkEndsOtherSessions(t *testing.T) {
	ctx := context.Background()
	f := newSetupFixture(t)
	old, _ := f.sessions.StartSession(ctx, f.member)
	link, _ := f.sessions.IssueSetupLink(ctx, f.admin, f.member.ID)
	if _, err := f.sessions.RedeemWithPassword(ctx, link.Token, "anna1234"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.sessions.Authenticate(ctx, old.Token); !errors.Is(err, auth.ErrNoSession) {
		t.Fatalf("old session survived: %v", err)
	}
}

func TestRevokedSetupLinkNoLongerWorks(t *testing.T) {
	ctx := context.Background()
	f := newSetupFixture(t)
	link, _ := f.sessions.IssueSetupLink(ctx, f.admin, f.member.ID)
	if err := f.sessions.RevokeSetupLink(ctx, f.admin, f.member.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.sessions.PeekSetupLink(ctx, link.Token); !errors.Is(err, auth.ErrNoSetupLink) {
		t.Fatalf("revoked link still open: %v", err)
	}
	// Revoking again is fine, and a new link works afterwards.
	if err := f.sessions.RevokeSetupLink(ctx, f.admin, f.member.ID); err != nil {
		t.Fatal(err)
	}
	next, _ := f.sessions.IssueSetupLink(ctx, f.admin, f.member.ID)
	if _, err := f.sessions.PeekSetupLink(ctx, next.Token); err != nil {
		t.Fatal(err)
	}
}

func TestRevokeSetupLinkFollowsRank(t *testing.T) {
	ctx := context.Background()
	f := newSetupFixture(t)
	if err := f.sessions.RevokeSetupLink(ctx, f.admin, f.owner.ID); !errors.Is(err, user.ErrSuperadminProtected) {
		t.Fatalf("admin → owner: %v", err)
	}
}

func TestOpenSetupLinksListsOnlyOpenOnes(t *testing.T) {
	ctx := context.Background()
	f := newSetupFixture(t)
	now := time.Now()
	f.sessions.SetClock(func() time.Time { return now })
	link, _ := f.sessions.IssueSetupLink(ctx, f.admin, f.member.ID)
	open, err := f.sessions.OpenSetupLinks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Compared to the second: db.FormatTime may drop sub-second precision.
	if got, ok := open[f.member.ID]; !ok || got.Unix() != link.ExpiresAt.Unix() {
		t.Fatalf("open = %v, want %v for %s", open, link.ExpiresAt, f.member.ID)
	}
	f.sessions.SetClock(func() time.Time { return now.Add(auth.SetupLinkTTL + time.Second) })
	open, _ = f.sessions.OpenSetupLinks(ctx)
	if _, ok := open[f.member.ID]; ok {
		t.Fatal("expired link listed")
	}
}

func TestRedeemSetupLinkOverHTTP(t *testing.T) {
	h, sessions, users := newHandlerWithSessions(t)
	ctx := context.Background()
	sam, _ := users.Authenticate(ctx, "sam", "pw")
	anna, _ := users.Create(ctx, user.CreateParams{Username: "anna", Role: user.RoleUser})
	link, err := sessions.IssueSetupLink(ctx, sam, anna.ID)
	if err != nil {
		t.Fatal(err)
	}
	rec := do(h, http.MethodPost, "/api/v1/auth/setup/inspect", `{"token":"`+link.Token+`"}`, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"username":"anna"`) {
		t.Fatalf("inspect: %d %s", rec.Code, rec.Body.String())
	}
	rec = do(h, http.MethodPost, "/api/v1/auth/setup/password", `{"token":"`+link.Token+`","password":"anna1234"}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("redeem: %d %s", rec.Code, rec.Body.String())
	}
	var sessionSet bool
	for _, c := range rec.Result().Cookies() {
		sessionSet = sessionSet || (c.Name == auth.CookieName && c.Value != "")
	}
	if !sessionSet {
		t.Fatal("no session cookie")
	}
	rec = do(h, http.MethodPost, "/api/v1/auth/setup/inspect", `{"token":"`+link.Token+`"}`, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("used link: %d", rec.Code)
	}
}
