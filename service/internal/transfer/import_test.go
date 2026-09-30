package transfer_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/s-frei/rezepte/service/internal/recipe"
	"github.com/s-frei/rezepte/service/internal/transfer"
)

func exportOf(t *testing.T, e *env, ids ...string) []byte {
	t.Helper()
	f, err := e.svc.Export(context.Background(), ids)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	defer f.Close()
	raw, err := os.ReadFile(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestImportRoundTrip(t *testing.T) {
	ctx := context.Background()
	src := newEnv(t)
	r, err := src.recipes.Create(ctx, src.admin.ID, sampleInput("Apfelstrudel"))
	if err != nil {
		t.Fatal(err)
	}
	for _, size := range [][2]int{{120, 80}, {80, 120}} {
		if _, err := src.images.Upload(ctx, r.ID, src.admin, bytes.NewReader(jpegBytes(t, size[0], size[1]))); err != nil {
			t.Fatal(err)
		}
	}
	stored, err := src.recipes.ByID(ctx, r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := src.images.SetCover(ctx, r.ID, stored.Images[1].ID, src.admin); err != nil {
		t.Fatal(err)
	}
	archive := exportOf(t, src, r.ID)

	dst := newEnv(t)
	created, err := dst.svc.Import(ctx, dst.admin, bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatal(err)
	}
	if len(created) != 1 || created[0].Title != "Apfelstrudel" || created[0].Slug != "apfelstrudel" {
		t.Fatalf("created = %+v", created)
	}
	got, err := dst.recipes.ByID(ctx, created[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.CreatedBy.ID != dst.admin.ID {
		t.Fatalf("author = %s", got.CreatedBy.ID)
	}
	if len(got.Images) != 2 || got.Images[0].Width != 120 || got.Images[1].Width != 80 {
		t.Fatalf("images = %+v", got.Images)
	}
	if got.CoverImageID == nil || *got.CoverImageID != got.Images[1].ID {
		t.Fatalf("cover = %v", got.CoverImageID)
	}
	if len(got.Steps) != 1 || len(got.Steps[0].References) != 1 {
		t.Fatalf("steps = %+v", got.Steps)
	}
}

func TestImportOfAnExistingTitleGetsANewSlug(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	r, err := e.recipes.Create(ctx, e.admin.ID, sampleInput("Focaccia"))
	if err != nil {
		t.Fatal(err)
	}
	archive := exportOf(t, e, r.ID)
	created, err := e.svc.Import(ctx, e.admin, bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatal(err)
	}
	if created[0].Slug != "focaccia-2" {
		t.Fatalf("slug = %s", created[0].Slug)
	}
}

func TestImportRefusesAFileThatIsNotAZip(t *testing.T) {
	e := newEnv(t)
	raw := []byte("not a zip")
	_, err := e.svc.Import(context.Background(), e.admin, bytes.NewReader(raw), int64(len(raw)))
	var te *transfer.Error
	if !errors.As(err, &te) || te.Msg != "not a zip file" {
		t.Fatalf("err = %v", err)
	}
}

// A corrupted stored photo fails its CRC while phase 1 reads it, so nothing
// is written at all. The phase-2 rollback has its own internal test.
func TestImportWritesNothingWhenAPhotoIsCorrupt(t *testing.T) {
	ctx := context.Background()
	src := newEnv(t)
	good, err := src.recipes.Create(ctx, src.admin.ID, sampleInput("First"))
	if err != nil {
		t.Fatal(err)
	}
	bad, err := src.recipes.Create(ctx, src.admin.ID, sampleInput("Second"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := src.images.Upload(ctx, bad.ID, src.admin, bytes.NewReader(jpegBytes(t, 100, 100))); err != nil {
		t.Fatal(err)
	}
	archive := exportOf(t, src, good.ID, bad.ID)
	stored, err := src.recipes.ByID(ctx, bad.ID)
	if err != nil {
		t.Fatal(err)
	}
	photo, err := os.ReadFile(filepath.Join(src.dataDir, "images", bad.ID, stored.Images[0].ID+".jpg"))
	if err != nil {
		t.Fatal(err)
	}
	idx := bytes.Index(archive, photo)
	if idx < 0 {
		t.Fatal("photo not stored verbatim")
	}
	for i := idx + len(photo)/2; i < idx+len(photo)-2; i++ {
		archive[i] = 0
	}

	dst := newEnv(t)
	_, err = dst.svc.Import(ctx, dst.admin, bytes.NewReader(archive), int64(len(archive)))
	var te *transfer.Error
	if !errors.As(err, &te) {
		t.Fatalf("err = %v, want *transfer.Error", err)
	}
	page, err := dst.recipes.List(ctx, recipe.ListParams{Page: 1, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 0 {
		t.Fatalf("%d recipes left behind", page.Total)
	}
	dirs, _ := os.ReadDir(filepath.Join(dst.dataDir, "images"))
	if len(dirs) != 0 {
		t.Fatalf("%d image dirs left behind", len(dirs))
	}
}
