package auth_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/db/dbtest"
	"github.com/s-frei/rezepte/service/internal/mail"
	"github.com/s-frei/rezepte/service/internal/user"
)

// fakeMailer stands in for mail.Service. sent is a channel, so a test can
// wait for the forgot-password handler's background send without a race.
type fakeMailer struct {
	off, fail bool
	sent      chan any
	clock     time.Time // mailHandler's service clock
}

func newFakeMailer() *fakeMailer { return &fakeMailer{sent: make(chan any, 16)} }

func (f *fakeMailer) Enabled(context.Context) bool { return !f.off }

func (f *fakeMailer) record(m any) error {
	if f.off {
		return mail.ErrDisabled
	}
	if f.fail {
		return fmt.Errorf("%w: 535 nope", mail.ErrSend)
	}
	f.sent <- m
	return nil
}

func (f *fakeMailer) SendReset(_ context.Context, m mail.Reset) error     { return f.record(m) }
func (f *fakeMailer) SendHint(_ context.Context, m mail.Hint) error       { return f.record(m) }
func (f *fakeMailer) SendConfirm(_ context.Context, m mail.Confirm) error { return f.record(m) }

// drain returns every mail sent so far.
func (f *fakeMailer) drain() []any {
	var out []any
	for {
		select {
		case m := <-f.sent:
			out = append(out, m)
		default:
			return out
		}
	}
}

const issuer = "https://idp.example.org"

type recoveryFixture struct {
	users    *user.Service
	sessions *auth.Service
	mailer   *fakeMailer
	clock    time.Time
	byName   map[string]user.User
}

// newRecoveryFixture: owner, jonas and kim (passwords, all three confirmed on
// shared@example.org), mila (password, confirmed), lea (password,
// unconfirmed), ida (no password, confirmed, linked to the provider), noah
// (nothing).
func newRecoveryFixture(t *testing.T) *recoveryFixture {
	t.Helper()
	ctx := context.Background()
	conn := dbtest.Open(t)
	f := &recoveryFixture{users: user.NewService(conn, ""), mailer: newFakeMailer(),
		clock: time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC), byName: map[string]user.User{}}
	f.sessions = auth.NewService(conn, f.users)
	f.sessions.SetClock(func() time.Time { return f.clock })
	f.sessions.SetMail(f.mailer, auth.Provider{Issuer: issuer, Name: "Dex"})
	mk := func(name string, role user.Role, pw, addr string, confirm bool) {
		u, err := f.users.Create(ctx, user.CreateParams{Username: name, Password: pw, Role: role, Email: addr})
		if err != nil {
			t.Fatal(err)
		}
		if confirm {
			if _, err := f.users.MarkEmailVerified(ctx, u.ID, addr); err != nil {
				t.Fatal(err)
			}
		}
		f.byName[name], _ = f.users.ByID(ctx, u.ID)
	}
	mk("owner", user.RoleSuperadmin, "pw", "shared@example.org", true)
	mk("mila", user.RoleUser, "pw", "mila@example.org", true)
	mk("jonas", user.RoleUser, "pw", "shared@example.org", true)
	mk("kim", user.RoleUser, "pw", "shared@example.org", true)
	mk("lea", user.RoleUser, "pw", "lea@example.org", false)
	mk("ida", user.RoleUser, "", "ida@example.org", true)
	mk("noah", user.RoleUser, "", "", false)
	if err := f.users.LinkIdentity(ctx, f.byName["ida"].ID, issuer, "sub-ida"); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestForgotByUsernameMailsResetLink(t *testing.T) {
	ctx := context.Background()
	f := newRecoveryFixture(t)
	f.sessions.ForgotPassword(ctx, "  MILA ")
	sent := f.mailer.drain()
	if len(sent) != 1 {
		t.Fatalf("sent %d mails, want 1: %+v", len(sent), sent)
	}
	r, ok := sent[0].(mail.Reset)
	if !ok || r.To != "mila@example.org" || r.Username != "mila" || !strings.HasPrefix(r.Path, "/welcome#") {
		t.Fatalf("mail = %+v", sent[0])
	}
	open, err := f.sessions.PeekSetupLink(ctx, strings.TrimPrefix(r.Path, "/welcome#"))
	if err != nil || open.Purpose != auth.PurposeReset {
		t.Fatalf("link = %+v, %v", open, err)
	}
}

