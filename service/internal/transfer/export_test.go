package transfer_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"slices"
	"testing"

	"github.com/s-frei/rezepte/service/internal/recipe"
	"github.com/s-frei/rezepte/service/internal/transfer"
)

func TestExportWritesOneFolderPerRecipe(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	r, err := e.recipes.Create(ctx, e.admin.ID, sampleInput("Apfelstrudel"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.images.Upload(ctx, r.ID, e.admin, bytes.NewReader(jpegBytes(t, 120, 80))); err != nil {
		t.Fatal(err)
	}
	second, err := e.images.Upload(ctx, r.ID, e.admin, bytes.NewReader(jpegBytes(t, 80, 120)))
	if err != nil {
		t.Fatal(err)
	}
	if err := e.images.SetCover(ctx, r.ID, second.ID, e.admin); err != nil {
		t.Fatal(err)
	}

	f, err := e.svc.Export(ctx, []string{r.ID})
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(f, info.Size())
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	methods := map[string]uint16{}
	for _, zf := range zr.File {
		names = append(names, zf.Name)
		methods[zf.Name] = zf.Method
		// Dated by the recipe's last change, not the format's 1980 default.
		if zf.Modified.Year() < 2020 {
			t.Fatalf("%s dated %v", zf.Name, zf.Modified)
		}
	}
	want := []string{"apfelstrudel/recipe.json", "apfelstrudel/1.jpg", "apfelstrudel/2.jpg"}
	if !slices.Equal(names, want) {
		t.Fatalf("entries = %v, want %v", names, want)
	}
	if methods["apfelstrudel/1.jpg"] != zip.Store || methods["apfelstrudel/recipe.json"] != zip.Deflate {
		t.Fatalf("methods = %v", methods)
	}
	rc, err := zr.File[0].Open()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		t.Fatal(err)
	}
	var got transfer.File
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.Format != transfer.Format || got.Version != transfer.Version || got.Title != "Apfelstrudel" {
		t.Fatalf("header = %+v", got)
	}
	if !slices.Equal(got.Images, []string{"1.jpg", "2.jpg"}) || got.Cover == nil || *got.Cover != "2.jpg" {
		t.Fatalf("images = %v cover = %v", got.Images, got.Cover)
	}
	if bytes.Contains(raw, []byte("editPolicy")) || bytes.Contains(raw, []byte(r.ID)) {
		t.Fatalf("recipe.json leaks instance data: %s", raw)
	}
}

func TestExportRefusesUnknownIDWithoutLeavingAFile(t *testing.T) {
	e := newEnv(t)
	_, err := e.svc.Export(context.Background(), []string{"0190aaaa-0000-7000-8000-000000000000"})
	if !errors.Is(err, recipe.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	entries, err := os.ReadDir(e.dataDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, en := range entries {
		if !en.IsDir() {
			t.Fatalf("temp file left behind: %s", en.Name())
		}
	}
}
