package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/db/dbtest"
	"github.com/s-frei/rezepte/service/internal/user"
)

func newServices(t *testing.T) (*auth.Service, *user.Service) { //nolint:unparam // helper mirrors brief signature; second value documents the seeded user.Service for future tests
	t.Helper()
	conn := dbtest.Open(t)
	users := user.NewService(conn)
	if _, err := users.Create(context.Background(), user.CreateParams{Username: "sam", Password: "pw", Role: user.RoleAdmin}); err != nil {
		t.Fatal(err)
	}
	return auth.NewService(conn, users), users
}

func TestLoginAndAuthenticate(t *testing.T) {
	ctx := context.Background()
	svc, _ := newServices(t)

	sess, err := svc.Login(ctx, "sam", "pw")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if len(sess.Token) < 32 {
		t.Fatalf("token too short: %q", sess.Token)
	}
	if sess.User.Username != "sam" {
		t.Fatalf("session user = %+v", sess.User)
	}
	if until := time.Until(sess.ExpiresAt); until < 29*24*time.Hour {
		t.Fatalf("expires in %v, want ~30d", until)
	}

	v, err := svc.Authenticate(ctx, sess.Token)
	if err != nil || v.User.Username != "sam" {
		t.Fatalf("Authenticate: %+v, %v", v, err)
	}
	// Stored timestamps round-trip through RFC3339 (second precision), so
	// compare with a tolerance rather than requiring exact equality.
	if diff := v.ExpiresAt.Sub(sess.ExpiresAt); diff < -time.Second || diff > time.Second {
		t.Fatalf("ExpiresAt = %v, want ~%v (no renewal due yet)", v.ExpiresAt, sess.ExpiresAt)
	}
	if v.Renewed {
		t.Fatal("Renewed = true right after login, want false")
	}
	if _, err := svc.Authenticate(ctx, "bogus"); !errors.Is(err, auth.ErrNoSession) {
		t.Fatalf("bogus token: err = %v", err)
	}
}

func TestLoginWrongPassword(t *testing.T) {
	svc, _ := newServices(t)
	if _, err := svc.Login(context.Background(), "sam", "nope"); !errors.Is(err, user.ErrInvalidCredentials) {
		t.Fatalf("err = %v", err)
	}
}

func TestLogoutInvalidatesToken(t *testing.T) {
	ctx := context.Background()
	svc, _ := newServices(t)
	sess, _ := svc.Login(ctx, "sam", "pw")
	if err := svc.Logout(ctx, sess.Token); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Authenticate(ctx, sess.Token); !errors.Is(err, auth.ErrNoSession) {
		t.Fatalf("after logout: err = %v", err)
	}
}

func TestExpiredSessionIsRejectedAndSlidingExtends(t *testing.T) {
	ctx := context.Background()
	svc, _ := newServices(t)

	clock := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	svc.SetClock(func() time.Time { return clock })

	sess, _ := svc.Login(ctx, "sam", "pw")

	// 10 days later: still valid and extended to 30 days from now.
	clock = clock.Add(10 * 24 * time.Hour)
	v, err := svc.Authenticate(ctx, sess.Token)
	if err != nil {
		t.Fatalf("day 10: %v", err)
	}
	if !v.Renewed {
		t.Fatal("day 10: Renewed = false, want true")
	}
	if want := clock.Add(auth.SessionTTL); !v.ExpiresAt.Equal(want) {
		t.Fatalf("day 10: ExpiresAt = %v, want %v", v.ExpiresAt, want)
	}
	// 35 days after login (25 after the extension): still valid.
	clock = clock.Add(25 * 24 * time.Hour)
	if _, err := svc.Authenticate(ctx, sess.Token); err != nil {
		t.Fatalf("day 35: %v", err)
	}
	// 31 days of silence: expired.
	clock = clock.Add(31 * 24 * time.Hour)
	if _, err := svc.Authenticate(ctx, sess.Token); !errors.Is(err, auth.ErrNoSession) {
		t.Fatalf("expired: err = %v", err)
	}
}

func TestDeleteUserSessionsExcept(t *testing.T) {
	ctx := context.Background()
	svc, _ := newServices(t)
	keep, err := svc.Login(ctx, "sam", "pw")
	if err != nil {
		t.Fatal(err)
	}
	other, err := svc.Login(ctx, "sam", "pw")
	if err != nil {
		t.Fatal(err)
	}

	if err := svc.DeleteUserSessionsExcept(ctx, keep.User.ID, keep.Token); err != nil {
		t.Fatalf("DeleteUserSessionsExcept: %v", err)
	}
	if _, err := svc.Authenticate(ctx, keep.Token); err != nil {
		t.Fatalf("kept session must still work: %v", err)
	}
	if _, err := svc.Authenticate(ctx, other.Token); !errors.Is(err, auth.ErrNoSession) {
		t.Fatalf("other session: err = %v, want ErrNoSession", err)
	}

	if err := svc.DeleteUserSessionsExcept(ctx, keep.User.ID, ""); err != nil {
		t.Fatalf("delete all: %v", err)
	}
	if _, err := svc.Authenticate(ctx, keep.Token); !errors.Is(err, auth.ErrNoSession) {
		t.Fatalf("after delete all: err = %v, want ErrNoSession", err)
	}
}
