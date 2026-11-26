//go:build unix

package pdf

import (
	"bytes"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/s-frei/rezepte/service/internal/pdf/typstbin"
)

const sentinel = "TOP-SECRET-SENTINEL"

func loadCustom(t *testing.T, files map[string]string) (recipe, book TemplateSet, r *Renderer, logs string) {
	t.Helper()
	r = newTestRenderer(t, builtin())
	parent := t.TempDir()
	dir := filepath.Join(parent, "tpl")
	if err := os.WriteFile(filepath.Join(parent, "sentinel.txt"), []byte(sentinel), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, body := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var buf bytes.Buffer
	LoadTemplates(t.Context(), r, dir, slog.New(slog.NewTextHandler(&buf, nil)))
	return r.recipe, r.book, r, buf.String()
}

func TestLoadTemplatesDefault(t *testing.T) {
	r := newTestRenderer(t, builtin())
	LoadTemplates(t.Context(), r, "", slog.New(slog.DiscardHandler))
	if r.recipe.Custom || r.book.Custom {
		t.Error("built-in expected")
	}
}

func TestLoadTemplatesValidRecipeOnly(t *testing.T) {
	recipe, book, r, logs := loadCustom(t, map[string]string{"recipe.typ": "#let d = json(\"data.json\")\nHello #d.recipes.at(0).title"})
	if !recipe.Custom || book.Custom {
		t.Errorf("recipe custom=%v book custom=%v, want true false", recipe.Custom, book.Custom)
	}
	if logs != "" {
		t.Errorf("unexpected log: %s", logs)
	}
	assertNoLeftovers(t, r)
}

func TestLoadTemplatesBrokenFallsBack(t *testing.T) {
	recipe, _, r, logs := loadCustom(t, map[string]string{"recipe.typ": "#let = (("})
	if recipe.Custom {
		t.Error("broken template accepted")
	}
	if n := strings.Count(logs, "custom PDF template failed"); n != 1 || !strings.Contains(logs, "recipe.typ") {
		t.Errorf("want one warning for recipe.typ, got: %s", logs)
	}
	assertNoLeftovers(t, r)
}

func TestLoadTemplatesBook(t *testing.T) {
	_, book, r, logs := loadCustom(t, map[string]string{"book.typ": "#let d = json(\"data.json\")\n#d.book.title"})
	if !book.Custom || logs != "" {
		t.Errorf("book custom=%v logs=%s", book.Custom, logs)
	}
	assertNoLeftovers(t, r)
}

// The built-in title page draws the mascot only without photos; a custom
// book copied without it must fail at start, not on the first such book.
func TestLoadTemplatesBookProbedWithoutPhotos(t *testing.T) {
	_, book, r, logs := loadCustom(t, map[string]string{"book.typ": "#let d = json(\"data.json\")\n#if d.options.photo [ok] else { image(\"mascot.png\") }"})
	if book.Custom || !strings.Contains(logs, "book.typ") {
		t.Errorf("book custom=%v logs=%s, want the built-in book and a warning", book.Custom, logs)
	}
	assertNoLeftovers(t, r)
}

func TestLoadTemplatesRecipeProbedWithoutPhotos(t *testing.T) {
	recipe, _, r, logs := loadCustom(t, map[string]string{"recipe.typ": "#let d = json(\"data.json\")\n#if d.options.photo [ok] else { image(\"missing.png\") }"})
	if recipe.Custom || !strings.Contains(logs, "without photos") {
		t.Errorf("recipe custom=%v logs=%s, want the built-in recipe and a warning", recipe.Custom, logs)
	}
	assertNoLeftovers(t, r)
}

func TestCustomFontsPassedAsFontPath(t *testing.T) {
	args := filepath.Join(t.TempDir(), "args")
	bin := filepath.Join(t.TempDir(), "typst")
	script := "#!/bin/sh\necho \"$@\" >> " + args + "\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	r := newTestRenderer(t, builtin())
	r.bin = bin
	custom := TemplateSet{FS: fstest.MapFS{
		"recipe.typ":   {Data: []byte("x")},
		"fonts/my.ttf": {Data: []byte("x")},
	}, Custom: true}
	var res0 string
	for _, tpl := range []TemplateSet{custom, tree("x")} {
		r.recipe = tpl
		res, err := r.Render(t.Context(), fixture(t), fixturePhotos(t))
		if err != nil {
			t.Fatal(err)
		}
		if res0 == "" {
			res0 = res.Path
		}
		res.Close()
	}
	b, _ := os.ReadFile(args)
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) != 2 || strings.Count(lines[0], "--font-path") != 2 || strings.Count(lines[1], "--font-path") != 1 {
		t.Errorf("font paths wrong: %q", lines)
	}
	if !strings.Contains(lines[0], "--font-path "+filepath.Join(filepath.Dir(res0), "fonts")+" ") || !strings.Contains(lines[0], "--ignore-system-fonts") {
		t.Errorf("custom fonts dir missing: %q", lines[0])
	}
}

