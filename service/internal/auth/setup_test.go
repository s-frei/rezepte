package auth_test

import (
	"context"
	"database/sql"
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
	conn     *sql.DB
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
		conn: conn, users: users, sessions: auth.NewService(conn, users),
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
	// A reset the admin asked for themselves is no setup link to show.
	if _, err := f.sessions.IssueResetLink(ctx, f.admin.ID); err != nil {
		t.Fatal(err)
	}
	open, err := f.sessions.OpenSetupLinks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := open[f.admin.ID]; ok {
		t.Fatal("reset link listed as a setup link")
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

func ptr[T any](v T) *T { return &v }

func TestConsumeVerifiesOnlyMatchingAddress(t *testing.T) {
	for name, tc := range map[string]struct {
		sentTo, emailNow string
		want             bool
	}{
		"mailed to current address": {"lena@example.org", "lena@example.org", true},
		"address changed since":     {"lena@example.org", "lena@other.org", false},
		"not mailed":                {"", "lena@example.org", false},
	} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			f := newSetupFixture(t)
			lena, err := f.users.Create(ctx, user.CreateParams{Username: "lena", Role: user.RoleUser})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := f.users.SetProfile(ctx, lena.ID, user.ProfileUpdate{Email: ptr("lena@example.org")}); err != nil {
				t.Fatal(err)
			}
			link, err := f.sessions.IssueSetupLink(ctx, f.owner, lena.ID)
			if err != nil {
				t.Fatal(err)
			}
			if tc.sentTo != "" {
				if err := f.sessions.MarkSetupLinkSent(ctx, link.Token, tc.sentTo); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := f.users.SetProfile(ctx, lena.ID, user.ProfileUpdate{Email: ptr(tc.emailNow)}); err != nil {
				t.Fatal(err)
			}
			if _, err := f.sessions.RedeemWithPassword(ctx, link.Token, "a-long-password"); err != nil {
				t.Fatal(err)
			}
			got, _ := f.users.ByID(ctx, lena.ID)
			if got.EmailVerified != tc.want {
				t.Fatalf("EmailVerified = %v, want %v", got.EmailVerified, tc.want)
			}
		})
	}
}

func TestConsumeByHashVerifies(t *testing.T) {
	ctx := context.Background()
	f := newSetupFixture(t)
	lena, _ := f.users.Create(ctx, user.CreateParams{Username: "lena", Role: user.RoleUser})
	_, _ = f.users.SetProfile(ctx, lena.ID, user.ProfileUpdate{Email: ptr("lena@example.org")})
	link, _ := f.sessions.IssueSetupLink(ctx, f.owner, lena.ID)
	_ = f.sessions.MarkSetupLinkSent(ctx, link.Token, "lena@example.org")
	if _, err := f.sessions.ConsumeSetupLinkByHash(ctx, auth.SetupLinkHash(link.Token)); err != nil {
		t.Fatal(err)
	}
	if got, _ := f.users.ByID(ctx, lena.ID); !got.EmailVerified {
		t.Fatal("OIDC setup path did not verify the mailed address")
	}
}

func TestMarkSentIsKeyedByLinkNotAccount(t *testing.T) {
	ctx := context.Background()
	f := newSetupFixture(t)
	lena, _ := f.users.Create(ctx, user.CreateParams{Username: "lena", Role: user.RoleUser, Email: "lena@example.org"})
	l1, _ := f.sessions.IssueSetupLink(ctx, f.owner, lena.ID)
	l2, _ := f.sessions.IssueSetupLink(ctx, f.owner, lena.ID) // replaces l1 while its mail is "in flight"
	if err := f.sessions.MarkSetupLinkSent(ctx, l1.Token, "lena@example.org"); err != nil {
		t.Fatalf("marking a replaced link must be a no-op: %v", err)
	}
	if _, err := f.sessions.RedeemWithPassword(ctx, l2.Token, "a-long-password"); err != nil {
		t.Fatal(err)
	}
	if got, _ := f.users.ByID(ctx, lena.ID); got.EmailVerified {
		t.Fatal("a never-mailed link verified the address")
	}
}

