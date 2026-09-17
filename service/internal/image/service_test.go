package image_test

import (
	"bytes"
	"context"
	"errors"
	stdimage "image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/s-frei/rezepte/service/internal/db/dbtest"
	"github.com/s-frei/rezepte/service/internal/image"
	"github.com/s-frei/rezepte/service/internal/recipe"
	"github.com/s-frei/rezepte/service/internal/user"
)

type env struct {
	images  *image.Service
	recipes *recipe.Service
	dir     string
	recipe  recipe.Recipe
}

func setup(t *testing.T) env {
	t.Helper()
	ctx := context.Background()
	conn := dbtest.Open(t)
	u, err := user.NewService(conn).Create(ctx, "sam", "pw", user.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	recipes := recipe.NewService(conn, recipe.WithImageDir(dir))
	r, err := recipes.Create(ctx, u.ID, recipe.Input{
		Title: "Testrezept", Servings: 2,
		IngredientGroups: []recipe.IngredientGroup{{Ingredients: []recipe.Ingredient{{Name: "Salz"}}}},
		Steps:            []string{"Salzen."},
	})
	if err != nil {
		t.Fatal(err)
	}
	return env{images: image.NewService(conn, dir), recipes: recipes, dir: dir, recipe: r}
}

// pngBytes renders a w×h PNG in a flat colour.
func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := stdimage.NewNRGBA(stdimage.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.NRGBA{180, 90, 30, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func upload(t *testing.T, e env, w, h int) recipe.Image {
	t.Helper()
	img, err := e.images.Upload(context.Background(), e.recipe.ID, bytes.NewReader(pngBytes(t, w, h)))
	if err != nil {
		t.Fatalf("upload %dx%d: %v", w, h, err)
	}
	return img
}

func TestUploadWritesVariantsAndSetsFirstCover(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	first := upload(t, e, 3000, 2000)
	if first.Width != 2400 || first.Height != 1600 || first.Position != 0 || first.ID == "" {
		t.Fatalf("first = %+v", first)
	}
	for _, name := range []string{first.ID + ".jpg", first.ID + "_detail.jpg", first.ID + "_thumb.jpg"} {
		if _, err := os.Stat(filepath.Join(e.dir, e.recipe.ID, name)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	second := upload(t, e, 640, 480)
	if second.Position != 1 || second.Width != 640 {
		t.Fatalf("second = %+v", second)
	}
	r, err := e.recipes.ByID(ctx, e.recipe.ID)
	if err != nil {
		t.Fatal(err)
	}
	if r.CoverImageID == nil || *r.CoverImageID != first.ID {
		t.Fatalf("cover = %v, want first upload", r.CoverImageID)
	}
	if len(r.Images) != 2 || r.Images[1].ID != second.ID {
		t.Fatalf("images = %+v", r.Images)
	}
	if !r.UpdatedAt.After(e.recipe.UpdatedAt) && !r.UpdatedAt.Equal(e.recipe.UpdatedAt) {
		t.Fatalf("updatedAt went backwards: %v < %v", r.UpdatedAt, e.recipe.UpdatedAt)
	}
}

func TestUploadErrors(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	if _, err := e.images.Upload(ctx, "missing", bytes.NewReader(pngBytes(t, 64, 64))); !errors.Is(err, image.ErrNotFound) {
		t.Fatalf("unknown recipe: %v", err)
	}
	if entries, _ := os.ReadDir(e.dir); len(entries) != 0 {
		t.Fatalf("unknown recipe must write nothing, got %d entries", len(entries))
	}
	if _, err := e.images.Upload(ctx, e.recipe.ID, bytes.NewReader([]byte("not an image"))); !errors.Is(err, image.ErrUnsupported) {
		t.Fatalf("garbage: %v", err)
	}
	if _, err := e.images.Upload(ctx, e.recipe.ID, bytes.NewReader(pngBytes(t, 32, 64))); !errors.Is(err, image.ErrInvalid) {
		t.Fatalf("too small: %v", err)
	}
	for i := 0; i < 20; i++ {
		upload(t, e, 64, 64)
	}
	if _, err := e.images.Upload(ctx, e.recipe.ID, bytes.NewReader(pngBytes(t, 64, 64))); !errors.Is(err, image.ErrTooMany) {
		t.Fatalf("21st: %v", err)
	}
	if entries, _ := os.ReadDir(filepath.Join(e.dir, e.recipe.ID)); len(entries) != 60 {
		t.Fatalf("%d files, want 60 (rejected upload must not leave files)", len(entries))
	}
}

func TestDeletePromotesNextCoverAndCompactsPositions(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	a, b, c := upload(t, e, 64, 64), upload(t, e, 64, 64), upload(t, e, 64, 64)
	if err := e.images.Delete(ctx, e.recipe.ID, a.ID); err != nil {
		t.Fatal(err)
	}
	r, _ := e.recipes.ByID(ctx, e.recipe.ID)
	if r.CoverImageID == nil || *r.CoverImageID != b.ID {
		t.Fatalf("cover after deleting cover = %v, want %s", r.CoverImageID, b.ID)
	}
	if len(r.Images) != 2 || r.Images[0].Position != 0 || r.Images[1].Position != 1 || r.Images[1].ID != c.ID {
		t.Fatalf("images = %+v", r.Images)
	}
	if _, err := os.Stat(filepath.Join(e.dir, e.recipe.ID, a.ID+"_thumb.jpg")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("files of deleted image remain: %v", err)
	}
	// Deleting a non-cover keeps the cover.
	if err := e.images.Delete(ctx, e.recipe.ID, c.ID); err != nil {
		t.Fatal(err)
	}
	r, _ = e.recipes.ByID(ctx, e.recipe.ID)
	if *r.CoverImageID != b.ID {
		t.Fatalf("cover changed to %s", *r.CoverImageID)
	}
	if err := e.images.Delete(ctx, e.recipe.ID, b.ID); err != nil {
		t.Fatal(err)
	}
	r, _ = e.recipes.ByID(ctx, e.recipe.ID)
	if r.CoverImageID != nil || len(r.Images) != 0 {
		t.Fatalf("after last delete: cover=%v images=%d", r.CoverImageID, len(r.Images))
	}
	if err := e.images.Delete(ctx, e.recipe.ID, b.ID); !errors.Is(err, image.ErrNotFound) {
		t.Fatalf("second delete: %v", err)
	}
	// A fresh upload onto a coverless recipe becomes the cover again.
	d := upload(t, e, 64, 64)
	r, _ = e.recipes.ByID(ctx, e.recipe.ID)
	if r.CoverImageID == nil || *r.CoverImageID != d.ID || d.Position != 0 {
		t.Fatalf("re-upload: cover=%v pos=%d", r.CoverImageID, d.Position)
	}
}

func TestReorderRequiresExactPermutation(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	a, b, c := upload(t, e, 64, 64), upload(t, e, 64, 64), upload(t, e, 64, 64)
	got, err := e.images.Reorder(ctx, e.recipe.ID, []string{c.ID, a.ID, b.ID})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].ID != c.ID || got[0].Position != 0 || got[1].ID != a.ID || got[2].ID != b.ID || got[2].Position != 2 {
		t.Fatalf("reordered = %+v", got)
	}
	bad := [][]string{
		{a.ID, b.ID},             // missing one
		{a.ID, b.ID, c.ID, c.ID}, // duplicate
		{a.ID, b.ID, "nope"},     // foreign id
		{a.ID, a.ID, b.ID},       // duplicate with right length
	}
	for _, ids := range bad {
		if _, err := e.images.Reorder(ctx, e.recipe.ID, ids); !errors.Is(err, image.ErrBadOrder) {
			t.Errorf("%v: err = %v, want ErrBadOrder", ids, err)
		}
	}
	if _, err := e.images.Reorder(ctx, "missing", []string{}); !errors.Is(err, image.ErrNotFound) {
		t.Fatalf("unknown recipe: %v", err)
	}
}

func TestSetCover(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	a, b := upload(t, e, 64, 64), upload(t, e, 64, 64)
	if err := e.images.SetCover(ctx, e.recipe.ID, b.ID); err != nil {
		t.Fatal(err)
	}
	r, _ := e.recipes.ByID(ctx, e.recipe.ID)
	if *r.CoverImageID != b.ID {
		t.Fatalf("cover = %s, want %s", *r.CoverImageID, b.ID)
	}
	if err := e.images.SetCover(ctx, e.recipe.ID, "nope"); !errors.Is(err, image.ErrNotFound) {
		t.Fatalf("foreign image: %v", err)
	}
	if err := e.images.SetCover(ctx, "missing", a.ID); !errors.Is(err, image.ErrNotFound) {
		t.Fatalf("unknown recipe: %v", err)
	}
}

func TestOpenValidatesVariantAndIds(t *testing.T) {
	e := setup(t)
	a := upload(t, e, 64, 64)
	for _, v := range []string{"thumb", "detail", "original"} {
		f, err := e.images.Open(e.recipe.ID, a.ID, v)
		if err != nil {
			t.Fatalf("%s: %v", v, err)
		}
		if _, err := jpeg.DecodeConfig(f); err != nil {
			t.Fatalf("%s: not a jpeg: %v", v, err)
		}
		_ = f.Close()
	}
	for _, c := range [][3]string{
		{e.recipe.ID, a.ID, "large"},
		{e.recipe.ID, "nope", "thumb"},
		{"..", a.ID, "thumb"},
		{e.recipe.ID, "../" + a.ID, "thumb"},
	} {
		if _, err := e.images.Open(c[0], c[1], c[2]); !errors.Is(err, image.ErrNotFound) {
			t.Errorf("%v: err = %v, want ErrNotFound", c, err)
		}
	}
}
