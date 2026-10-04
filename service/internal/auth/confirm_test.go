package auth_test

import (
	"context"
	"errors"
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

func TestDeleteExpiredSweepsConfirmations(t *testing.T) {
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
	f.sessions.SetClock(func() time.Time { return now })
	if _, err := f.sessions.Confirm(ctx, token); !errors.Is(err, auth.ErrNoConfirmation) {
		t.Fatalf("swept confirmation still works: %v", err)
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
		if got, err := f.sessions.ConfirmationPending(ctx, u); err != nil || got != want {
			t.Fatalf("%s: pending = %v, %v; want %v", why, got, err, want)
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
