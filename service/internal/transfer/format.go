// Package transfer moves recipes between Rezepte instances as a zip: one
// folder per recipe with a recipe.json and its photos. It is not a backup;
// see docs/memory/content/features/import-export.mdx.
package transfer

import (
	"fmt"

	"github.com/s-frei/rezepte/service/internal/recipe"
)

// Format and Version identify recipe.json. A reader refuses any other
// format and any higher version.
const (
	Format  = "rezepte.recipe"
	Version = 1
)

const (
	maxExportRecipes = 500
	maxImportBytes   = 1 << 30
	maxJSONBytes     = 1 << 20
	maxPhotoBytes    = 10 << 20
	maxPhotos        = 20
)

// File is recipe.json: the recipe as the API accepts it, plus the format
// header and the photos of its folder in display order.
type File struct {
	Format  string `json:"format"`
	Version int    `json:"version"`
	recipe.Input
	Images []string `json:"images"`
	Cover  *string  `json:"cover"`
}

// Error is a problem with one entry of an import, located by its path
// inside the zip (and, for a schema error, the field inside recipe.json).
type Error struct {
	Location string
	Msg      string
}

func (e *Error) Error() string { return fmt.Sprintf("%s: %s", e.Location, e.Msg) }
