package settings_test

import (
	"bytes"
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
	if got.LinkPreviews {
		t.Error("LinkPreviews = true on a fresh instance, want false")
	}
	if got.LinkPreviewMinutes != 15 {
		t.Errorf("LinkPreviewMinutes = %d on a fresh instance, want 15", got.LinkPreviewMinutes)
	}
}

func TestOnlyTheOwnerSetsLinkPreviews(t *testing.T) {
	ctx := context.Background()
	svc := settings.NewService(dbtest.Open(t))
	admin := user.User{ID: "a", Role: user.RoleAdmin}
	if _, err := svc.SetLinkPreviews(ctx, admin, true); !errors.Is(err, settings.ErrOwnerRequired) {
		t.Errorf("admin switch: err = %v, want ErrOwnerRequired", err)
	}
	if _, err := svc.SetLinkPreviewMinutes(ctx, admin, 60); !errors.Is(err, settings.ErrOwnerRequired) {
		t.Errorf("admin lifetime: err = %v, want ErrOwnerRequired", err)
	}
	owner := user.User{ID: "o", Role: user.RoleSuperadmin}
	if _, err := svc.SetLinkPreviewMinutes(ctx, owner, 30); !errors.Is(err, settings.ErrUnknownLifetime) {
		t.Errorf("30 minutes: err = %v, want ErrUnknownLifetime", err)
	}
	if _, err := svc.SetLinkPreviews(ctx, owner, true); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetLinkPreviewMinutes(ctx, owner, 1440); err != nil {
		t.Fatal(err)
	}
	got, err := svc.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !got.LinkPreviews || got.LinkPreviewMinutes != 1440 {
		t.Errorf("got %+v, want previews on for 1440 minutes", got)
	}
}

func TestLinkPreviewKeyIsStable(t *testing.T) {
	ctx := context.Background()
	conn := dbtest.Open(t)
	first, err := settings.NewService(conn).LinkPreviewKey(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 32 {
		t.Fatalf("key has %d bytes, want 32", len(first))
	}
	second, err := settings.NewService(conn).LinkPreviewKey(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Error("a second service read a different key")
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
