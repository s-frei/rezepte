package transfer

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/s-frei/rezepte/service/internal/db/dbtest"
	"github.com/s-frei/rezepte/service/internal/image"
	"github.com/s-frei/rezepte/service/internal/recipe"
	"github.com/s-frei/rezepte/service/internal/user"
)

// The second photo's headers decode, so phase 1 passes; its pixel data is
// missing, so the upload in phase 2 fails after the first recipe and its
// photo were written. Both must be gone afterwards.
func TestImportRollsBackEverythingWhenAPhotoFailsToDecode(t *testing.T) {
	ctx := context.Background()
	conn := dbtest.Open(t)
	admin, err := user.NewService(conn, "").Create(ctx, user.CreateParams{Username: "sam", Password: "pw", Role: user.RoleAdmin})
	if err != nil {
		t.Fatal(err)
	}
	imageDir := filepath.Join(t.TempDir(), "images")
	recipes := recipe.NewService(conn, imageDir)
	svc := NewService(recipes, image.NewService(conn, imageDir), t.TempDir(), checker().validate)

	// Cut a real JPEG right after its start-of-scan header: every header
	// DecodeConfig reads is intact, the Huffman data the full decode needs
	// is not.
	jpg := tinyJPEG(t)
	sos := bytes.Index(jpg, []byte{0xff, 0xda})
	if sos < 0 {
		t.Fatal("no SOS marker")
	}
	broken := slices.Concat(jpg[:sos+14], make([]byte, 16))
	raw := zipBytes(t, []string{"a/recipe.json", "a/1.jpg", "b/recipe.json", "b/1.jpg"},
		map[string][]byte{"a/recipe.json": recipeJSON(t, nil), "a/1.jpg": tinyJPEG(t), "b/recipe.json": recipeJSON(t, nil), "b/1.jpg": broken})
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.check(zr); err != nil {
		t.Fatalf("phase 1 rejected the zip, so phase 2 is not exercised: %v", err)
	}

	_, err = svc.Import(ctx, admin, bytes.NewReader(raw), int64(len(raw)))
	var te *Error
	if !errors.As(err, &te) || te.Location != "b/1.jpg" {
		t.Fatalf("err = %v, want *Error at b/1.jpg", err)
	}
	page, err := recipes.List(ctx, recipe.ListParams{Page: 1, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 0 {
		t.Fatalf("%d recipes left behind", page.Total)
	}
	dirs, _ := os.ReadDir(imageDir)
	if len(dirs) != 0 {
		t.Fatalf("%d image dirs left behind", len(dirs))
	}
}
