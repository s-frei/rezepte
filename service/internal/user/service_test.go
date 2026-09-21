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

func TestCreateTrimsUsername(t *testing.T) {
	ctx := context.Background()
	svc := user.NewService(dbtest.Open(t))

	if _, err := svc.Create(ctx, "  ", "secret123", user.RoleUser); !errors.Is(err, user.ErrInvalidUsername) {
		t.Fatalf("blank username: err = %v, want ErrInvalidUsername", err)
	}

	created, err := svc.Create(ctx, " anna ", "secret123", user.RoleUser)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.Username != "anna" {
		t.Fatalf("username = %q, want trimmed %q", created.Username, "anna")
	}

	// Authenticate trims the same way, so surrounding spaces typed by
	// mistake at login still resolve to the trimmed account.
	got, err := svc.Authenticate(ctx, " anna ", "secret123")
	if err != nil || got.ID != created.ID {
		t.Fatalf("Authenticate with spaces: %+v, %v", got, err)
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

func TestEnsureSuperadmin(t *testing.T) {
	ctx := context.Background()

	t.Run("creates the owner on an empty instance", func(t *testing.T) {
		svc := user.NewService(dbtest.Open(t))
		if err := svc.EnsureSuperadmin(ctx, "boss", "secret123"); err != nil {
			t.Fatalf("EnsureSuperadmin: %v", err)
		}
		got, err := svc.Authenticate(ctx, "boss", "secret123")
		if err != nil || got.Role != user.RoleSuperadmin {
			t.Fatalf("owner = %+v, %v", got, err)
		}
	})

	t.Run("is a no-op once an owner exists", func(t *testing.T) {
		svc := user.NewService(dbtest.Open(t))
		if err := svc.EnsureSuperadmin(ctx, "boss", "secret123"); err != nil {
			t.Fatal(err)
		}
		// REZEPTE_ADMIN_PASSWORD has no default, and restarting an existing
		// instance without it is supported: the second call is a no-op on the
		// strength of the existing owner alone, before any password check.
		if err := svc.EnsureSuperadmin(ctx, "other", ""); err != nil {
			t.Fatalf("second call: %v", err)
		}
		list, err := svc.List(ctx)
		if err != nil || len(list) != 1 {
			t.Fatalf("users = %d (%v), want 1", len(list), err)
		}
	})

	t.Run("needs a password on an empty instance", func(t *testing.T) {
		svc := user.NewService(dbtest.Open(t))
		if err := svc.EnsureSuperadmin(ctx, "boss", ""); !errors.Is(err, user.ErrAdminPasswordRequired) {
			t.Fatalf("err = %v, want ErrAdminPasswordRequired", err)
		}
	})

	t.Run("refuses a populated instance with no owner", func(t *testing.T) {
		svc := user.NewService(dbtest.Open(t))
		if _, err := svc.Create(ctx, "stray", "secret123", user.RoleAdmin); err != nil {
			t.Fatal(err)
		}
		if err := svc.EnsureSuperadmin(ctx, "boss", "secret123"); !errors.Is(err, user.ErrNoSuperadmin) {
			t.Fatalf("err = %v, want ErrNoSuperadmin", err)
		}
	})
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

func TestDeleteGuards(t *testing.T) {
	ctx := context.Background()
	_, svc, sam, kim := seedTwo(t)

	if err := svc.Delete(ctx, sam, sam.ID); !errors.Is(err, user.ErrSelfDelete) {
		t.Fatalf("self delete: err = %v, want ErrSelfDelete", err)
	}
	if err := svc.Delete(ctx, sam, "missing"); !errors.Is(err, user.ErrNotFound) {
		t.Fatalf("unknown id: err = %v, want ErrNotFound", err)
	}
	if err := svc.Delete(ctx, sam, kim.ID); err != nil {
		t.Fatalf("delete member: %v", err)
	}
	if _, err := svc.ByID(ctx, kim.ID); !errors.Is(err, user.ErrNotFound) {
		t.Fatalf("after delete: err = %v, want ErrNotFound", err)
	}
}

func TestDeleteReassignsRecipes(t *testing.T) {
	ctx := context.Background()
	conn, svc, sam, kim := seedTwo(t)
	_, err := conn.ExecContext(ctx, `INSERT INTO recipes (id, slug, title, description, servings, created_by, created_at, updated_by, updated_at)
		VALUES ('r1', 'r1', 'Kims Rezept', '', 4, ?, '2026-01-01T00:00:00Z', ?, '2026-01-01T00:00:00Z')`, kim.ID, kim.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(ctx, sam, kim.ID); err != nil {
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

// A recipe somebody else wrote but the deleted user last edited holds their
// id in updated_by alone. That column is NOT NULL and references users too,
// so leaving it behind would fail the delete on the foreign key.
func TestDeleteReassignsRecipesEditedByTheDeletedUser(t *testing.T) {
	ctx := context.Background()
	conn, svc, sam, kim := seedTwo(t)
	_, err := conn.ExecContext(ctx, `INSERT INTO recipes (id, slug, title, description, servings, created_by, created_at, updated_by, updated_at)
		VALUES ('r1', 'r1', 'Sams Rezept', '', 4, ?, '2026-01-01T00:00:00Z', ?, '2026-01-02T00:00:00Z')`, sam.ID, kim.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(ctx, sam, kim.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	var author, editor string
	if err := conn.QueryRowContext(ctx, `SELECT created_by, updated_by FROM recipes WHERE id = 'r1'`).Scan(&author, &editor); err != nil {
		t.Fatal(err)
	}
	if editor != sam.ID {
		t.Fatalf("updated_by = %q, want %q (the acting admin)", editor, sam.ID)
	}
	if author != sam.ID {
		t.Fatalf("created_by = %q, want the original author %q", author, sam.ID)
	}
}

func TestSetAndChangePassword(t *testing.T) {
	ctx := context.Background()
	_, svc, sam, kim := seedTwo(t)

	if err := svc.SetPassword(ctx, sam, kim.ID, "reset-by-admin"); err != nil {
		t.Fatalf("SetPassword: %v", err)
	}
	if _, err := svc.Authenticate(ctx, "kim", "reset-by-admin"); err != nil {
		t.Fatalf("login with reset password: %v", err)
	}
	if err := svc.SetPassword(ctx, sam, "missing", "whatever-pw"); !errors.Is(err, user.ErrNotFound) {
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

// seedRanks returns a service holding one superadmin, two admins and one
// member, plus the four users, so the rank tests read as a matrix.
func seedRanks(t *testing.T) (*user.Service, map[string]user.User) {
	t.Helper()
	ctx := context.Background()
	svc := user.NewService(dbtest.Open(t))
	out := map[string]user.User{}
	for _, seed := range []struct {
		name string
		role user.Role
	}{
		{"owner", user.RoleSuperadmin},
		{"ada", user.RoleAdmin},
		{"bob", user.RoleAdmin},
		{"kim", user.RoleUser},
	} {
		u, err := svc.Create(ctx, seed.name, "secret123", seed.role)
		if err != nil {
			t.Fatalf("seed %s: %v", seed.name, err)
		}
		out[seed.name] = u
	}
	return svc, out
}

func TestNobodyDeletesTheSuperadmin(t *testing.T) {
	ctx := context.Background()
	svc, u := seedRanks(t)
	for _, actor := range []string{"ada", "kim"} {
		if err := svc.Delete(ctx, u[actor], u["owner"].ID); !errors.Is(err, user.ErrSuperadminProtected) {
			t.Errorf("%s deleting the owner: err = %v, want ErrSuperadminProtected", actor, err)
		}
	}
	// Not even the owner, and not through the self-delete refusal either:
	// Delete returns ErrSelfDelete first, so aim another superadmin-shaped
	// call at the row instead.
	if _, err := svc.SetRole(ctx, u["owner"], u["owner"].ID, user.RoleAdmin); !errors.Is(err, user.ErrSuperadminProtected) {
		t.Errorf("owner demoting themselves: err = %v, want ErrSuperadminProtected", err)
	}
	if err := svc.SetPassword(ctx, u["owner"], u["owner"].ID, "newsecret1"); !errors.Is(err, user.ErrSuperadminProtected) {
		t.Errorf("owner resetting themselves: err = %v, want ErrSuperadminProtected", err)
	}
}

func TestAdminsCannotManageEachOther(t *testing.T) {
	ctx := context.Background()
	svc, u := seedRanks(t)
	if err := svc.Delete(ctx, u["ada"], u["bob"].ID); !errors.Is(err, user.ErrSuperadminRequired) {
		t.Errorf("admin deleting an admin: err = %v, want ErrSuperadminRequired", err)
	}
	if _, err := svc.SetRole(ctx, u["ada"], u["bob"].ID, user.RoleUser); !errors.Is(err, user.ErrSuperadminRequired) {
		t.Errorf("admin demoting an admin: err = %v, want ErrSuperadminRequired", err)
	}
	if err := svc.SetPassword(ctx, u["ada"], u["bob"].ID, "newsecret1"); !errors.Is(err, user.ErrSuperadminRequired) {
		t.Errorf("admin resetting an admin: err = %v, want ErrSuperadminRequired", err)
	}
	if _, err := svc.SetRole(ctx, u["ada"], u["kim"].ID, user.RoleAdmin); !errors.Is(err, user.ErrSuperadminRequired) {
		t.Errorf("admin promoting a member: err = %v, want ErrSuperadminRequired", err)
	}
}

func TestAdminManagesMembersAndSuperadminManagesAdmins(t *testing.T) {
	ctx := context.Background()
	svc, u := seedRanks(t)
	if err := svc.SetPassword(ctx, u["ada"], u["kim"].ID, "newsecret1"); err != nil {
		t.Fatalf("admin resetting a member: %v", err)
	}
	if _, err := svc.Authenticate(ctx, "kim", "newsecret1"); err != nil {
		t.Fatalf("member logs in with the reset password: %v", err)
	}
	promoted, err := svc.SetRole(ctx, u["owner"], u["kim"].ID, user.RoleAdmin)
	if err != nil || promoted.Role != user.RoleAdmin {
		t.Fatalf("owner promoting a member: %+v, %v", promoted, err)
	}
	if err := svc.Delete(ctx, u["owner"], u["bob"].ID); err != nil {
		t.Fatalf("owner deleting an admin: %v", err)
	}
	if err := svc.Delete(ctx, u["ada"], u["kim"].ID); !errors.Is(err, user.ErrSuperadminRequired) {
		t.Fatalf("admin deleting the freshly promoted admin: err = %v, want ErrSuperadminRequired", err)
	}
}

func TestSuperadminChangesTheirOwnPassword(t *testing.T) {
	ctx := context.Background()
	svc, u := seedRanks(t)
	if err := svc.ChangePassword(ctx, u["owner"].ID, "secret123", "newsecret1"); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}
	if _, err := svc.Authenticate(ctx, "owner", "newsecret1"); err != nil {
		t.Fatalf("owner logs in with the new password: %v", err)
	}
}

func TestResetSuperadminPassword(t *testing.T) {
	ctx := context.Background()
	svc := user.NewService(dbtest.Open(t))
	if err := svc.EnsureSuperadmin(ctx, "boss", "secret123"); err != nil {
		t.Fatal(err)
	}

	owner, err := svc.ResetSuperadminPassword(ctx, "newsecret1")
	if err != nil || owner.Username != "boss" {
		t.Fatalf("ResetSuperadminPassword: %+v, %v", owner, err)
	}
	if _, err := svc.Authenticate(ctx, "boss", "newsecret1"); err != nil {
		t.Fatalf("login with the reset password: %v", err)
	}
	if _, err := svc.Authenticate(ctx, "boss", "secret123"); !errors.Is(err, user.ErrInvalidCredentials) {
		t.Fatalf("old password still works: %v", err)
	}
	if _, err := svc.ResetSuperadminPassword(ctx, ""); !errors.Is(err, user.ErrAdminPasswordRequired) {
		t.Fatalf("empty password: err = %v, want ErrAdminPasswordRequired", err)
	}

	empty := user.NewService(dbtest.Open(t))
	if _, err := empty.ResetSuperadminPassword(ctx, "newsecret1"); !errors.Is(err, user.ErrNoSuperadmin) {
		t.Fatalf("no owner: err = %v, want ErrNoSuperadmin", err)
	}
}
