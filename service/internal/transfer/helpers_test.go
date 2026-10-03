package transfer_test

import (
	"bytes"
	"context"
	stdimage "image"
	"image/color"
	"image/jpeg"
	"path/filepath"
	"testing"

	"github.com/danielgtaylor/huma/v2"

	"github.com/s-frei/rezepte/service/internal/db/dbtest"
	"github.com/s-frei/rezepte/service/internal/image"
	"github.com/s-frei/rezepte/service/internal/recipe"
	"github.com/s-frei/rezepte/service/internal/transfer"
	"github.com/s-frei/rezepte/service/internal/user"
)

type env struct {
	recipes *recipe.Service
	images  *image.Service
	svc     *transfer.Service
	admin   user.User
	member  user.User
	dataDir string
}

func newEnv(t *testing.T) *env {
	t.Helper()
	ctx := context.Background()
	conn := dbtest.Open(t)
	users := user.NewService(conn, "")
	admin, err := users.Create(ctx, user.CreateParams{Username: "sam", Password: "pw", Role: user.RoleAdmin})
	if err != nil {
		t.Fatal(err)
	}
	member, err := users.Create(ctx, user.CreateParams{Username: "kim", Password: "pw", Role: user.RoleUser})
	if err != nil {
		t.Fatal(err)
	}
	dataDir := t.TempDir()
	imageDir := filepath.Join(dataDir, "images")
	recipes := recipe.NewService(conn, imageDir)
	images := image.NewService(conn, imageDir)
	// Unit tests do not go through huma; the handler tests cover the real validator.
	noValidate := func(map[string]any) []*huma.ErrorDetail { return nil }
	return &env{
		recipes: recipes, images: images, admin: admin, member: member, dataDir: dataDir,
		svc: transfer.NewService(recipes, images, dataDir, noValidate),
	}
}

func jpegBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := stdimage.NewRGBA(stdimage.Rect(0, 0, w, h))
	for x := range w {
		for y := range h {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 90, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func sampleInput(title string) recipe.Input {
	group := "Teig"
	return recipe.Input{
		Title:    title,
		Servings: 4,
		Tags:     []string{"dessert"},
		IngredientGroups: []recipe.IngredientGroup{{
			Name:        &group,
			Ingredients: []recipe.Ingredient{{Name: "Mehl"}, {Name: "Äpfel"}},
		}},
		Steps: []recipe.Step{{
			Text:       "Äpfel schälen, dann 20–25 Minuten backen.",
			Times:      []recipe.StepTime{{Phrase: "20–25 Minuten", Seconds: 1200, MaxSeconds: new(1500)}},
			References: []recipe.IngredientRef{{Word: "Äpfel", GroupName: &group, IngredientName: "Äpfel"}},
		}},
	}
}