func TestForgotByAddressReachesEveryAccount(t *testing.T) {
	f := newRecoveryFixture(t)
	f.sessions.ForgotPassword(context.Background(), "Shared@Example.org")
	var names []string
	for _, m := range f.mailer.drain() {
		names = append(names, m.(mail.Reset).Username)
	}
	// The owner shares the address and gets nothing.
	if strings.Join(names, ",") != "jonas,kim" {
		t.Fatalf("mailed %v, want jonas and kim", names)
	}
}

func TestForgotReachesNobodyElse(t *testing.T) {
	f := newRecoveryFixture(t)
	for _, login := range []string{"owner", "lea", "lea@example.org", "noah", "nobody", "nobody@example.org", "   "} {
		f.sessions.ForgotPassword(context.Background(), login)
	}
	if sent := f.mailer.drain(); len(sent) != 0 {
		t.Fatalf("sent %+v, want nothing", sent)
	}
}

func TestForgotWithoutPasswordGetsProviderHint(t *testing.T) {
	f := newRecoveryFixture(t)
	f.sessions.ForgotPassword(context.Background(), "ida")
	sent := f.mailer.drain()
	if len(sent) != 1 {
		t.Fatalf("sent %+v", sent)
	}
	if h, ok := sent[0].(mail.Hint); !ok || h.To != "ida@example.org" || h.Provider != "Dex" {
		t.Fatalf("mail = %+v", sent[0])
	}
}

func TestForgotThrottlesPerAccount(t *testing.T) {
	f := newRecoveryFixture(t)
	ctx := context.Background()
	f.sessions.ForgotPassword(ctx, "mila")
	f.sessions.ForgotPassword(ctx, "mila@example.org")
	if n := len(f.mailer.drain()); n != 1 {
		t.Fatalf("sent %d within 5 minutes, want 1", n)
	}
	f.clock = f.clock.Add(5 * time.Minute)
	f.sessions.ForgotPassword(ctx, "mila")
	if n := len(f.mailer.drain()); n != 1 {
		t.Fatalf("sent %d after 5 minutes, want 1", n)
	}
}

func TestForgotDoesNothingWithoutMail(t *testing.T) {
	f := newRecoveryFixture(t)
	f.mailer.off = true
	f.sessions.ForgotPassword(context.Background(), "mila")
	// Nothing issued and no cooldown taken: with mail back on, the next
	// request mails at once.
	f.mailer.off = false
	f.sessions.ForgotPassword(context.Background(), "mila")
	if n := len(f.mailer.drain()); n != 1 {
		t.Fatalf("sent %d, want 1", n)
	}
}

