package auth_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/mail"
	"github.com/s-frei/rezepte/service/internal/user"
)

// fakeNotifier records the invitations it was told were accepted.
type fakeNotifier struct {
	inviters  []string
	newcomers []user.User
}

func (n *fakeNotifier) InviteAccepted(_ context.Context, inviterID string, newcomer user.User) {
	n.inviters = append(n.inviters, inviterID)
	n.newcomers = append(n.newcomers, newcomer)
}

// onlyPasswordChanged fails unless sent is exactly one PasswordChanged.
func onlyPasswordChanged(t *testing.T, sent []any) mail.PasswordChanged {
	t.Helper()
	if len(sent) != 1 {
		t.Fatalf("sent %+v, want one PasswordChanged", sent)
	}
	pc, ok := sent[0].(mail.PasswordChanged)
	if !ok {
		t.Fatalf("sent %+v, want a PasswordChanged", sent[0])
	}
	return pc
}

func TestChangePasswordMailsHolder(t *testing.T) {
	h, m := mailHandler(t)
	c := login(t, h)
	if rec := do(h, http.MethodPatch, "/api/v1/auth/me", `{"currentPassword":"pw","password":"brand-new-pw"}`, c); rec.Code != http.StatusNoContent {
		t.Fatalf("change = %d %s", rec.Code, rec.Body)
	}
	pc := onlyPasswordChanged(t, m.drain())
	if pc.To != "sam@example.org" || pc.Username != "sam" || pc.Origin != mail.OriginProfile || pc.Admin != "" || pc.At.IsZero() {
		t.Fatalf("mail = %+v", pc)
	}
}

func TestPasswordChangedSkipsUnverified(t *testing.T) {
	f := newRecoveryFixture(t)
	f.sessions.PasswordChanged(context.Background(), f.byName["lea"], mail.OriginProfile, "")
	f.sessions.PasswordChanged(context.Background(), f.byName["noah"], mail.OriginProfile, "")
	if sent := f.mailer.drain(); len(sent) != 0 {
		t.Fatalf("sent %+v", sent)
	}
}

func TestPasswordChangedWithMailOff(t *testing.T) {
	f := newRecoveryFixture(t)
	f.mailer.off = true
	f.sessions.PasswordChanged(context.Background(), f.byName["mila"], mail.OriginProfile, "")
	if sent := f.mailer.drain(); len(sent) != 0 {
		t.Fatalf("sent %+v", sent)
	}
}

func TestResetRedeemMailsResetOrigin(t *testing.T) {
	ctx := context.Background()
	f := newRecoveryFixture(t)
	f.sessions.ForgotPassword(ctx, "mila")
	sent := f.mailer.drain()
	if len(sent) != 1 {
		t.Fatalf("sent %+v", sent)
	}
	token := strings.TrimPrefix(sent[0].(mail.Reset).Path, "/welcome#")
	if _, err := f.sessions.RedeemWithPassword(ctx, token, "mila-new-pw"); err != nil {
		t.Fatal(err)
	}
	pc := onlyPasswordChanged(t, f.mailer.drain())
	if pc.To != "mila@example.org" || pc.Origin != mail.OriginResetLink {
		t.Fatalf("mail = %+v", pc)
	}
}

