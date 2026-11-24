// Package transfer moves recipes between Rezepte instances as a zip: one
// folder per recipe with a recipe.json and its photos. It is not a backup;
// see docs/memory/content/features/import-export.mdx.
package transfer

import (
	"encoding/json"
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
// header and the photos of its folder in display order. ImageFocus runs
// parallel to Images; an archive without it, or with fewer entries, leaves
// the rest centered. Export leaves it out while every photo is centered:
// a Rezepte from before focus points validates recipe.json strictly and
// refuses the key.
type File struct {
	Format  string `json:"format"`
	Version int    `json:"version"`
	recipe.Input
	Images     []string  `json:"images"`
	Cover      *string   `json:"cover"`
	ImageFocus FocusList `json:"imageFocus,omitempty"`
}

// FocusList is recipe.json's imageFocus. It decodes leniently: a point is
// display data, so a list that is not one reads as absent and an entry
// that is not {x, y} as centered, never as a reason to refuse the archive.
type FocusList []recipe.ImageFocus

// UnmarshalJSON never fails; see FocusList.
func (l *FocusList) UnmarshalJSON(b []byte) error {
	var raw []json.RawMessage
	if json.Unmarshal(b, &raw) != nil {
		*l = nil
		return nil
	}
	*l = make(FocusList, len(raw))
	for i, r := range raw {
		var p struct{ X, Y *float64 }
		if json.Unmarshal(r, &p) != nil || p.X == nil || p.Y == nil {
			(*l)[i] = recipe.ImageFocus{X: 0.5, Y: 0.5}
			continue
		}
		(*l)[i] = recipe.ImageFocus{X: *p.X, Y: *p.Y}
	}
	return nil
}

// archiveStep is how recipe.json writes a step: a photo link's imageId holds
// the archive file name ("3.jpg"), the way cover does, because the ids of an
// export mean nothing to the instance that imports it. Photos shadows the
// embedded Step's to leave the key out without links, so an older Rezepte
// still reads the step.
type archiveStep struct {
	recipe.Step
	Photos []recipe.StepPhoto `json:"photos,omitempty"`
}

// Error is a problem with one entry of an import, located by its path
// inside the zip (and, for a schema error, the field inside recipe.json).
type Error struct {
	Location string
	Msg      string
}

func (e *Error) Error() string { return fmt.Sprintf("%s: %s", e.Location, e.Msg) }
