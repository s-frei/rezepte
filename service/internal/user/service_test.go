package user_test

import (
	"context"
	"errors"
	"testing"

	"github.com/s-frei/rezepte/service/internal/db/dbtest"
	"github.com/s-frei/rezepte/service/internal/user"
)

func TestCreateAndAuthenticate(t *testing.T) {
	ctx := context.Background()
	svc := user.NewService(dbtest.Open(t))

	created, err := svc.Create(ctx, "Sam", "secret123", user.RoleAdmin)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID == "" || created.Username != "Sam" || created.Role != user.RoleAdmin {
		t.Fatalf("unexpected user: %+v", created)
	}

	got, err := svc.Authenticate(ctx, "sam", "secret123") // username is case-insensitive
	if err != nil || got.ID != created.ID {
		t.Fatalf("Authenticate: %+v, %v", got, err)
	}
	if _, err := svc.Authenticate(ctx, "sam", "nope"); !errors.Is(err, user.ErrInvalidCredentials) {
		t.Fatalf("wrong password: err = %v", err)
	}
	if _, err := svc.Authenticate(ctx, "nobody", "x"); !errors.Is(err, user.ErrInvalidCredentials) {
		t.Fatalf("unknown user: err = %v", err)
	}
}

func TestCreateRejectsDuplicateUsername(t *testing.T) {
	ctx := context.Background()
	svc := user.NewService(dbtest.Open(t))
	if _, err := svc.Create(ctx, "sam", "x", user.RoleUser); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(ctx, "SAM", "y", user.RoleUser); !errors.Is(err, user.ErrUsernameTaken) {
		t.Fatalf("err = %v, want ErrUsernameTaken", err)
	}
}

func TestByIDNotFound(t *testing.T) {
	svc := user.NewService(dbtest.Open(t))
	if _, err := svc.ByID(context.Background(), "missing"); !errors.Is(err, user.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestEnsureInitialAdmin(t *testing.T) {
	ctx := context.Background()
	svc := user.NewService(dbtest.Open(t))

	if err := svc.EnsureInitialAdmin(ctx, "admin", ""); !errors.Is(err, user.ErrAdminPasswordRequired) {
		t.Fatalf("empty password: err = %v", err)
	}
	if err := svc.EnsureInitialAdmin(ctx, "admin", "pw"); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := svc.Authenticate(ctx, "admin", "pw"); err != nil {
		t.Fatalf("admin cannot log in: %v", err)
	}
	// Once users exist the environment is ignored, even without a password.
	if err := svc.EnsureInitialAdmin(ctx, "other", ""); err != nil {
		t.Fatalf("second call: %v", err)
	}
	if _, err := svc.Authenticate(ctx, "other", ""); !errors.Is(err, user.ErrInvalidCredentials) {
		t.Fatal("second admin must not be created")
	}
}