func TestResetLinkIsOneHourAndPeeksAsReset(t *testing.T) {
	ctx := context.Background()
	f := newSetupFixture(t)
	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	f.sessions.SetClock(func() time.Time { return now })
	link, err := f.sessions.IssueResetLink(ctx, f.admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !link.ExpiresAt.Equal(now.Add(auth.ResetLinkTTL)) {
		t.Fatalf("expires %v, want an hour from now", link.ExpiresAt)
	}
	open, err := f.sessions.PeekSetupLink(ctx, link.Token)
	if err != nil || open.Purpose != auth.PurposeReset || open.User.ID != f.admin.ID {
		t.Fatalf("peek = %+v, %v", open, err)
	}
	f.sessions.SetClock(func() time.Time { return now.Add(auth.ResetLinkTTL + time.Second) })
	if _, err := f.sessions.PeekSetupLink(ctx, link.Token); !errors.Is(err, auth.ErrNoSetupLink) {
		t.Fatalf("after an hour: err = %v", err)
	}
}

func TestResetLinkLeavesOpenSetupLink(t *testing.T) {
	ctx := context.Background()
	f := newSetupFixture(t)
	setup, _ := f.sessions.IssueSetupLink(ctx, f.owner, f.admin.ID)
	if _, err := f.sessions.IssueResetLink(ctx, f.admin.ID); err == nil {
		t.Fatal("a reset link replaced an open setup link")
	}
	if _, err := f.sessions.PeekSetupLink(ctx, setup.Token); err != nil {
		t.Fatalf("the admin's link no longer opens: %v", err)
	}
}

func TestResetLinkRefusedForOwner(t *testing.T) {
	f := newSetupFixture(t)
	if _, err := f.sessions.IssueResetLink(context.Background(), f.owner.ID); !errors.Is(err, user.ErrSuperadminProtected) {
		t.Fatalf("err = %v, want ErrSuperadminProtected", err)
	}
}

// The OIDC flow peeks by hash; a reset link must never get that far.
func TestResetLinkRefusedByOIDCPeek(t *testing.T) {
	ctx := context.Background()
	f := newSetupFixture(t)
	reset, _ := f.sessions.IssueResetLink(ctx, f.admin.ID)
	if _, err := f.sessions.PeekSetupLinkByHash(ctx, auth.SetupLinkHash(reset.Token)); !errors.Is(err, auth.ErrNoSetupLink) {
		t.Fatalf("OIDC peek of a reset link: err = %v", err)
	}
	setup, _ := f.sessions.IssueSetupLink(ctx, f.admin, f.member.ID)
	if u, err := f.sessions.PeekSetupLinkByHash(ctx, auth.SetupLinkHash(setup.Token)); err != nil || u.ID != f.member.ID {
		t.Fatalf("OIDC peek of a setup link: %+v, %v", u, err)
	}
}

func TestRedeemResetLinkEndsEverySession(t *testing.T) {
	ctx := context.Background()
	f := newSetupFixture(t)
	old, err := f.sessions.StartSession(ctx, f.admin)
	if err != nil {
		t.Fatal(err)
	}
	reset, _ := f.sessions.IssueResetLink(ctx, f.admin.ID)
	if _, err := f.sessions.RedeemWithPassword(ctx, reset.Token, "sam-new-pass"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.sessions.Authenticate(ctx, old.Token); !errors.Is(err, auth.ErrNoSession) {
		t.Fatalf("old session: err = %v, want ErrNoSession", err)
	}
	if _, err := f.users.Authenticate(ctx, "sam", "sam-new-pass"); err != nil {
		t.Fatal(err)
	}
}

func TestSetupLinkReplacesOpenResetLink(t *testing.T) {
	ctx := context.Background()
	f := newSetupFixture(t)
	if _, err := f.sessions.IssueResetLink(ctx, f.admin.ID); err != nil {
		t.Fatal(err)
	}
	setup, err := f.sessions.IssueSetupLink(ctx, f.owner, f.admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	open, err := f.sessions.PeekSetupLink(ctx, setup.Token)
	if err != nil || open.Purpose != auth.PurposeSetup {
		t.Fatalf("peek = %+v, %v", open, err)
	}
	if u, err := f.sessions.PeekSetupLinkByHash(ctx, auth.SetupLinkHash(setup.Token)); err != nil || u.ID != f.admin.ID {
		t.Fatalf("peek by hash = %+v, %v", u, err)
	}
}
