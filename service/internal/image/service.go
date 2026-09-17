package image

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"github.com/s-frei/rezepte/service/internal/db"
	"github.com/s-frei/rezepte/service/internal/db/sqlc"
	"github.com/s-frei/rezepte/service/internal/recipe"
)

// maxUploadBytes caps an upload body; the handler enforces it on the wire,
// the service reads at most this much so a bug there cannot exhaust memory.
const maxUploadBytes = 10 << 20

// variantSuffixes maps the public variant names to file name suffixes.
var variantSuffixes = map[string]string{"original": "", "detail": "_detail", "thumb": "_thumb"}

// Service stores and serves recipe images.
type Service struct {
	conn *sql.DB
	q    *sqlc.Queries
	dir  string
	now  func() time.Time
	// decodeSlots is a counting semaphore around the only expensive part
	// of an upload. Everything else (the queries, the 10 MiB body) is
	// cheap, but a decoded image costs up to maxPixels x 4 bytes and the
	// resize allocates another one, so unbounded concurrency would let a
	// handful of clients exhaust memory. Buffered to concurrentDecodes;
	// the rest of an upload runs outside it.
	decodeSlots chan struct{}
}

// NewService returns a Service writing files below dir (one subdirectory
// per recipe id).
func NewService(conn *sql.DB, dir string) *Service {
	return &Service{
		conn:        conn,
		q:           sqlc.New(conn),
		dir:         dir,
		now:         time.Now,
		decodeSlots: make(chan struct{}, concurrentDecodes),
	}
}

// Upload decodes the image in r, writes its variants and records it as the
// last image of recipeID. When the recipe has no cover yet, the new image
// becomes its cover. Errors: ErrNotFound (recipe), ErrUnsupported,
// ErrInvalid, ErrTooLarge, ErrTooMany.
func (s *Service) Upload(ctx context.Context, recipeID string, r io.Reader) (recipe.Image, error) {
	if !isID(recipeID) {
		return recipe.Image{}, ErrNotFound
	}
	// Cheap checks before decoding a possibly 10 MiB body.
	if _, err := s.q.GetRecipe(ctx, recipeID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return recipe.Image{}, ErrNotFound
		}
		return recipe.Image{}, fmt.Errorf("get recipe %s: %w", recipeID, err)
	}
	if n, err := s.q.CountImagesByRecipe(ctx, recipeID); err != nil {
		return recipe.Image{}, fmt.Errorf("count images: %w", err)
	} else if n >= maxImages {
		return recipe.Image{}, ErrTooMany
	}
	data, err := io.ReadAll(io.LimitReader(r, maxUploadBytes+1))
	if err != nil {
		return recipe.Image{}, fmt.Errorf("read upload: %w", err)
	}
	if len(data) > maxUploadBytes {
		return recipe.Image{}, fmt.Errorf("%w: larger than %d bytes", ErrInvalid, maxUploadBytes)
	}
	id := uuid.Must(uuid.NewV7()).String()
	recipeDir := filepath.Join(s.dir, recipeID)
	w, err := s.render(ctx, data, recipeDir, id)
	if err != nil {
		return recipe.Image{}, err
	}

	var out recipe.Image
	err = db.Tx(ctx, s.conn, func(q *sqlc.Queries) error {
		row, err := q.GetRecipe(ctx, recipeID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return fmt.Errorf("get recipe %s: %w", recipeID, err)
		}
		n, err := q.CountImagesByRecipe(ctx, recipeID)
		if err != nil {
			return fmt.Errorf("count images: %w", err)
		}
		if n >= maxImages {
			return ErrTooMany
		}
		pos, err := q.NextImagePosition(ctx, recipeID)
		if err != nil {
			return fmt.Errorf("next position: %w", err)
		}
		now := db.FormatTime(s.now())
		inserted, err := q.InsertImage(ctx, sqlc.InsertImageParams{
			ID: id, RecipeID: recipeID, Filename: id + ".jpg",
			Width: int64(w.Width), Height: int64(w.Height), SizeBytes: w.Size,
			Position: pos, CreatedAt: now,
		})
		if err != nil {
			return fmt.Errorf("insert image: %w", err)
		}
		if row.CoverImageID == nil {
			if err := q.SetRecipeCover(ctx, sqlc.SetRecipeCoverParams{CoverImageID: &id, UpdatedAt: now, ID: recipeID}); err != nil {
				return fmt.Errorf("set cover: %w", err)
			}
		} else if err := q.TouchRecipe(ctx, sqlc.TouchRecipeParams{UpdatedAt: now, ID: recipeID}); err != nil {
			return fmt.Errorf("touch recipe: %w", err)
		}
		out = toImage(inserted)
		return nil
	})
	if err != nil {
		removeFiles(recipeDir, id)
		return recipe.Image{}, err
	}
	return out, nil
}