func TestRedeemOnExistingAccountNotifiesHolder(t *testing.T) {
	ctx := context.Background()
	f := newRecoveryFixture(t)
	n := &fakeNotifier{}
	f.sessions.SetNotifier(n)
	owner := f.byName["owner"]
	link, err := f.sessions.IssueSetupLink(ctx, owner, f.byName["mila"].ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.sessions.RedeemWithPassword(ctx, link.Token, "mila-new-pw"); err != nil {
		t.Fatal(err)
	}
	pc := onlyPasswordChanged(t, f.mailer.drain())
	if pc.To != "mila@example.org" || pc.Origin != mail.OriginSetupLink || pc.Admin != owner.DisplayName || pc.Admin == "" || pc.First {
		t.Fatalf("mail = %+v", pc)
	}
	if len(n.inviters) != 0 {
		t.Fatalf("notifier called: %+v", n.inviters)
	}
}

func TestRedeemNewAccountCallsNotifier(t *testing.T) {
	ctx := context.Background()
	f := newRecoveryFixture(t)
	n := &fakeNotifier{}
	f.sessions.SetNotifier(n)
	owner, noah := f.byName["owner"], f.byName["noah"]
	link, err := f.sessions.IssueSetupLink(ctx, owner, noah.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.sessions.RedeemWithPassword(ctx, link.Token, "noah-new-pw"); err != nil {
		t.Fatal(err)
	}
	if len(n.inviters) != 1 || n.inviters[0] != owner.ID || n.newcomers[0].ID != noah.ID {
		t.Fatalf("notifier = %+v", n)
	}
	if sent := f.mailer.drain(); len(sent) != 0 {
		t.Fatalf("sent %+v", sent)
	}
}

// The OIDC setup links the identity first and consumes the link after, as
// oidc.setup does: that fresh identity is not a credential the account had.
func TestOIDCSetupOfNewcomerCallsNotifier(t *testing.T) {
	ctx := context.Background()
	f := newRecoveryFixture(t)
	n := &fakeNotifier{}
	f.sessions.SetNotifier(n)
	f.clock = time.Now().Add(-time.Hour) // the link is older than the identity
	owner, noah := f.byName["owner"], f.byName["noah"]
	link, err := f.sessions.IssueSetupLink(ctx, owner, noah.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.users.LinkIdentity(ctx, noah.ID, issuer, "sub-noah"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.sessions.ConsumeSetupLinkByHash(ctx, auth.SetupLinkHash(link.Token), true); err != nil {
		t.Fatal(err)
	}
	if len(n.inviters) != 1 || n.inviters[0] != owner.ID || n.newcomers[0].ID != noah.ID {
		t.Fatalf("notifier = %+v", n)
	}
	if sent := f.mailer.drain(); len(sent) != 0 {
		t.Fatalf("sent %+v", sent)
	}
}

// A setup link redeemed through OIDC on an account that could sign in
// already adds a way in: whoever held the link now has one, so the holder
// hears of it, naming the admin who issued the link. Signing in with the
// identity the account had adds nothing and mails nothing.
func TestOIDCSetupOnExistingAccountNoticesTheHolder(t *testing.T) {
	ctx := context.Background()
	f := newRecoveryFixture(t)
	n := &fakeNotifier{}
	f.sessions.SetNotifier(n)
	f.clock = time.Now().Add(time.Hour) // ida's identity is older than the link
	owner := f.byName["owner"]
	redeem := func(name, subject string) []any {
		t.Helper()
		u := f.byName[name]
		link, err := f.sessions.IssueSetupLink(ctx, owner, u.ID)
		if err != nil {
			t.Fatal(err)
		}
		added, err := f.users.LinkIdentity(ctx, u.ID, issuer, subject)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.sessions.ConsumeSetupLinkByHash(ctx, auth.SetupLinkHash(link.Token), added); err != nil {
			t.Fatal(err)
		}
		return f.mailer.drain()
	}
	// mila has a password and links a new identity.
	sent := redeem("mila", "sub-mila")
	if len(sent) != 1 {
		t.Fatalf("mila: sent %+v, want one IdentityLinked", sent)
	}
	il, ok := sent[0].(mail.IdentityLinked)
	if !ok || il.To != "mila@example.org" || il.Username != "mila" || il.Origin != mail.IdentitySetupLink ||
		il.Admin != owner.DisplayName || il.Admin == "" || il.Provider != "Dex" || il.At.IsZero() {
		t.Fatalf("mila: notice = %+v", sent[0])
	}
	// ida signs in with the identity she has had all along.
	if sent := redeem("ida", "sub-ida"); len(sent) != 0 {
		t.Fatalf("ida: sent %+v, nothing was added", sent)
	}
	if len(n.inviters) != 0 {
		t.Fatalf("notifier %+v", n.inviters)
	}
}

// The notice goes to the address confirmed before the redeem: a mailed link
// verifies its address on redeeming, and whoever opened it there is the one
// the notice is about.
func TestSetupNoticeSkipsAnAddressTheRedeemConfirmed(t *testing.T) {
	ctx := context.Background()
	f := newRecoveryFixture(t)
	lea := f.byName["lea"] // a password, lea@example.org unconfirmed
	link, err := f.sessions.IssueSetupLink(ctx, f.byName["owner"], lea.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.sessions.MarkSetupLinkSent(ctx, link.Token, lea.Email); err != nil {
		t.Fatal(err)
	}
	added, err := f.users.LinkIdentity(ctx, lea.ID, issuer, "sub-lea")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.sessions.ConsumeSetupLinkByHash(ctx, auth.SetupLinkHash(link.Token), added); err != nil {
		t.Fatal(err)
	}
	if got, _ := f.users.ByID(ctx, lea.ID); !got.EmailVerified {
		t.Fatal("the redeem did not verify the mailed address")
	}
	if sent := f.mailer.drain(); len(sent) != 0 {
		t.Fatalf("sent %+v, want nothing: no address was confirmed before", sent)
	}
}

// An account that signed in only through the provider and redeems a setup
// link with a password, or sets one in the profile, hears that a password
// was set, not changed.
func TestFirstPasswordNoticeSaysSet(t *testing.T) {
	ctx := context.Background()
	f := newRecoveryFixture(t)
	f.clock = time.Now().Add(time.Hour) // ida's identity is older than the link
	ida := f.byName["ida"]
	link, err := f.sessions.IssueSetupLink(ctx, f.byName["owner"], ida.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.sessions.RedeemWithPassword(ctx, link.Token, "ida-first-pw"); err != nil {
		t.Fatal(err)
	}
	if pc := onlyPasswordChanged(t, f.mailer.drain()); !pc.First || pc.Origin != mail.OriginSetupLink {
		t.Fatalf("mail = %+v", pc)
	}
	f.sessions.PasswordChanged(ctx, ida, mail.OriginProfile, "") // ida as she was before
	if pc := onlyPasswordChanged(t, f.mailer.drain()); !pc.First {
		t.Fatalf("mail = %+v", pc)
	}
}

func TestOwnerPasswordNoticeIsMarked(t *testing.T) {
	f := newRecoveryFixture(t)
	f.sessions.PasswordChanged(context.Background(), f.byName["owner"], mail.OriginCommandLine, "")
	if pc := onlyPasswordChanged(t, f.mailer.drain()); !pc.Owner || pc.Origin != mail.OriginCommandLine {
		t.Fatalf("owner notice = %+v", pc)
	}
	f.sessions.PasswordChanged(context.Background(), f.byName["mila"], mail.OriginProfile, "")
	if pc := onlyPasswordChanged(t, f.mailer.drain()); pc.Owner {
		t.Fatalf("member notice = %+v", pc)
	}
}

func TestRedeemWithoutNotifierIsSafe(t *testing.T) {
	ctx := context.Background()
	f := newRecoveryFixture(t)
	link, err := f.sessions.IssueSetupLink(ctx, f.byName["owner"], f.byName["noah"].ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.sessions.RedeemWithPassword(ctx, link.Token, "noah-new-pw"); err != nil {
		t.Fatal(err)
	}
}

func TestAddressChangeMailsOldVerifiedAddress(t *testing.T) {
	h, m := mailHandler(t)
	c := login(t, h)
	if rec := do(h, http.MethodPatch, "/api/v1/auth/me/profile", `{"email":"new@example.org"}`, c); rec.Code != http.StatusOK {
		t.Fatalf("save = %d %s", rec.Code, rec.Body)
	}
	sent := m.drain()
	if len(sent) != 2 {
		t.Fatalf("sent %+v, want notice and confirmation", sent)
	}
	ac, ok := sent[0].(mail.EmailChanged)
	if !ok || ac.To != "sam@example.org" || ac.NewAddress != "new@example.org" || ac.Admin != "" || ac.Username != "sam" {
		t.Fatalf("notice = %+v", sent[0])
	}
	if cf, ok := sent[1].(mail.Confirm); !ok || cf.To != "new@example.org" {
		t.Fatalf("confirmation = %+v", sent[1])
	}
}

func TestAddressRemovedMailsOldAddress(t *testing.T) {
	h, m := mailHandler(t)
	c := login(t, h)
	if rec := do(h, http.MethodPatch, "/api/v1/auth/me/profile", `{"email":""}`, c); rec.Code != http.StatusOK {
		t.Fatalf("save = %d %s", rec.Code, rec.Body)
	}
	sent := m.drain()
	if len(sent) != 1 {
		t.Fatalf("sent %+v", sent)
	}
	if ac, ok := sent[0].(mail.EmailChanged); !ok || ac.To != "sam@example.org" || ac.NewAddress != "" {
		t.Fatalf("notice = %+v", sent[0])
	}
}

func TestAddressChangeFromUnverifiedSendsNoNotice(t *testing.T) {
	ctx := context.Background()
	f := newRecoveryFixture(t)
	before := f.byName["lea"]
	addr := "lea@new.org"
	after, err := f.users.SetProfile(ctx, before.ID, user.ProfileUpdate{Email: &addr})
	if err != nil {
		t.Fatal(err)
	}
	f.sessions.ConfirmChangedAddress(ctx, before, after, "")
	sent := f.mailer.drain()
	if len(sent) != 1 {
		t.Fatalf("sent %+v", sent)
	}
	if _, ok := sent[0].(mail.Confirm); !ok {
		t.Fatalf("sent %+v, want the confirmation only", sent[0])
	}
}
