package auth_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/user"
)

func withEmail(t *testing.T, f setupFixture, u user.User, addr string) {
	t.Helper()
	if _, err := f.users.SetProfile(context.Background(), u.ID, user.ProfileUpdate{Email: &addr}); err != nil {
		t.Fatal(err)
	}
}

func TestConfirmVerifiesTheAddress(t *testing.T) {
	ctx := context.Background()
	f := newSetupFixture(t)
	withEmail(t, f, f.member, "anna@example.org")
	token, err := f.sessions.IssueConfirmation(ctx, f.member.ID, "anna@example.org")
	if err != nil {
		t.Fatal(err)
	}
	addr, err := f.sessions.Confirm(ctx, token)
	if err != nil || addr != "anna@example.org" {
		t.Fatalf("Confirm = %q, %v", addr, err)
	}
	if got, _ := f.users.ByID(ctx, f.member.ID); !got.EmailVerified {
		t.Fatal("address not verified")
	}
	if _, err := f.sessions.Confirm(ctx, token); !errors.Is(err, auth.ErrNoConfirmation) {
		t.Fatalf("second use: err = %v", err)
	}
}

func TestConfirmRefusesAChangedAddress(t *testing.T) {
	ctx := context.Background()
	f := newSetupFixture(t)
	withEmail(t, f, f.member, "anna@example.org")
	token, _ := f.sessions.IssueConfirmation(ctx, f.member.ID, "anna@example.org")
	withEmail(t, f, f.member, "anna@other.org")
	if _, err := f.sessions.Confirm(ctx, token); !errors.Is(err, auth.ErrNoConfirmation) {
		t.Fatalf("err = %v, want ErrNoConfirmation", err)
	}
	if got, _ := f.users.ByID(ctx, f.member.ID); got.EmailVerified {
		t.Fatal("the new address was verified by the old link")
	}
	// The stale token is used up: the row is gone, a second try fails alike.
	if _, err := f.sessions.Confirm(ctx, token); !errors.Is(err, auth.ErrNoConfirmation) {
		t.Fatalf("second try: err = %v", err)
	}
	var n int
	if err := f.conn.QueryRow(`SELECT count(*) FROM email_confirmations`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("rows left = %d, %v", n, err)
	}
}

func TestConfirmRefusesUnknownExpiredAndReplaced(t *testing.T) {
	ctx := context.Background()
	f := newSetupFixture(t)
	withEmail(t, f, f.member, "anna@example.org")
	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	f.sessions.SetClock(func() time.Time { return now })
	if _, err := f.sessions.Confirm(ctx, "nope"); !errors.Is(err, auth.ErrNoConfirmation) {
		t.Fatalf("unknown: %v", err)
	}
	first, _ := f.sessions.IssueConfirmation(ctx, f.member.ID, "anna@example.org")
	second, _ := f.sessions.IssueConfirmation(ctx, f.member.ID, "anna@example.org")
	if _, err := f.sessions.Confirm(ctx, first); !errors.Is(err, auth.ErrNoConfirmation) {
		t.Fatalf("replaced: %v", err)
	}
	f.sessions.SetClock(func() time.Time { return now.Add(auth.ConfirmationTTL + time.Second) })
	if _, err := f.sessions.Confirm(ctx, second); !errors.Is(err, auth.ErrNoConfirmation) {
		t.Fatalf("expired: %v", err)
	}
}

// One row per account is kept past its expiry, so the email row can say the
// link ran out; Confirm still refuses it.
func TestDeleteExpiredKeepsConfirmations(t *testing.T) {
	ctx := context.Background()
	f := newSetupFixture(t)
	withEmail(t, f, f.member, "anna@example.org")
	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	f.sessions.SetClock(func() time.Time { return now })
	token, _ := f.sessions.IssueConfirmation(ctx, f.member.ID, "anna@example.org")
	f.sessions.SetClock(func() time.Time { return now.Add(auth.ConfirmationTTL + time.Hour) })
	if err := f.sessions.DeleteExpired(ctx); err != nil {
		t.Fatal(err)
	}
	if st := mailStatus(t, f); st.Confirmation == nil {
		t.Fatal("expired confirmation was swept")
	}
	if _, err := f.sessions.Confirm(ctx, token); !errors.Is(err, auth.ErrNoConfirmation) {
		t.Fatalf("expired confirmation still works: %v", err)
	}
}

