//go:build unix

package pdf

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

//go:embed testdata/kaesespaetzle.json
var probeData []byte

//go:embed testdata/photos/1.jpg
var probePhoto []byte

// LoadTemplates sets r's templates from customDir before r serves anything.
// A custom dir that holds recipe.typ (or book.typ) is, as a whole, the tree
// for that template: its own lib.typ, fonts/ and packages/, never merged with
// built-in files. Each one is probed with a fixture recipe first; a template
// that fails is logged once and the built-in one stays, so PDFs keep
// working. runnable is false when a probe found that typst cannot run at all.
func LoadTemplates(ctx context.Context, r *Renderer, customDir string, log *slog.Logger) (runnable bool) {
	recipe, book := builtin(), builtin()
	defer func() { r.recipe, r.book = recipe, book }()
	if customDir == "" {
		return true
	}
	for _, t := range []struct {
		file string
		set  *TemplateSet
	}{{"recipe.typ", &recipe}, {"book.typ", &book}} {
		if _, err := os.Stat(filepath.Join(customDir, t.file)); err != nil {
			continue
		}
		custom := TemplateSet{FS: os.DirFS(customDir), Custom: true}
		err := probe(ctx, r, custom, t.file == "book.typ")
		if errors.Is(err, ErrCannotRun) {
			// Not the template's fault; PDFs start switched off.
			log.Warn("typst unavailable, custom PDF templates not checked", "err", err)
			recipe, book = builtin(), builtin()
			return false
		}
		if err != nil {
			log.Warn("custom PDF template failed, using the built-in one", "file", t.file, "err", err)
			continue
		}
		*t.set = custom
	}
	return true
}

// probe renders the fixture with tpl in its own pdf-* dir, removed afterwards,
// with photos and without, so an asset that only one branch uses (the
// built-in book's mascot) is checked at start.
func probe(ctx context.Context, r *Renderer, tpl TemplateSet, book bool) error {
	var d Data
	if err := json.Unmarshal(probeData, &d); err != nil {
		return fmt.Errorf("decode probe data: %w", err)
	}
	if book {
		d.Options.Bundle = BundleBook
		d.Book = &Book{Title: "Probe", Date: "2026-11-25", DateDisplay: "25.11.2026", CountDisplay: "1 Rezept"}
	}
	src, err := os.MkdirTemp(r.dataDir, "pdf-*")
	if err != nil {
		return fmt.Errorf("create probe dir: %w", err)
	}
	defer os.RemoveAll(src)
	photo := filepath.Join(src, "1.jpg")
	if err := os.WriteFile(photo, probePhoto, 0o600); err != nil {
		return fmt.Errorf("write probe photo: %w", err)
	}
	// Safe only because LoadTemplates runs before r serves anything, and it
	// sets both afterwards.
	r.recipe, r.book = tpl, tpl
	off := d
	off.Options.Photo = false
	for _, run := range []Data{d, off} {
		res, err := r.Render(ctx, run, map[string]string{"photos/1.jpg": photo})
		if err == nil {
			err = res.Close()
		}
		if err != nil && run.Options.Photo {
			return err
		}
		if err != nil {
			return fmt.Errorf("without photos: %w", err)
		}
	}
	return nil
}
