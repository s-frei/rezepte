// Package avatar keeps each account's picture: one square JPEG per account
// under <dir>/<user id>/<avatar id>.jpg, rendered by package image, with
// users.avatar_id naming the current one. A new upload gets a new id, so a
// changed picture is a changed URL and the file can be cached for a year.
package avatar

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
	"uuid"

	"github.com/s-frei/rezepte/service/internal/db"
	"github.com/s-frei/rezepte/service/internal/db/sqlc"
	"github.com/s-frei/rezepte/service/internal/image"
)

// ErrNotFound is an account that does not exist.
var ErrNotFound = errors.New("user not found")

// Service stores and serves account pictures.
type Service struct {
	conn   *sql.DB
	dir    string
	images *image.Service
	now    func() time.Time
}

// NewService returns a Service writing below dir. images renders the
// pictures, so an avatar upload waits for the same decode slots a recipe
// photo does and the two together stay within one memory bound.
func NewService(conn *sql.DB, dir string, images *image.Service) *Service {
	return &Service{conn: conn, dir: dir, images: images, now: time.Now}
}

func (s *Service) path(userID, avatarID string) string {
	return filepath.Join(s.dir, userID, avatarID+".jpg")
}

// Set renders the upload in r as userID's picture and returns its new id.
// The file is written before the row changes and the previous file removed
// only after the row names the new one, so a failure at any step leaves the
// previous picture in place and a reader never follows the row to a
// missing file. Errors: ErrNotFound, image.ErrUnsupported,
// image.ErrInvalid, image.ErrTooLarge, image.ErrBadCrop.
func (s *Service) Set(ctx context.Context, userID string, r io.Reader, c *image.Crop) (string, error) {
	if !image.IsID(userID) {
		return "", ErrNotFound
	}
	// Cheap check before decoding a possibly 10 MiB body.
	if _, err := sqlc.New(s.conn).GetUserByID(ctx, userID); errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	} else if err != nil {
		return "", fmt.Errorf("load user: %w", err)
	}
	id := uuid.NewV7().String()
	p := s.path(userID, id)
	if err := s.images.RenderAvatar(ctx, r, c, p); err != nil {
		return "", err
	}
	old, err := s.swap(ctx, userID, &id)
	if err != nil {
		_ = os.Remove(p)
		return "", err
	}
	if old != nil {
		_ = os.Remove(s.path(userID, *old))
	}
	return id, nil
}

// Remove clears userID's picture and deletes its file. Removing a picture
// that is not there is not an error.
func (s *Service) Remove(ctx context.Context, userID string) error {
	if !image.IsID(userID) {
		return ErrNotFound
	}
	old, err := s.swap(ctx, userID, nil)
	if err != nil {
		return err
	}
	if old != nil {
		_ = os.Remove(s.path(userID, *old))
	}
	return nil
}

// swap points users.avatar_id at next and returns what it named before.
func (s *Service) swap(ctx context.Context, userID string, next *string) (*string, error) {
	var old *string
	err := db.Tx(ctx, s.conn, func(q *sqlc.Queries) error {
		row, err := q.GetUserByID(ctx, userID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("load user: %w", err)
		}
		old = row.AvatarID
		if _, err := q.SetUserAvatar(ctx, sqlc.SetUserAvatarParams{AvatarID: next, UpdatedAt: db.FormatTime(s.now()), ID: userID}); err != nil {
			return fmt.Errorf("set avatar: %w", err)
		}
		return nil
	})
	return old, err
}

// RemoveAll deletes every file of userID, for an account that is gone. The
// row is gone with it, so nothing names the files any more.
func (s *Service) RemoveAll(userID string) error {
	if !image.IsID(userID) {
		return nil
	}
	if err := os.RemoveAll(filepath.Join(s.dir, userID)); err != nil {
		return fmt.Errorf("remove avatars of %s: %w", userID, err)
	}
	return nil
}

// Open returns the picture file avatarID of userID. Ids are checked against
// the UUID alphabet before they reach a path, so nothing escapes dir.
func (s *Service) Open(userID, avatarID string) (*os.File, error) {
	if !image.IsID(userID) || !image.IsID(avatarID) {
		return nil, os.ErrNotExist
	}
	f, err := os.Open(s.path(userID, avatarID))
	if err != nil {
		return nil, fmt.Errorf("open avatar: %w", err)
	}
	return f, nil
}
