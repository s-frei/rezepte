package image

import (
	"errors"
	"fmt"
	"testing"

	"github.com/danielgtaylor/huma/v2"

	"github.com/s-frei/rezepte/service/internal/recipe"
)

func TestImageErr(t *testing.T) {
	for err, want := range map[error]int{
		ErrNotFound:                       404,
		recipe.ErrEditForbidden:           403,
		ErrUnsupported:                    415,
		fmt.Errorf("%w: bad", ErrInvalid): 422,
		ErrTooLarge:                       422,
		ErrTooMany:                        422,
		ErrBadOrder:                       422,
		fmt.Errorf("x: %w", ErrNotFound):  404,
	} {
		var se huma.StatusError
		if got := imageErr(err); !errors.As(got, &se) || se.GetStatus() != want {
			t.Errorf("imageErr(%v) = %v, want status %d", err, got, want)
		}
	}
	other := errors.New("disk full")
	if got := imageErr(other); got != other { //nolint:errorlint // identity is the point: unmapped errors pass through untouched
		t.Errorf("imageErr(other) = %v, want it unchanged", got)
	}
}
