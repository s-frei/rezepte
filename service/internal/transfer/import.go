package transfer

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"slices"

	"github.com/s-frei/rezepte/service/internal/image"
	"github.com/s-frei/rezepte/service/internal/recipe"
	"github.com/s-frei/rezepte/service/internal/user"
)

// Created names a recipe an import created.
type Created struct {
	ID    string `json:"id"`
	Slug  string `json:"slug"`
	Title string `json:"title"`
}

// Import checks the whole zip, then creates its recipes in zip order with
// actor as author. Any failure deletes every recipe this call created, so
// one call is all or nothing. Errors about the file are *Error.
func (s *Service) Import(ctx context.Context, actor user.User, r io.ReaderAt, size int64) (created []Created, err error) {
	if size > maxImportBytes {
		return nil, &Error{Msg: fmt.Sprintf("larger than %d bytes", maxImportBytes)}
	}
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, &Error{Msg: "not a zip file"}
	}
	folders, err := s.check(zr)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err == nil {
			return
		}
		// The request may be gone; the rollback must still run. Deleting a
		// recipe also removes its image directory.
		cleanup := context.WithoutCancel(ctx)
		for _, c := range created {
			if derr := s.recipes.Delete(cleanup, c.ID, actor); derr != nil {
				err = errors.Join(err, fmt.Errorf("roll back %s: %w", c.Slug, derr))
			}
		}
		created = nil
	}()
	for _, f := range folders {
		c, cerr := s.create(ctx, actor, f)
		if c.ID != "" {
			created = append(created, c)
		}
		if cerr != nil {
			return created, cerr
		}
	}
	return created, nil
}

// create writes one checked folder. A non-empty Created comes back even on
// error, so the caller can roll the recipe back.
func (s *Service) create(ctx context.Context, actor user.User, f folder) (Created, error) {
	// Create refuses links to photos that do not exist yet; they are written
	// by one Update once the photos are uploaded.
	in := f.file.Input
	in.Steps = slices.Clone(in.Steps)
	linked := false
	for i := range in.Steps {
		linked = linked || len(in.Steps[i].Photos) > 0
		in.Steps[i].Photos = []recipe.StepPhoto{}
	}
	rec, err := s.recipes.Create(ctx, actor.ID, in)
	if err != nil {
		return Created{}, fmt.Errorf("create %s: %w", f.name, err)
	}
	c := Created{ID: rec.ID, Slug: rec.Slug, Title: rec.Title}
	ids := make(map[string]string, len(f.file.Images))
	for _, name := range f.file.Images {
		loc := f.name + "/" + name
		rc, err := f.photos[name].Open()
		if err != nil {
			return c, &Error{Location: loc, Msg: "cannot be read"}
		}
		img, err := s.images.Upload(ctx, rec.ID, actor, rc)
		_ = rc.Close()
		if errors.Is(err, image.ErrUnsupported) || errors.Is(err, image.ErrInvalid) || errors.Is(err, image.ErrTooLarge) {
			return c, &Error{Location: loc, Msg: err.Error()}
		}
		if err != nil {
			return c, fmt.Errorf("upload %s: %w", loc, err)
		}
		ids[name] = img.ID
	}
	// The first upload became the cover; move it when the file says so.
	if f.file.Cover != nil && *f.file.Cover != f.file.Images[0] {
		if err := s.images.SetCover(ctx, rec.ID, ids[*f.file.Cover], actor); err != nil {
			return c, fmt.Errorf("set cover of %s: %w", f.name, err)
		}
	}
	if linked {
		withIDs := slices.Clone(f.file.Steps)
		for i, st := range withIDs {
			withIDs[i].Photos = slices.Clone(st.Photos)
			for j, ph := range st.Photos {
				withIDs[i].Photos[j].ImageID = ids[ph.ImageID]
			}
		}
		in.Steps = withIDs
		if _, err := s.recipes.Update(ctx, rec.ID, actor, in); err != nil {
			return c, fmt.Errorf("link photos of %s: %w", f.name, err)
		}
	}
	return c, nil
}
