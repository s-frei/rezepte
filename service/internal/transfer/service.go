package transfer

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/danielgtaylor/huma/v2"

	"github.com/s-frei/rezepte/service/internal/image"
	"github.com/s-frei/rezepte/service/internal/recipe"
)

// Validator checks a decoded recipe.json (without the four keys File adds)
// against the rules the API enforces on recipe.Input. recipe.Service does
// not check lengths itself, so an import must.
type Validator func(raw map[string]any) []*huma.ErrorDetail

// Service writes and reads transfer zips.
type Service struct {
	recipes  *recipe.Service
	images   *image.Service
	tmpDir   string
	validate Validator
}

// NewService returns a Service that keeps its temp files in tmpDir.
func NewService(recipes *recipe.Service, images *image.Service, tmpDir string, validate Validator) *Service {
	return &Service{recipes: recipes, images: images, tmpDir: tmpDir, validate: validate}
}

// SweepTemp removes the export and import files a killed process left in
// dir - an import spools up to 1 GiB there. It runs once at startup, before
// any request can create new ones.
func SweepTemp(dir string) error {
	for _, pattern := range []string{"export-*.zip", "import-*.zip"} {
		matches, err := filepath.Glob(filepath.Join(dir, pattern))
		if err != nil {
			return fmt.Errorf("sweep %s: %w", pattern, err)
		}
		for _, m := range matches {
			if err := os.Remove(m); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("sweep %s: %w", m, err)
			}
		}
	}
	return nil
}
