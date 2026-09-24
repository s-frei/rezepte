package settings_test

import (
	"context"
	"errors"
	"testing"

	"github.com/s-frei/rezepte/service/internal/db/dbtest"
	"github.com/s-frei/rezepte/service/internal/settings"
	"github.com/s-frei/rezepte/service/internal/user"
)

func TestDefaultsToOpen(t *testing.T) {
	svc := settings.NewService(dbtest.Open(t))
	got, err := svc.Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.RecipesLockedByDefault {
		t.Error("RecipesLockedByDefault = true on a fresh instance, want false")
	}
}

func TestOnlyTheOwnerSetsTheLock(t *testing.T) {
	ctx := context.Background()
	svc := settings.NewService(dbtest.Open(t))
	for _, role := range []user.Role{user.RoleUser, user.RoleAdmin} {
		_, err := svc.SetRecipesLockedByDefault(ctx, user.User{ID: "x", Role: role}, true)
		if !errors.Is(err, settings.ErrOwnerRequired) {
			t.Errorf("%s: err = %v, want ErrOwnerRequired", role, err)
		}
	}
	got, err := svc.SetRecipesLockedByDefault(ctx, user.User{ID: "o", Role: user.RoleSuperadmin}, true)
	if err != nil {
		t.Fatal(err)
	}
	if !got.RecipesLockedByDefault {
		t.Error("owner write did not land")
	}
	again, err := svc.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !again.RecipesLockedByDefault {
		t.Error("Get after the owner's write = false, want true")
	}
}