// render decodes data and writes the variants of id into recipeDir while
// holding one of the decodeSlots, so at most concurrentDecodes uploads are
// ever decoding or resizing at the same time. A client that gives up while
// queued releases its place through ctx.
func (s *Service) render(ctx context.Context, data []byte, recipeDir, id string) (written, error) {
	select {
	case s.decodeSlots <- struct{}{}:
	case <-ctx.Done():
		return written{}, fmt.Errorf("wait for decode slot: %w", ctx.Err())
	}
	defer func() { <-s.decodeSlots }()

	img, err := decode(data)
	if err != nil {
		return written{}, err
	}
	return writeVariants(recipeDir, id, img)
}

// Delete removes imageID from recipeID: the row, then (after commit) the
// files. If it was the cover, the first remaining image by position takes
// over, or the cover is cleared. Remaining positions are renumbered 0..n-1.
func (s *Service) Delete(ctx context.Context, recipeID, imageID string) error {
	if !isID(recipeID) || !isID(imageID) {
		return ErrNotFound
	}
	err := db.Tx(ctx, s.conn, func(q *sqlc.Queries) error {
		row, err := q.GetRecipe(ctx, recipeID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return fmt.Errorf("get recipe %s: %w", recipeID, err)
		}
		n, err := q.DeleteImage(ctx, sqlc.DeleteImageParams{RecipeID: recipeID, ID: imageID})
		if err != nil {
			return fmt.Errorf("delete image: %w", err)
		}
		if n == 0 {
			return ErrNotFound
		}
		rest, err := q.ListImagesByRecipe(ctx, recipeID)
		if err != nil {
			return fmt.Errorf("list images: %w", err)
		}
		if err := renumber(ctx, q, recipeID, rest); err != nil {
			return err
		}
		now := db.FormatTime(s.now())
		if row.CoverImageID != nil && *row.CoverImageID == imageID {
			var cover *string
			if len(rest) > 0 {
				cover = &rest[0].ID
			}
			if err := q.SetRecipeCover(ctx, sqlc.SetRecipeCoverParams{CoverImageID: cover, UpdatedAt: now, ID: recipeID}); err != nil {
				return fmt.Errorf("set cover: %w", err)
			}
			return nil
		}
		if err := q.TouchRecipe(ctx, sqlc.TouchRecipeParams{UpdatedAt: now, ID: recipeID}); err != nil {
			return fmt.Errorf("touch recipe: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	removeFiles(filepath.Join(s.dir, recipeID), imageID)
	return nil
}

// Reorder assigns positions 0..n-1 following ids, which must name every
// image of recipeID exactly once (ErrBadOrder otherwise), and returns the
// images in their new order.
func (s *Service) Reorder(ctx context.Context, recipeID string, ids []string) ([]recipe.Image, error) {
	if !isID(recipeID) {
		return nil, ErrNotFound
	}
	var out []recipe.Image
	err := db.Tx(ctx, s.conn, func(q *sqlc.Queries) error {
		if _, err := q.GetRecipe(ctx, recipeID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return fmt.Errorf("get recipe %s: %w", recipeID, err)
		}
		current, err := q.ListImagesByRecipe(ctx, recipeID)
		if err != nil {
			return fmt.Errorf("list images: %w", err)
		}
		byID := make(map[string]sqlc.Image, len(current))
		for _, im := range current {
			byID[im.ID] = im
		}
		if len(ids) != len(current) {
			return ErrBadOrder
		}
		ordered := make([]sqlc.Image, 0, len(ids))
		seen := make(map[string]bool, len(ids))
		for _, id := range ids {
			im, ok := byID[id]
			if !ok || seen[id] {
				return ErrBadOrder
			}
			seen[id] = true
			ordered = append(ordered, im)
		}
		if err := renumber(ctx, q, recipeID, ordered); err != nil {
			return err
		}
		if err := q.TouchRecipe(ctx, sqlc.TouchRecipeParams{UpdatedAt: db.FormatTime(s.now()), ID: recipeID}); err != nil {
			return fmt.Errorf("touch recipe: %w", err)
		}
		out = make([]recipe.Image, len(ordered))
		for i, im := range ordered {
			im.Position = int64(i)
			out[i] = toImage(im)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// SetCover makes imageID the cover of recipeID. The image must belong to
// the recipe (ErrNotFound otherwise).
func (s *Service) SetCover(ctx context.Context, recipeID, imageID string) error {
	if !isID(recipeID) || !isID(imageID) {
		return ErrNotFound
	}
	return db.Tx(ctx, s.conn, func(q *sqlc.Queries) error {
		if _, err := q.GetImage(ctx, sqlc.GetImageParams{RecipeID: recipeID, ID: imageID}); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return fmt.Errorf("get image: %w", err)
		}
		id := imageID
		if err := q.SetRecipeCover(ctx, sqlc.SetRecipeCoverParams{CoverImageID: &id, UpdatedAt: db.FormatTime(s.now()), ID: recipeID}); err != nil {
			return fmt.Errorf("set cover: %w", err)
		}
		return nil
	})
}

// Open returns the file of one variant ("thumb", "detail" or "original")
// for reading. Unknown variants, malformed ids and missing files are all
// ErrNotFound; ids are checked against the UUID alphabet so no path
// component can escape the image directory.
func (s *Service) Open(recipeID, imageID, variant string) (*os.File, error) {
	suffix, ok := variantSuffixes[variant]
	if !ok || !isID(recipeID) || !isID(imageID) {
		return nil, ErrNotFound
	}
	f, err := os.Open(variantPath(s.dir, recipeID, imageID, suffix))
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("open image: %w", err)
	}
	return f, nil
}

// renumber writes positions 0..n-1 onto images in slice order.
func renumber(ctx context.Context, q *sqlc.Queries, recipeID string, images []sqlc.Image) error {
	for i, im := range images {
		if int(im.Position) == i {
			continue
		}
		if err := q.UpdateImagePosition(ctx, sqlc.UpdateImagePositionParams{Position: int64(i), RecipeID: recipeID, ID: im.ID}); err != nil {
			return fmt.Errorf("update position: %w", err)
		}
	}
	return nil
}

// removeFiles deletes every variant of imageID; missing files are fine.
func removeFiles(recipeDir, imageID string) {
	for _, v := range variants {
		_ = os.Remove(filepath.Join(recipeDir, imageID+v.suffix+".jpg"))
	}
}

func toImage(im sqlc.Image) recipe.Image {
	return recipe.Image{ID: im.ID, Width: int(im.Width), Height: int(im.Height), Position: int(im.Position)}
}

// isID accepts the canonical 36-character UUID spelling and nothing else.
func isID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		switch {
		case i == 8 || i == 13 || i == 18 || i == 23:
			if c != '-' {
				return false
			}
		case c >= '0' && c <= '9', c >= 'a' && c <= 'f':
		default:
			return false
		}
	}
	return true
}
