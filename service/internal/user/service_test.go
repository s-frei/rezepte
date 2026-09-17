package user_test

import (
	"context"
	"database/sql"
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

func seedTwo(t *testing.T) (*sql.DB, *user.Service, user.User, user.User) {
	t.Helper()
	ctx := context.Background()
	conn := dbtest.Open(t)
	svc := user.NewService(conn)
	sam, err := svc.Create(ctx, "sam", "password-sam", user.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	kim, err := svc.Create(ctx, "kim", "password-kim", user.RoleUser)
	if err != nil {
		t.Fatal(err)
	}
	return conn, svc, sam, kim
}

func TestListOrdersByUsername(t *testing.T) {
	ctx := context.Background()
	_, svc, _, _ := seedTwo(t)
	if _, err := svc.Create(ctx, "Anna", "password-anna", user.RoleUser); err != nil {
		t.Fatal(err)
	}
	got, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 3 || got[0].Username != "Anna" || got[1].Username != "kim" || got[2].Username != "sam" {
		t.Fatalf("List = %+v", got)
	}
}

func TestSetRoleGuardsLastAdmin(t *testing.T) {
	ctx := context.Background()
	_, svc, sam, kim := seedTwo(t)

	if _, err := svc.SetRole(ctx, sam.ID, user.RoleUser); !errors.Is(err, user.ErrLastAdmin) {
		t.Fatalf("demote only admin: err = %v, want ErrLastAdmin", err)
	}
	promoted, err := svc.SetRole(ctx, kim.ID, user.RoleAdmin)
	if err != nil || promoted.Role != user.RoleAdmin {
		t.Fatalf("promote kim: %+v, %v", promoted, err)
	}
	demoted, err := svc.SetRole(ctx, sam.ID, user.RoleUser)
	if err != nil || demoted.Role != user.RoleUser {
		t.Fatalf("demote sam with a second admin present: %+v, %v", demoted, err)
	}
	if _, err := svc.SetRole(ctx, kim.ID, user.RoleUser); !errors.Is(err, user.ErrLastAdmin) {
		t.Fatalf("demote new last admin: err = %v, want ErrLastAdmin", err)
	}
	if _, err := svc.SetRole(ctx, "missing", user.RoleUser); !errors.Is(err, user.ErrNotFound) {
		t.Fatalf("unknown id: err = %v, want ErrNotFound", err)
	}
}

func TestDeleteGuards(t *testing.T) {
	ctx := context.Background()
	_, svc, sam, kim := seedTwo(t)

	if err := svc.Delete(ctx, sam.ID, sam.ID); !errors.Is(err, user.ErrSelfDelete) {
		t.Fatalf("self delete: err = %v, want ErrSelfDelete", err)
	}
	if err := svc.Delete(ctx, kim.ID, sam.ID); !errors.Is(err, user.ErrLastAdmin) {
		t.Fatalf("delete last admin: err = %v, want ErrLastAdmin", err)
	}
	if err := svc.Delete(ctx, sam.ID, "missing"); !errors.Is(err, user.ErrNotFound) {
		t.Fatalf("unknown id: err = %v, want ErrNotFound", err)
	}
	if err := svc.Delete(ctx, sam.ID, kim.ID); err != nil {
		t.Fatalf("delete member: %v", err)
	}
	if _, err := svc.ByID(ctx, kim.ID); !errors.Is(err, user.ErrNotFound) {
		t.Fatalf("after delete: err = %v, want ErrNotFound", err)
	}
}

func TestDeleteReassignsRecipes(t *testing.T) {
	ctx := context.Background()
	conn, svc, sam, kim := seedTwo(t)
	_, err := conn.ExecContext(ctx, `INSERT INTO recipes (id, slug, title, description, servings, created_by, created_at, updated_at)
		VALUES ('r1', 'r1', 'Kims Rezept', '', 4, ?, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`, kim.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(ctx, sam.ID, kim.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	var owner string
	if err := conn.QueryRowContext(ctx, `SELECT created_by FROM recipes WHERE id = 'r1'`).Scan(&owner); err != nil {
		t.Fatal(err)
	}
	if owner != sam.ID {
		t.Fatalf("created_by = %q, want %q (the acting admin)", owner, sam.ID)
	}
}

func TestSetAndChangePassword(t *testing.T) {
	ctx := context.Background()
	_, svc, _, kim := seedTwo(t)

	if err := svc.SetPassword(ctx, kim.ID, "reset-by-admin"); err != nil {
		t.Fatalf("SetPassword: %v", err)
	}
	if _, err := svc.Authenticate(ctx, "kim", "reset-by-admin"); err != nil {
		t.Fatalf("login with reset password: %v", err)
	}
	if err := svc.SetPassword(ctx, "missing", "whatever-pw"); !errors.Is(err, user.ErrNotFound) {
		t.Fatalf("SetPassword unknown: err = %v, want ErrNotFound", err)
	}

	if err := svc.ChangePassword(ctx, kim.ID, "wrong", "chosen-by-kim"); !errors.Is(err, user.ErrWrongPassword) {
		t.Fatalf("wrong current: err = %v, want ErrWrongPassword", err)
	}
	if err := svc.ChangePassword(ctx, kim.ID, "reset-by-admin", "chosen-by-kim"); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}
	if _, err := svc.Authenticate(ctx, "kim", "chosen-by-kim"); err != nil {
		t.Fatalf("login with changed password: %v", err)
	}
	if _, err := svc.Authenticate(ctx, "kim", "reset-by-admin"); !errors.Is(err, user.ErrInvalidCredentials) {
		t.Fatalf("old password still works: err = %v", err)
	}
	if err := svc.ChangePassword(ctx, "missing", "x", "chosen-by-kim"); !errors.Is(err, user.ErrNotFound) {
		t.Fatalf("ChangePassword unknown: err = %v, want ErrNotFound", err)
	}
}
