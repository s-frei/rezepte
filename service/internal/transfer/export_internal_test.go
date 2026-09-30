package transfer

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/s-frei/rezepte/service/internal/recipe"
)

func TestWriteZipStopsWhenTheClientIsGone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// The zero Service has no image store: reaching a recipe's photos would
	// panic, so returning at all proves the loop checked ctx first.
	err := (&Service{}).writeZip(ctx, io.Discard, []recipe.Recipe{{}})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}
