package demo

import (
	"bytes"
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log/slog"

	"github.com/s-frei/rezepte/service/internal/avatar"
	"github.com/s-frei/rezepte/service/internal/user"
)

// The embedded avatars are derived: `mise run demo-avatars` scales the
// originals in assets/demo-avatars/seed down to 640 px.
//
//go:embed avatars
var avatarFiles embed.FS

// avatarUsers are the demo people who get a picture. jonas is left out on
// purpose: the demo then shows the color-and-initial fallback, and a manual
// test has an account to set a picture on. His picture is embedded all the
// same, so adding him is one word here.
var avatarUsers = []string{AdminUser, "mila"}

// SeedAvatars gives avatarUsers their pictures through the same service the
// API uses. An account that is missing or already has a picture is left
// alone, so a second run changes nothing.
func SeedAvatars(ctx context.Context, conn *sql.DB, avatars *avatar.Service, logger *slog.Logger) error {
	list, err := user.NewService(conn, "").List(ctx)
	if err != nil {
		return fmt.Errorf("list users: %w", err)
	}
	byName := make(map[string]user.User, len(list))
	for _, u := range list {
		byName[u.Username] = u
	}
	n := 0
	for _, name := range avatarUsers {
		u, ok := byName[name]
		if !ok || u.AvatarID != nil {
			continue
		}
		data, err := avatarFiles.ReadFile("avatars/" + name + ".jpg")
		if err != nil {
			return fmt.Errorf("read avatar of %s: %w", name, err)
		}
		if _, err := avatars.Set(ctx, u.ID, bytes.NewReader(data), nil); err != nil {
			return fmt.Errorf("set avatar of %s: %w", name, err)
		}
		n++
	}
	logger.Info("demo: avatars set", "count", n)
	return nil
}
