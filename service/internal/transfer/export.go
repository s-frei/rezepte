package transfer

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/s-frei/rezepte/service/internal/recipe"
)

// Export writes the recipes with the given ids into a zip in a temp file
// and returns it rewound. The caller closes and removes it. Every id is
// loaded before the file is created, so an unknown id costs no disk.
func (s *Service) Export(ctx context.Context, ids []string) (*os.File, error) {
	if len(ids) > maxExportRecipes {
		return nil, fmt.Errorf("export: at most %d recipes", maxExportRecipes)
	}
	recipes := make([]recipe.Recipe, 0, len(ids))
	for _, id := range ids {
		r, err := s.recipes.ByID(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("export %s: %w", id, err)
		}
		recipes = append(recipes, r)
	}
	f, err := os.CreateTemp(s.tmpDir, "export-*.zip")
	if err != nil {
		return nil, fmt.Errorf("create export file: %w", err)
	}
	err = s.writeZip(ctx, f, recipes)
	if err == nil {
		if _, serr := f.Seek(0, io.SeekStart); serr != nil {
			err = fmt.Errorf("rewind export file: %w", serr)
		}
	}
	if err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return nil, err
	}
	return f, nil
}

func (s *Service) writeZip(ctx context.Context, w io.Writer, recipes []recipe.Recipe) error {
	zw := zip.NewWriter(w)
	for _, r := range recipes {
		// A client that gave up gets nothing from the rest of a large export.
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("export: %w", err)
		}
		in := r.Input
		in.EditPolicy = "" // omitempty: the target's default applies
		file := File{Format: Format, Version: Version, Input: in, Images: []string{}}
		// Image id -> file name; r.Images is ordered by position.
		names := make(map[string]string, len(r.Images))
		for i, im := range r.Images {
			name := strconv.Itoa(i+1) + ".jpg"
			file.Images = append(file.Images, name)
			names[im.ID] = name
		}
		if r.CoverImageID != nil {
			if name, ok := names[*r.CoverImageID]; ok {
				file.Cover = &name
			}
		}
		raw, err := json.MarshalIndent(file, "", "  ")
		if err != nil {
			return fmt.Errorf("encode %s: %w", r.Slug, err)
		}
		jw, err := zw.CreateHeader(&zip.FileHeader{Name: r.Slug + "/recipe.json", Method: zip.Deflate, Modified: r.UpdatedAt})
		if err != nil {
			return fmt.Errorf("zip %s: %w", r.Slug, err)
		}
		if _, err := jw.Write(raw); err != nil {
			return fmt.Errorf("zip %s: %w", r.Slug, err)
		}
		for _, im := range r.Images {
			if err := s.copyPhoto(zw, r.ID, im.ID, r.Slug+"/"+names[im.ID], r.UpdatedAt); err != nil {
				return err
			}
		}
	}
	if err := zw.Close(); err != nil {
		return fmt.Errorf("finish zip: %w", err)
	}
	return nil
}

// copyPhoto stores the largest variant as it is: a JPEG gains nothing
// from deflate. Every entry carries the recipe's last change as its date,
// rather than the zip format's 1980 default.
func (s *Service) copyPhoto(zw *zip.Writer, recipeID, imageID, name string, modified time.Time) error {
	src, err := s.images.Open(recipeID, imageID, "original")
	if err != nil {
		return fmt.Errorf("open %s: %w", name, err)
	}
	defer func() { _ = src.Close() }()
	dst, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Store, Modified: modified})
	if err != nil {
		return fmt.Errorf("zip %s: %w", name, err)
	}
	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("zip %s: %w", name, err)
	}
	return nil
}