// A username may hold an @: when no confirmed address matches, the login is
// tried as a username.
func TestForgotByUsernameWithAt(t *testing.T) {
	ctx := context.Background()
	f := newRecoveryFixture(t)
	u, err := f.users.Create(ctx, user.CreateParams{Username: "max@home", Password: "pw", Role: user.RoleUser, Email: "max@example.org"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.users.MarkEmailVerified(ctx, u.ID, "max@example.org"); err != nil {
		t.Fatal(err)
	}
	f.sessions.ForgotPassword(ctx, "Max@Home")
	sent := f.mailer.drain()
	if len(sent) != 1 || sent[0].(mail.Reset).Username != "max@home" {
		t.Fatalf("sent %+v, want a reset for max@home", sent)
	}
}

// An account without a password whose identity is at another issuer than
// the configured provider has no way in to hint at.
func TestForgotWithoutPasswordAtOtherIssuerGetsNothing(t *testing.T) {
	ctx := context.Background()
	f := newRecoveryFixture(t)
	u, err := f.users.Create(ctx, user.CreateParams{Username: "eva", Role: user.RoleUser, Email: "eva@example.org"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.users.MarkEmailVerified(ctx, u.ID, "eva@example.org"); err != nil {
		t.Fatal(err)
	}
	if err := f.users.LinkIdentity(ctx, u.ID, "https://other.example.org", "sub-eva"); err != nil {
		t.Fatal(err)
	}
	f.sessions.ForgotPassword(ctx, "eva")
	if sent := f.mailer.drain(); len(sent) != 0 {
		t.Fatalf("sent %+v, want nothing", sent)
	}
}

func TestEveryAddressChangeMails(t *testing.T) {
	ctx := context.Background()
	f := newRecoveryFixture(t)
	before := f.byName["lea"]
	for _, addr := range []string{"lea@one.org", "lea@two.org"} {
		u, err := f.users.SetProfile(ctx, before.ID, user.ProfileUpdate{Email: &addr})
		if err != nil {
			t.Fatal(err)
		}
		f.sessions.ConfirmChangedAddress(ctx, before, u, "")
		if pending, _ := f.sessions.ConfirmationPending(ctx, u); !pending {
			t.Fatalf("%s: no confirmation pending", addr)
		}
		before = u
		f.clock = f.clock.Add(time.Minute)
	}
	sent := f.mailer.drain()
	if len(sent) != 2 || sent[1].(mail.Confirm).To != "lea@two.org" {
		t.Fatalf("sent %+v", sent)
	}
	first := strings.TrimPrefix(sent[0].(mail.Confirm).Path, "/confirm-email#")
	if _, err := f.sessions.Confirm(ctx, first); err == nil {
		t.Fatal("the first link still confirms")
	}
}

func TestResendConfirmation(t *testing.T) {
	ctx := context.Background()
	f := newRecoveryFixture(t)
	lea := f.byName["lea"]
	if err := f.sessions.ResendConfirmation(ctx, lea); err != nil {
		t.Fatal(err)
	}
	var throttled *auth.ThrottledError
	if err := f.sessions.ResendConfirmation(ctx, lea); !errors.As(err, &throttled) || throttled.RetryAfter != time.Minute {
		t.Fatalf("second within a minute: %v", err)
	}
	f.clock = f.clock.Add(time.Minute)
	if err := f.sessions.ResendConfirmation(ctx, lea); err != nil {
		t.Fatalf("after a minute: %v", err)
	}
	if err := f.sessions.ResendConfirmation(ctx, f.byName["mila"]); !errors.Is(err, auth.ErrNothingToConfirm) {
		t.Fatalf("confirmed: %v", err)
	}
	if err := f.sessions.ResendConfirmation(ctx, f.byName["noah"]); !errors.Is(err, auth.ErrNothingToConfirm) {
		t.Fatalf("no address: %v", err)
	}
	f.mailer.off = true
	if err := f.sessions.ResendConfirmation(ctx, lea); !errors.Is(err, mail.ErrDisabled) {
		t.Fatalf("mail off: %v", err)
	}
}

// mailHandler is newHandlerWithSessions with a fake mailer and sam holding a
// confirmed address. Its clock moves only when a test moves m.clock.
func mailHandler(t *testing.T) (http.Handler, *fakeMailer) {
	t.Helper()
	h, sessions, users := newHandlerWithSessions(t)
	m := newFakeMailer()
	m.clock = time.Now()
	sessions.SetClock(func() time.Time { return m.clock })
	sessions.SetMail(m, auth.Provider{})
	ctx := context.Background()
	sam, _ := users.ByUsername(ctx, "sam")
	addr := "sam@example.org"
	if _, err := users.SetProfile(ctx, sam.ID, user.ProfileUpdate{Email: &addr}); err != nil {
		t.Fatal(err)
	}
	if _, err := users.MarkEmailVerified(ctx, sam.ID, addr); err != nil {
		t.Fatal(err)
	}
	return h, m
}

func TestForgotPasswordAnswersTheSame(t *testing.T) {
	h, m := mailHandler(t)
	for _, login := range []string{"nobody", "sam"} {
		rec := do(h, http.MethodPost, "/api/v1/auth/password/forgot", `{"login":"`+login+`"}`, nil)
		if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
			t.Fatalf("%s: %d %s", login, rec.Code, rec.Body)
		}
	}
	select {
	case got := <-m.sent:
		if r, ok := got.(mail.Reset); !ok || r.Username != "sam" {
			t.Fatalf("mail = %+v", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no reset mail sent in the background")
	}
	for _, body := range []string{`{"login":""}`, `{"login":"` + strings.Repeat("a", 255) + `"}`} {
		if rec := do(h, http.MethodPost, "/api/v1/auth/password/forgot", body, nil); rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("%s: status %d", body[:12], rec.Code)
		}
	}
}

func TestPasswordResetConfig(t *testing.T) {
	h, sessions, _ := newHandlerWithSessions(t)
	if rec := do(h, http.MethodGet, "/api/v1/auth/password", "", nil); !strings.Contains(rec.Body.String(), `"available":false`) {
		t.Fatalf("without mail: %d %s", rec.Code, rec.Body)
	}
	sessions.SetMail(newFakeMailer(), auth.Provider{})
	if rec := do(h, http.MethodGet, "/api/v1/auth/password", "", nil); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"available":true`) {
		t.Fatalf("with mail: %d %s", rec.Code, rec.Body)
	}
}

func TestConfirmEmailEndpoint(t *testing.T) {
	h, sessions, users := newHandlerWithSessions(t)
	ctx := context.Background()
	sam, _ := users.ByUsername(ctx, "sam")
	addr := "sam@example.org"
	_, _ = users.SetProfile(ctx, sam.ID, user.ProfileUpdate{Email: &addr})
	token, _ := sessions.IssueConfirmation(ctx, sam.ID, addr)
	rec := do(h, http.MethodPost, "/api/v1/auth/email/confirm", `{"token":"`+token+`"}`, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"address":"sam@example.org"`) {
		t.Fatalf("confirm = %d %s", rec.Code, rec.Body)
	}
	if len(rec.Result().Cookies()) != 0 {
		t.Fatalf("confirming set cookies: %v", rec.Result().Cookies())
	}
	if rec := do(h, http.MethodPost, "/api/v1/auth/email/confirm", `{"token":"`+token+`"}`, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("second use: %d", rec.Code)
	}
}

func TestProfileSaveMailsConfirmation(t *testing.T) {
	h, m := mailHandler(t)
	c := login(t, h)
	rec := do(h, http.MethodPatch, "/api/v1/auth/me/profile", `{"email":"new@example.org"}`, c)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"emailConfirmationPending":true`) || strings.Contains(rec.Body.String(), "confirmationSent") {
		t.Fatalf("save = %d %s", rec.Code, rec.Body)
	}
	if got := m.drain(); len(got) != 1 || got[0].(mail.Confirm).To != "new@example.org" || got[0].(mail.Confirm).Admin != "" || got[0].(mail.Confirm).Username != "sam" {
		t.Fatalf("sent %+v", got)
	}
	// The save's mail took the minute "Send again" waits for.
	rec = do(h, http.MethodPost, "/api/v1/auth/me/email/confirmation", "", c)
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") != "60" {
		t.Fatalf("resend right after the save = %d Retry-After=%q", rec.Code, rec.Header().Get("Retry-After"))
	}
	m.clock = m.clock.Add(time.Minute)
	if rec := do(h, http.MethodPost, "/api/v1/auth/me/email/confirmation", "", c); rec.Code != http.StatusNoContent {
		t.Fatalf("resend after a minute = %d %s", rec.Code, rec.Body)
	}
}

func TestProfileSaveReportsAFailedConfirmation(t *testing.T) {
	h, m := mailHandler(t)
	m.fail = true
	c := login(t, h)
	rec := do(h, http.MethodPatch, "/api/v1/auth/me/profile", `{"email":"new@example.org"}`, c)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"emailConfirmationPending":false`) {
		t.Fatalf("save = %d %s", rec.Code, rec.Body)
	}
}

func TestProfileSaveWithoutAddressChangeMailsNothing(t *testing.T) {
	h, m := mailHandler(t)
	c := login(t, h)
	rec := do(h, http.MethodPatch, "/api/v1/auth/me/profile", `{"displayName":"Sam","email":"sam@example.org"}`, c)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"emailVerified":true`) {
		t.Fatalf("save = %d %s", rec.Code, rec.Body)
	}
	if got := m.drain(); len(got) != 0 {
		t.Fatalf("sent %+v", got)
	}
}

func TestResendConfirmationConflicts(t *testing.T) {
	h, m := mailHandler(t)
	c := login(t, h)
	if rec := do(h, http.MethodPost, "/api/v1/auth/me/email/confirmation", "", c); rec.Code != http.StatusConflict {
		t.Fatalf("confirmed address: %d", rec.Code)
	}
	m.off = true
	if rec := do(h, http.MethodPatch, "/api/v1/auth/me/profile", `{"email":"new@example.org"}`, c); !strings.Contains(rec.Body.String(), `"emailConfirmationPending":false`) {
		t.Fatalf("mail off reported a send: %s", rec.Body)
	}
	if rec := do(h, http.MethodPost, "/api/v1/auth/me/email/confirmation", "", c); rec.Code != http.StatusConflict {
		t.Fatalf("mail off: %d", rec.Code)
	}
}

// The profile may only say a mail went out when one did: the flag follows an
// open link for the current address, and a failed send leaves none behind.
func TestMeReportsAnOpenConfirmation(t *testing.T) {
	h, m := mailHandler(t)
	c := login(t, h)
	pendingIs := func(want bool) {
		t.Helper()
		rec := do(h, http.MethodGet, "/api/v1/auth/me", "", c)
		if !strings.Contains(rec.Body.String(), fmt.Sprintf(`"emailConfirmationPending":%v`, want)) {
			t.Fatalf("me = %d %s; want pending %v", rec.Code, rec.Body, want)
		}
	}
	rec := do(h, http.MethodPatch, "/api/v1/auth/me/profile", `{"email":"new@example.org"}`, c)
	if !strings.Contains(rec.Body.String(), `"emailConfirmationPending":true`) {
		t.Fatalf("save = %d %s", rec.Code, rec.Body)
	}
	pendingIs(true)
	m.clock = m.clock.Add(time.Minute)
	m.fail = true
	if rec := do(h, http.MethodPost, "/api/v1/auth/me/email/confirmation", "", c); rec.Code == http.StatusNoContent {
		t.Fatalf("resend with a failing mailer = %d", rec.Code)
	}
	pendingIs(false)
	m.clock = m.clock.Add(time.Minute)
	rec = do(h, http.MethodPatch, "/api/v1/auth/me/profile", `{"email":"third@example.org"}`, c)
	if !strings.Contains(rec.Body.String(), `"emailConfirmationPending":false`) {
		t.Fatalf("failed save = %d %s", rec.Code, rec.Body)
	}
}

// resetToken asks for a reset of name and returns the mailed token.
func (f *recoveryFixture) resetToken(t *testing.T, name string) string {
	t.Helper()
	f.sessions.ForgotPassword(context.Background(), name)
	sent := f.mailer.drain()
	if len(sent) != 1 {
		t.Fatalf("forgot %s sent %+v", name, sent)
	}
	return strings.TrimPrefix(sent[0].(mail.Reset).Path, "/welcome#")
}

// A reset link went to the address the account had when it was asked for,
// for the password it had then: either changing closes it.
func TestAddressOrPasswordChangeClosesTheResetLink(t *testing.T) {
	ctx := context.Background()
	f := newRecoveryFixture(t)
	mila := f.byName["mila"]
	token := f.resetToken(t, "mila")
	addr := "mila@new.org"
	if _, err := f.users.SetProfile(ctx, mila.ID, user.ProfileUpdate{Email: &addr}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.sessions.PeekSetupLink(ctx, token); !errors.Is(err, auth.ErrNoSetupLink) {
		t.Fatalf("after an address change: %v", err)
	}

	jonas := f.byName["jonas"]
	token = f.resetToken(t, "jonas")
	if err := f.users.ChangePassword(ctx, jonas.ID, "pw", "a-new-password"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.sessions.PeekSetupLink(ctx, token); !errors.Is(err, auth.ErrNoSetupLink) {
		t.Fatalf("after a password change: %v", err)
	}

	// The same address in other letters is no change and closes nothing.
	kim := f.byName["kim"]
	token = f.resetToken(t, "kim")
	upper := "SHARED@example.org"
	if _, err := f.users.SetProfile(ctx, kim.ID, user.ProfileUpdate{Email: &upper}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.sessions.PeekSetupLink(ctx, token); err != nil {
		t.Fatalf("after a case-only change: %v", err)
	}

	// An admin's setup link is not a reset link and stays open.
	link, err := f.sessions.IssueSetupLink(ctx, f.byName["owner"], kim.ID)
	if err != nil {
		t.Fatal(err)
	}
	other := "kim@new.org"
	if _, err := f.users.SetEmail(ctx, f.byName["owner"], kim.ID, other); err != nil {
		t.Fatal(err)
	}
	if _, err := f.sessions.PeekSetupLink(ctx, link.Token); err != nil {
		t.Fatalf("setup link after an address change: %v", err)
	}
}

// A forgotten-password request never takes the place of an admin's open
// setup link; an expired one, or an older reset link, it replaces.
func TestForgotLeavesAnOpenSetupLink(t *testing.T) {
	ctx := context.Background()
	f := newRecoveryFixture(t)
	mila := f.byName["mila"]
	link, err := f.sessions.IssueSetupLink(ctx, f.byName["owner"], mila.ID)
	if err != nil {
		t.Fatal(err)
	}
	f.sessions.ForgotPassword(ctx, "mila")
	if sent := f.mailer.drain(); len(sent) != 0 {
		t.Fatalf("sent %+v while a setup link is open", sent)
	}
	if open, err := f.sessions.PeekSetupLink(ctx, link.Token); err != nil || open.Purpose != auth.PurposeSetup {
		t.Fatalf("setup link = %+v, %v", open, err)
	}
	f.clock = f.clock.Add(auth.SetupLinkTTL)
	first := f.resetToken(t, "mila")
	f.clock = f.clock.Add(5 * time.Minute)
	second := f.resetToken(t, "mila")
	if _, err := f.sessions.PeekSetupLink(ctx, first); !errors.Is(err, auth.ErrNoSetupLink) {
		t.Fatalf("older reset link: %v", err)
	}
	if _, err := f.sessions.PeekSetupLink(ctx, second); err != nil {
		t.Fatalf("newer reset link: %v", err)
	}
}

// The automatic mail after the member's own change and "Send again" share
// one minute per account; an admin's change is not held back by it.
func TestOwnAddressChangeSharesTheResendMinute(t *testing.T) {
	ctx := context.Background()
	f := newRecoveryFixture(t)
	lea := f.byName["lea"]
	change := func(addr, by string) user.User {
		t.Helper()
		before, _ := f.users.ByID(ctx, lea.ID)
		after, err := f.users.SetProfile(ctx, lea.ID, user.ProfileUpdate{Email: &addr})
		if err != nil {
			t.Fatal(err)
		}
		f.sessions.ConfirmChangedAddress(ctx, before, after, by)
		return after
	}
	change("lea@one.org", "")
	var throttled *auth.ThrottledError
	if err := f.sessions.ResendConfirmation(ctx, f.byName["lea"]); !errors.As(err, &throttled) {
		t.Fatalf("resend right after the save: %v", err)
	}
	after := change("lea@two.org", "")
	if sent := f.mailer.drain(); len(sent) != 1 {
		t.Fatalf("sent %d within a minute, want 1: %+v", len(sent), sent)
	}
	if pending, _ := f.sessions.ConfirmationPending(ctx, after); pending {
		t.Fatal("pending although the held-back mail never went out")
	}
	change("lea@three.org", "Owner")
	if sent := f.mailer.drain(); len(sent) != 1 || sent[0].(mail.Confirm).Admin != "Owner" {
		t.Fatalf("admin change sent %+v", sent)
	}
}

// A failed send leaves the minute unused, so the retry the profile offers
// goes out at once, after a save and after "Send again" alike.
func TestFailedSendKeepsTheMinute(t *testing.T) {
	ctx := context.Background()
	f := newRecoveryFixture(t)
	lea := f.byName["lea"]
	addr := "lea@one.org"
	after, err := f.users.SetProfile(ctx, lea.ID, user.ProfileUpdate{Email: &addr})
	if err != nil {
		t.Fatal(err)
	}
	f.mailer.fail = true
	f.sessions.ConfirmChangedAddress(ctx, lea, after, "")
	if err := f.sessions.ResendConfirmation(ctx, after); !errors.Is(err, mail.ErrSend) {
		t.Fatalf("resend after a failed save: %v", err)
	}
	f.mailer.fail = false
	if err := f.sessions.ResendConfirmation(ctx, after); err != nil {
		t.Fatalf("resend after a failed resend: %v", err)
	}
	if sent := f.mailer.drain(); len(sent) != 1 {
		t.Fatalf("sent %d, want 1", len(sent))
	}
}

// confirmRacer issues a newer confirmation while the first one is being
// sent, then fails that send.
type confirmRacer struct {
	*fakeMailer
	race func()
}

func (r *confirmRacer) SendConfirm(context.Context, mail.Confirm) error {
	r.race()
	return fmt.Errorf("%w: 451 later", mail.ErrSend)
}

func TestFailedSendDropsOnlyItsOwnConfirmation(t *testing.T) {
	ctx := context.Background()
	f := newRecoveryFixture(t)
	lea := f.byName["lea"]
	f.sessions.SetMail(&confirmRacer{fakeMailer: f.mailer, race: func() {
		if _, err := f.sessions.IssueConfirmation(ctx, lea.ID, lea.Email); err != nil {
			t.Error(err)
		}
	}}, auth.Provider{})
	if err := f.sessions.ResendConfirmation(ctx, lea); !errors.Is(err, mail.ErrSend) {
		t.Fatalf("resend: %v", err)
	}
	if pending, err := f.sessions.ConfirmationPending(ctx, lea); err != nil || !pending {
		t.Fatalf("the newer confirmation is gone: %v %v", pending, err)
	}
}

// panicMailer records the reset and then panics, like a bug in a send would.
type panicMailer struct{ *fakeMailer }

func (p panicMailer) SendReset(_ context.Context, m mail.Reset) error {
	p.sent <- m
	panic("boom")
}

func TestForgotPasswordSurvivesAPanic(t *testing.T) {
	h, sessions, users := newHandlerWithSessions(t)
	m := newFakeMailer()
	sessions.SetMail(panicMailer{m}, auth.Provider{})
	ctx := context.Background()
	sam, _ := users.ByUsername(ctx, "sam")
	addr := "sam@example.org"
	_, _ = users.SetProfile(ctx, sam.ID, user.ProfileUpdate{Email: &addr})
	_, _ = users.MarkEmailVerified(ctx, sam.ID, addr)
	do(h, http.MethodPost, "/api/v1/auth/password/forgot", `{"login":"sam"}`, nil)
	select {
	case <-m.sent:
	case <-time.After(5 * time.Second):
		t.Fatal("no reset attempted")
	}
	// Without a recover the panic ends the test binary about now.
	time.Sleep(100 * time.Millisecond)
}
