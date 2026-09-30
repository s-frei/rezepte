package demo_test

import (
	"context"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/s-frei/rezepte/service/internal/avatar"
	"github.com/s-frei/rezepte/service/internal/db/dbtest"
	"github.com/s-frei/rezepte/service/internal/demo"
	"github.com/s-frei/rezepte/service/internal/image"
	"github.com/s-frei/rezepte/service/internal/user"
)

func TestSeedAvatarsSkipsJonas(t *testing.T) {
	ctx := context.Background()
	conn := dbtest.Open(t)
	users := user.NewService(conn, "")
	if err := users.EnsureSuperadmin(ctx, demo.AdminUser, demo.AdminPassword); err != nil {
		t.Fatal(err)
	}
	if _, err := demo.AddMembers(ctx, conn, "en", slog.New(slog.DiscardHandler)); err != nil {
		t.Fatal(err)
	}
	avatars := avatar.NewService(conn, filepath.Join(t.TempDir(), "avatars"), image.NewService(conn, t.TempDir()))
	if err := demo.SeedAvatars(ctx, conn, avatars, slog.New(slog.DiscardHandler)); err != nil {
		t.Fatal(err)
	}
	list, _ := users.List(ctx)
	got := map[string]bool{}
	for _, u := range list {
		got[u.Username] = u.AvatarID != nil
	}
	want := map[string]bool{demo.AdminUser: true, "mila": true, "jonas": false}
	for name, has := range want {
		if got[name] != has {
			t.Errorf("%s has picture = %v, want %v", name, got[name], has)
		}
	}
}
