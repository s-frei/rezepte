package recipe

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/s-frei/rezepte/service/internal/db/sqlc"
)

// StepPhoto ties one of the recipe's photos to a phrase of a step's text. Like
// StepTime it is anchored by the phrase, and it points at the image by id, so
// reordering the photos leaves the link on the same picture.
type StepPhoto struct {
	Phrase  string `json:"phrase" minLength:"1" maxLength:"120" doc:"The words of the step's text that open the photo"`
	ImageID string `json:"imageId" minLength:"1" doc:"Id of one of the recipe's photos"`
}

// ValidatePhotos checks every step's photo links against imageIDs, the ids of
// the recipe's photos (nil on create, which has none yet; the archive check
// passes file names). The rules: features/step-photos in the memory.
func ValidatePhotos(steps []Step, imageIDs map[string]struct{}) error {
	for si, step := range steps {
		type span struct{ from, to int }
		var taken []span
		for _, tm := range step.Times {
			p := strings.TrimSpace(tm.Phrase)
			if i := wordIndex(step.Text, p); i >= 0 {
				taken = append(taken, span{i, i + len(p)})
			}
		}
		for pi, ph := range step.Photos {
			fail := func(field, msg string) error {
				return &RefError{List: "photos", Step: si, Ref: pi, Field: field, Msg: msg}
			}
			phrase := strings.TrimSpace(ph.Phrase)
			i := wordIndex(step.Text, phrase)
			if i < 0 {
				return fail("phrase", fmt.Sprintf("%q does not occur in the step's text", phrase))
			}
			cur := span{i, i + len(phrase)}
			for _, o := range taken {
				if cur.from < o.to && o.from < cur.to {
					return fail("phrase", fmt.Sprintf("%q overlaps a time or another photo link", phrase))
				}
			}
			taken = append(taken, cur)
			if imageIDs == nil {
				return fail("imageId", "a new recipe has no photos yet; upload them, then link them with an update")
			}
			if _, ok := imageIDs[ph.ImageID]; !ok {
				return fail("imageId", "is not a photo of this recipe")
			}
		}
	}
	return nil
}

// dropDeletedPhotos leaves out every link to an image that no longer exists
// at all: deleting a photo removes its links, also from an editor that still
// held one, and the phrase stays text. A link to an image of another recipe
// stays in, so ValidatePhotos refuses it. steps is copied, never changed.
func dropDeletedPhotos(ctx context.Context, q *sqlc.Queries, steps []Step, imageIDs map[string]struct{}) ([]Step, error) {
	out := slices.Clone(steps)
	for i, st := range out {
		var kept []StepPhoto
		for _, ph := range st.Photos {
			if _, ok := imageIDs[ph.ImageID]; !ok {
				exists, err := q.ImageExists(ctx, ph.ImageID)
				if err != nil {
					return nil, fmt.Errorf("look up image: %w", err)
				}
				if !exists {
					continue
				}
			}
			kept = append(kept, ph)
		}
		out[i].Photos = kept
	}
	return out, nil
}
