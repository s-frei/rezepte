package transfer

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	stdimage "image"
	"image/jpeg"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"
)

func buildZip(t *testing.T, names []string, entries map[string][]byte) *zip.Reader {
	t.Helper()
	raw := zipBytes(t, names, entries)
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatal(err)
	}
	return zr
}

func zipBytes(t *testing.T, names []string, entries map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, name := range names {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(entries[name]); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func tinyJPEG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, stdimage.NewRGBA(stdimage.Rect(0, 0, 64, 64)), nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func recipeJSON(t *testing.T, mutate func(m map[string]any)) []byte {
	t.Helper()
	m := map[string]any{
		"format": Format, "version": Version, "title": "Focaccia", "description": "", "servings": 2,
		"tags": []string{}, "steps": []any{},
		"ingredientGroups": []any{map[string]any{"name": nil, "ingredients": []any{map[string]any{"name": "Mehl"}}}},
		"images":           []string{"1.jpg"}, "cover": "1.jpg",
	}
	if mutate != nil {
		mutate(m)
	}
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func checker() *Service {
	return &Service{validate: func(raw map[string]any) []*huma.ErrorDetail {
		if raw["title"] == "" {
			return []*huma.ErrorDetail{{Location: "body.title", Message: "expected length >= 1"}}
		}
		return nil
	}}
}

func TestCheckAcceptsAFolderAndSkipsMacOSClutter(t *testing.T) {
	zr := buildZip(t,
		[]string{"focaccia/recipe.json", "focaccia/1.jpg", "__MACOSX/focaccia/._1.jpg", "focaccia/.DS_Store"},
		map[string][]byte{"focaccia/recipe.json": recipeJSON(t, nil), "focaccia/1.jpg": tinyJPEG(t)})
	folders, err := checker().check(zr)
	if err != nil {
		t.Fatal(err)
	}
	if len(folders) != 1 || folders[0].name != "focaccia" || folders[0].file.Title != "Focaccia" || folders[0].photos["1.jpg"] == nil {
		t.Fatalf("folders = %+v", folders)
	}
}

func TestCheckRejects(t *testing.T) {
	jpg := tinyJPEG(t)
	noPhotos := func(m map[string]any) { m["images"] = []string{}; m["cover"] = nil }
	cases := []struct {
		name     string
		names    []string
		entries  map[string][]byte
		location string
		msg      string
	}{
		{"path traversal", []string{"../evil/recipe.json"}, map[string][]byte{"../evil/recipe.json": recipeJSON(t, nil)}, "../evil/recipe.json", "unexpected entry"},
		{"absolute path", []string{"/a/recipe.json"}, map[string][]byte{"/a/recipe.json": recipeJSON(t, nil)}, "/a/recipe.json", "unexpected entry"},
		{"nested folder", []string{"a/b/recipe.json"}, map[string][]byte{"a/b/recipe.json": recipeJSON(t, nil)}, "a/b/recipe.json", "unexpected entry"},
		{"file at the root", []string{"recipe.json"}, map[string][]byte{"recipe.json": recipeJSON(t, nil)}, "recipe.json", "unexpected entry"},
		{"other file", []string{"a/notes.txt"}, map[string][]byte{"a/notes.txt": []byte("x")}, "a/notes.txt", "unexpected entry"},
		{"folder without recipe.json", []string{"a/1.jpg"}, map[string][]byte{"a/1.jpg": jpg}, "a/recipe.json", "missing"},
		{"not json", []string{"a/recipe.json"}, map[string][]byte{"a/recipe.json": []byte("{")}, "a/recipe.json", "not valid JSON"},
		{"unknown format", []string{"a/recipe.json"}, map[string][]byte{"a/recipe.json": recipeJSON(t, func(m map[string]any) { noPhotos(m); m["format"] = "x" })}, "a/recipe.json", "unknown format"},
		{"newer version", []string{"a/recipe.json"}, map[string][]byte{"a/recipe.json": recipeJSON(t, func(m map[string]any) { noPhotos(m); m["version"] = 2 })}, "a/recipe.json", "newer version"},
		{"missing version", []string{"a/recipe.json"}, map[string][]byte{"a/recipe.json": recipeJSON(t, func(m map[string]any) { noPhotos(m); delete(m, "version") })}, "a/recipe.json", "unsupported version"},
		{"string version", []string{"a/recipe.json"}, map[string][]byte{"a/recipe.json": recipeJSON(t, func(m map[string]any) { noPhotos(m); m["version"] = "1" })}, "a/recipe.json", "unsupported version"},
		{"zero version", []string{"a/recipe.json"}, map[string][]byte{"a/recipe.json": recipeJSON(t, func(m map[string]any) { noPhotos(m); m["version"] = 0 })}, "a/recipe.json", "unsupported version"},
		{"schema", []string{"a/recipe.json"}, map[string][]byte{"a/recipe.json": recipeJSON(t, func(m map[string]any) { noPhotos(m); m["title"] = "" })}, "a/recipe.json: body.title", "expected length"},
		{"unresolved reference", []string{"a/recipe.json"}, map[string][]byte{"a/recipe.json": recipeJSON(t, func(m map[string]any) {
			noPhotos(m)
			m["steps"] = []any{map[string]any{"text": "Salz dazu.", "references": []any{map[string]any{"word": "Salz", "groupName": nil, "ingredientName": "Salz"}}}}
		})}, "a/recipe.json: body.steps[0].references[0].ingredientName", `no ingredient "Salz"`},
		{"missing photo", []string{"a/recipe.json"}, map[string][]byte{"a/recipe.json": recipeJSON(t, nil)}, "a/1.jpg", "missing"},
		{"cover not listed", []string{"a/recipe.json", "a/1.jpg"}, map[string][]byte{"a/recipe.json": recipeJSON(t, func(m map[string]any) { m["cover"] = "2.jpg" }), "a/1.jpg": jpg}, "a/recipe.json: cover", "not one of images"},
		{"not a jpeg", []string{"a/recipe.json", "a/1.jpg"}, map[string][]byte{"a/recipe.json": recipeJSON(t, nil), "a/1.jpg": []byte("GIF89a")}, "a/1.jpg", "not a JPEG"},
		{"empty zip", nil, nil, "", "no recipes"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := checker().check(buildZip(t, c.names, c.entries))
			var te *Error
			if !errors.As(err, &te) {
				t.Fatalf("err = %v, want *Error", err)
			}
			if te.Location != c.location || !strings.Contains(te.Msg, c.msg) {
				t.Fatalf("got %q %q, want %q containing %q", te.Location, te.Msg, c.location, c.msg)
			}
		})
	}
}

func TestCheckRejectsAnOversizedRecipeJSONWhileReading(t *testing.T) {
	big := recipeJSON(t, func(m map[string]any) { m["description"] = strings.Repeat("x", maxJSONBytes) })
	_, err := checker().check(buildZip(t, []string{"a/recipe.json"}, map[string][]byte{"a/recipe.json": big}))
	var te *Error
	if !errors.As(err, &te) || !strings.Contains(te.Msg, "larger than") {
		t.Fatalf("err = %v", err)
	}
}