// A font family only the custom fonts/ ships must resolve under
// --ignore-system-fonts: typst warns "unknown font family" on stderr when not.
func TestCustomFontResolves(t *testing.T) {
	r := newTestRenderer(t, builtin())
	bin := r.bin
	if bin == "" {
		var err error
		if bin, err = typstbin.Path(t.Context()); err != nil {
			t.Skip(err)
		}
	}
	ttf, err := fontFiles.ReadFile("fonts/karla/Karla-Regular.ttf")
	if err != nil {
		t.Fatal(err)
	}
	tpl := TemplateSet{Custom: true, FS: fstest.MapFS{
		"recipe.typ":              {Data: []byte("#set text(font: \"Karla\")\nHello")},
		"fonts/Karla-Regular.ttf": {Data: ttf},
	}}
	run := func(withFonts bool) string {
		work := t.TempDir()
		if _, err := r.prepare(work, fixture(t), fixturePhotos(t), tpl); err != nil {
			t.Fatal(err)
		}
		args := []string{"compile", "--root", work, "--ignore-system-fonts"}
		if withFonts {
			args = append(args, "--font-path", filepath.Join(work, "fonts"))
		}
		out, err := exec.CommandContext(t.Context(), bin, append(args, filepath.Join(work, "recipe.typ"), filepath.Join(work, "o.pdf"))...).CombinedOutput()
		if err != nil {
			t.Fatalf("typst: %v: %s", err, out)
		}
		return strings.ToLower(string(out))
	}
	if !strings.Contains(run(false), "unknown font family") {
		t.Fatal("control: typst did not warn without the font path")
	}
	if out := run(true); strings.Contains(out, "unknown font family") {
		t.Errorf("custom font not found: %s", out)
	}
}

// The probe's root is r.dataDir/pdf-*/root beside pdf-*/fonts, so each
// relative path lands on a real file; each case would succeed if --root
// were wider.
func TestCustomTemplateCannotReadOutsideItsRoot(t *testing.T) {
	r := newTestRenderer(t, builtin())
	abs := filepath.Join(t.TempDir(), "abs-sentinel.txt")
	rel := filepath.Join(r.dataDir, "sentinel.txt")
	rel2 := filepath.Join(filepath.Dir(r.dataDir), "rezepte-sentinel.txt")
	for _, p := range []string{abs, rel, rel2} {
		if err := os.WriteFile(p, []byte(sentinel), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Remove(p) })
	}
	for name, src := range map[string]string{
		"fonts":          `#read("../fonts/karla/Karla-Regular.ttf")`,
		"data-dir":       `#read("../../sentinel.txt")`,
		"parent-of-data": `#read("../../../rezepte-sentinel.txt")`,
		"absolute":       `#read("` + abs + `")`,
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "recipe.typ"), []byte(src), 0o644); err != nil {
				t.Fatal(err)
			}
			var buf bytes.Buffer
			LoadTemplates(t.Context(), r, dir, slog.New(slog.NewTextHandler(&buf, nil)))
			if r.recipe.Custom {
				t.Error("template reading outside its root was accepted")
			}
			if !strings.Contains(buf.String(), "custom PDF template failed") || strings.Contains(buf.String(), sentinel) {
				t.Errorf("want a failure without the sentinel, got: %s", buf.String())
			}
			assertNoLeftovers(t, r)
		})
	}
}

func TestLoadTemplatesCannotRun(t *testing.T) {
	r := NewRenderer(t.TempDir(), 1)
	r.bin = filepath.Join(t.TempDir(), "typst")
	if err := os.WriteFile(r.bin, []byte("#!/bin/sh\n"), 0o600); err != nil { // EACCES
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "recipe.typ"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	runnable := LoadTemplates(t.Context(), r, dir, slog.New(slog.NewTextHandler(&buf, nil)))
	if runnable || r.recipe.Custom || r.book.Custom {
		t.Errorf("runnable=%v custom=%v, want false false", runnable, r.recipe.Custom)
	}
	if logs := buf.String(); !strings.Contains(logs, "typst unavailable") || strings.Contains(logs, "custom PDF template failed") {
		t.Errorf("logs: %s", logs)
	}
	assertNoLeftovers(t, r)
}