func mailStatus(t *testing.T, f setupFixture) auth.MailStatus {
	t.Helper()
	u, err := f.users.ByID(context.Background(), f.member.ID)
	if err != nil {
		t.Fatal(err)
	}
	st, err := f.sessions.MailStatus(context.Background(), u)
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func TestMailStatusOpenConfirmation(t *testing.T) {
	f := newSetupFixture(t)
	now := time.Date(2026, 11, 8, 14, 3, 0, 0, time.UTC)
	f.sessions.SetClock(func() time.Time { return now })
	withEmail(t, f, f.member, "anna@example.org")
	if _, err := f.sessions.IssueConfirmation(context.Background(), f.member.ID, "anna@example.org"); err != nil {
		t.Fatal(err)
	}
	st := mailStatus(t, f)
	if st.Confirmation == nil || !st.Confirmation.SentAt.Equal(now) || !st.Confirmation.ExpiresAt.Equal(now.Add(auth.ConfirmationTTL)) {
		t.Fatalf("confirmation = %+v", st.Confirmation)
	}
	if st.SetupLink != nil {
		t.Fatalf("setup link = %+v", st.SetupLink)
	}
}

func TestMailStatusExpiredConfirmation(t *testing.T) {
	f := newSetupFixture(t)
	now := time.Date(2026, 11, 8, 14, 3, 0, 0, time.UTC)
	f.sessions.SetClock(func() time.Time { return now })
	withEmail(t, f, f.member, "anna@example.org")
	_, _ = f.sessions.IssueConfirmation(context.Background(), f.member.ID, "anna@example.org")
	f.sessions.SetClock(func() time.Time { return now.Add(auth.ConfirmationTTL + time.Hour) })
	st := mailStatus(t, f)
	if st.Confirmation == nil || !st.Confirmation.ExpiresAt.Before(now.Add(auth.ConfirmationTTL+time.Hour)) {
		t.Fatalf("confirmation = %+v", st.Confirmation)
	}
}

func TestMailStatusVerifiedIsEmpty(t *testing.T) {
	ctx := context.Background()
	f := newSetupFixture(t)
	withEmail(t, f, f.member, "anna@example.org")
	token, _ := f.sessions.IssueConfirmation(ctx, f.member.ID, "anna@example.org")
	if _, err := f.sessions.Confirm(ctx, token); err != nil {
		t.Fatal(err)
	}
	if st := mailStatus(t, f); st.Confirmation != nil || st.SetupLink != nil {
		t.Fatalf("status = %+v", st)
	}
}

func TestMailStatusAfterSwallowedSave(t *testing.T) {
	f := newSetupFixture(t)
	withEmail(t, f, f.member, "anna@example.org")
	_, _ = f.sessions.IssueConfirmation(context.Background(), f.member.ID, "anna@example.org")
	withEmail(t, f, f.member, "anna@other.org")
	if st := mailStatus(t, f); st.Confirmation != nil {
		t.Fatalf("confirmation for the old address shown: %+v", st.Confirmation)
	}
}

func TestMailStatusSetupLinkForOtherAddress(t *testing.T) {
	ctx := context.Background()
	f := newSetupFixture(t)
	now := time.Date(2026, 11, 8, 14, 3, 0, 0, time.UTC)
	f.sessions.SetClock(func() time.Time { return now })
	withEmail(t, f, f.member, "anna@example.org")
	link, err := f.sessions.IssueSetupLink(ctx, f.admin, f.member.ID)
	if err != nil {
		t.Fatal(err)
	}
	// Redeeming confirms only the address exactly as it was mailed to
	// (VerifyEmailIfMatches), so a link to other letters is no setup link
	// for this one: the row offers a confirmation instead.
	if err := f.sessions.MarkSetupLinkSent(ctx, link.Token, "Anna@Example.org"); err != nil {
		t.Fatal(err)
	}
	if st := mailStatus(t, f); st.SetupLink != nil {
		t.Fatalf("setup link mailed to other letters shown: %+v", st.SetupLink)
	}
	if err := f.sessions.MarkSetupLinkSent(ctx, link.Token, "anna@example.org"); err != nil {
		t.Fatal(err)
	}
	st := mailStatus(t, f)
	if st.SetupLink == nil || !st.SetupLink.ExpiresAt.Equal(link.ExpiresAt) {
		t.Fatalf("setup link mailed to the address: %+v", st.SetupLink)
	}
	withEmail(t, f, f.member, "anna@other.org")
	if st := mailStatus(t, f); st.SetupLink != nil {
		t.Fatalf("setup link mailed to the old address shown: %+v", st.SetupLink)
	}
}

func TestMailStatusesMatchesMailStatus(t *testing.T) {
	ctx := context.Background()
	f := newSetupFixture(t)
	withEmail(t, f, f.member, "anna@example.org")
	_, _ = f.sessions.IssueConfirmation(ctx, f.member.ID, "anna@example.org")
	withEmail(t, f, f.admin, "sam@example.org")
	link, _ := f.sessions.IssueSetupLink(ctx, f.owner, f.admin.ID)
	_ = f.sessions.MarkSetupLinkSent(ctx, link.Token, "sam@example.org")
	var list []user.User
	for _, id := range []string{f.owner.ID, f.admin.ID, f.member.ID} {
		u, err := f.users.ByID(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		list = append(list, u)
	}
	all, err := f.sessions.MailStatuses(ctx, list)
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range list {
		one, err := f.sessions.MailStatus(ctx, u)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(all[u.ID], one) {
			t.Fatalf("%s: MailStatuses = %+v, MailStatus = %+v", u.Username, all[u.ID], one)
		}
	}
	if all[f.member.ID].Confirmation == nil || all[f.admin.ID].SetupLink == nil {
		t.Fatalf("statuses = %+v", all)
	}
}

func TestConfirmationPending(t *testing.T) {
	ctx := context.Background()
	f := newSetupFixture(t)
	pending := func(want bool, why string) {
		t.Helper()
		u, err := f.users.ByID(ctx, f.member.ID)
		if err != nil {
			t.Fatal(err)
		}
		if st, err := f.sessions.MailStatus(ctx, u); err != nil || f.sessions.ConfirmationPending(st) != want {
			t.Fatalf("%s: pending = %v, %v; want %v", why, !want, err, want)
		}
	}
	withEmail(t, f, f.member, "anna@example.org")
	pending(false, "no link issued")
	token, err := f.sessions.IssueConfirmation(ctx, f.member.ID, "anna@example.org")
	if err != nil {
		t.Fatal(err)
	}
	pending(true, "open link")
	withEmail(t, f, f.member, "other@example.org")
	pending(false, "address changed since")
	withEmail(t, f, f.member, "anna@example.org")
	now := time.Now()
	f.sessions.SetClock(func() time.Time { return now.Add(auth.ConfirmationTTL + time.Minute) })
	pending(false, "expired")
	f.sessions.SetClock(time.Now)
	if _, err := f.sessions.Confirm(ctx, token); err != nil {
		t.Fatal(err)
	}
	pending(false, "confirmed")
}
