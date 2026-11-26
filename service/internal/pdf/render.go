//go:build unix

package pdf

import (
	"archive/zip"
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/s-frei/rezepte/service/internal/pdf/typstbin"
)

//go:embed templates
var templateFiles embed.FS

//go:embed fonts
var fontFiles embed.FS

// ErrCannotRun marks a failure no retry fixes: no typst at all, or one the
// OS refuses to start (a noexec temp dir, a binary for another platform).
// Only finding and starting typst return it; a work dir, template or photo
// error never does.
var ErrCannotRun = errors.New("typst cannot run")

// TemplateSet is one whole template tree: built-in or an operator's, never
// a mix, so a custom tree never imports a built-in file by accident.
type TemplateSet struct {
	FS     fs.FS
	Custom bool
}

// builtin returns the embedded tree that holds recipe.typ, book.typ and
// their shared lib.typ.
func builtin() TemplateSet {
	sub, err := fs.Sub(templateFiles, "templates")
	if err != nil {
		panic(err) // a constant path, so a programming error
	}
	return TemplateSet{FS: sub}
}

// Renderer runs typst over a Data in a per-request work dir in dataDir.
type Renderer struct {
	dataDir      string
	recipe, book TemplateSet
	sem          chan struct{}

	// Test hooks.
	timeout time.Duration // replaces the computed timeout when set
	started func(pid int) // called once typst runs
	bin     string        // replaces typstbin.Path when set
}

// NewRenderer renders with the built-in templates, which LoadTemplates may
// replace, and runs at most maxParallel typst processes at a time.
func NewRenderer(dataDir string, maxParallel int) *Renderer {
	return &Renderer{
		dataDir: dataDir, recipe: builtin(), book: builtin(),
		sem: make(chan struct{}, max(1, maxParallel)),
	}
}

// Result is a rendered file inside its work dir.
type Result struct {
	Path, ContentType string
	dir               string
}

// Close removes the work dir; the handler defers it after serving Path.
func (r *Result) Close() error { return os.RemoveAll(r.dir) }

// previewArgs make typst write page 1 as a PNG crisp in the pass-on sheet's
// preview (240 CSS px wide) on a 2x screen: 794 px for A4, 559 px for A5.
var previewArgs = []string{"--pages", "1", "--ppi", "96"}

// Preview renders page 1 of a single-recipe d as a PNG, through the same
// work dir, sandbox and semaphore as Render.
func (r *Renderer) Preview(ctx context.Context, d Data, photos map[string]string) (*Result, error) {
	return r.render(ctx, d, photos, true)
}

// Render writes d (with the photos, keyed by Photo.Path, from their absolute
// source files) into a fresh pdf-* dir and runs typst there: a PDF for single
// and book, a zip of one PDF per recipe for zip. The dir holds fonts/ and the
// --root of each typst run side by side, so a template reads its own tree
// and never a font file as data. Every way out except a returned Result
// removes the dir.
func (r *Renderer) Render(ctx context.Context, d Data, photos map[string]string) (*Result, error) {
	return r.render(ctx, d, photos, false)
}

func (r *Renderer) render(ctx context.Context, d Data, photos map[string]string, preview bool) (*Result, error) {
	bin := r.bin
	if bin == "" {
		var err error
		if bin, err = typstbin.Path(ctx); err != nil {
			if errors.Is(err, typstbin.ErrUnavailable) {
				return nil, fmt.Errorf("%w: %w", ErrCannotRun, err)
			}
			return nil, fmt.Errorf("typst: %w", err)
		}
	}
	select {
	case r.sem <- struct{}{}:
	case <-ctx.Done():
		return nil, fmt.Errorf("wait for a render slot: %w", ctx.Err())
	}
	defer func() { <-r.sem }()
	ctx, cancel := context.WithTimeout(ctx, r.timeoutFor(d))
	defer cancel()

	dir, err := os.MkdirTemp(r.dataDir, "pdf-*")
	if err != nil {
		return nil, fmt.Errorf("create work dir: %w", err)
	}
	keep := false
	defer func() {
		if !keep {
			_ = os.RemoveAll(dir)
		}
	}()

	// ponytail: copies the fonts (about 2 MB) per render; one shared dir
	// is the upgrade if that ever shows in a profile.
	fonts, _ := fs.Sub(fontFiles, "fonts") // a constant path, cannot fail
	fontDir := filepath.Join(dir, "fonts")
	if err := os.CopyFS(fontDir, fonts); err != nil {
		return nil, fmt.Errorf("copy fonts: %w", err)
	}
	res := &Result{dir: dir, ContentType: "application/pdf"}
	root := filepath.Join(dir, "root")
	switch {
	case preview:
		res.ContentType = "image/png"
		res.Path, err = r.renderOne(ctx, bin, fontDir, root, d, photos, r.recipe, true)
	case d.Options.Bundle == BundleZip:
		res.ContentType = "application/zip"
		res.Path, err = r.renderZip(ctx, bin, fontDir, dir, d, photos)
	case d.Options.Bundle == BundleBook:
		res.Path, err = r.renderOne(ctx, bin, fontDir, root, d, photos, r.book, false)
	default:
		res.Path, err = r.renderOne(ctx, bin, fontDir, root, d, photos, r.recipe, false)
	}
	if err != nil {
		return nil, err
	}
	keep = true
	return res, nil
}

func (r *Renderer) timeoutFor(d Data) time.Duration {
	if r.timeout > 0 {
		return r.timeout
	}
	if d.Options.Bundle == BundleSingle {
		return 30 * time.Second
	}
	return min(30*time.Second+time.Duration(len(d.Recipes))*time.Second, 10*time.Minute)
}

// renderOne prepares dir, the --root, and compiles its entry to dir/out.pdf,
// or its page 1 to dir/out.png for a preview.
func (r *Renderer) renderOne(ctx context.Context, bin, fontDir, dir string, d Data, photos map[string]string, tpl TemplateSet, preview bool) (string, error) {
	if err := os.Mkdir(dir, 0o700); err != nil {
		return "", fmt.Errorf("create root: %w", err)
	}
	entry, err := r.prepare(dir, d, photos, tpl)
	if err != nil {
		return "", err
	}
	out, extra := filepath.Join(dir, "out.pdf"), []string(nil)
	if preview {
		out, extra = filepath.Join(dir, "out.png"), previewArgs
	}
	fontPaths := []string{fontDir}
	if tpl.Custom {
		// The tree's own fonts, copied into dir with the rest of it.
		if st, err := os.Stat(filepath.Join(dir, "fonts")); err == nil && st.IsDir() {
			fontPaths = append(fontPaths, filepath.Join(dir, "fonts"))
		}
	}
	return out, r.compile(ctx, bin, fontPaths, dir, entry, out, extra)
}

// renderZip renders each recipe as its own single Data in a subdirectory and
// stores the PDFs, uncompressed, in dir/out.zip.
func (r *Renderer) renderZip(ctx context.Context, bin, fontDir, dir string, d Data, photos map[string]string) (string, error) {
	out := filepath.Join(dir, "out.zip")
	f, err := os.Create(out)
	if err != nil {
		return "", fmt.Errorf("create zip: %w", err)
	}
	err = r.fillZip(ctx, bin, fontDir, dir, d, photos, f)
	if cerr := f.Close(); err == nil && cerr != nil {
		err = fmt.Errorf("close zip: %w", cerr)
	}
	if err != nil {
		return "", err
	}
	return out, nil
}

func (r *Renderer) fillZip(ctx context.Context, bin, fontDir, dir string, d Data, photos map[string]string, f io.Writer) error {
	zw := zip.NewWriter(f)
	taken := map[string]int{}
	for i, rec := range d.Recipes {
		one := d
		one.Options.Bundle = BundleSingle
		one.Book = nil
		one.Recipes = []Recipe{rec}
		sub := filepath.Join(dir, strconv.Itoa(i))
		pdfPath, err := r.renderOne(ctx, bin, fontDir, sub, one, photos, r.recipe, false)
		if err != nil {
			return fmt.Errorf("recipe %s: %w", rec.ID, err)
		}
		w, err := zw.CreateHeader(&zip.FileHeader{
			Name: FileName(rec.Title, d.Labels.RecipeFallback, taken), Method: zip.Store, Modified: time.Now(),
		})
		if err != nil {
			return fmt.Errorf("zip entry: %w", err)
		}
		if err := copyInto(w, pdfPath); err != nil {
			return err
		}
		// Only the zip grows; each recipe's tree goes as soon as it is stored.
		if err := os.RemoveAll(sub); err != nil {
			return fmt.Errorf("remove recipe dir: %w", err)
		}
	}
	if err := zw.Close(); err != nil {
		return fmt.Errorf("finish zip: %w", err)
	}
	return nil
}

// prepare copies the template tree, data.json, the photos and the QR codes
// into dir and returns the entry file. It is what a test needs to run typst
// itself.
func (r *Renderer) prepare(dir string, d Data, photos map[string]string, tpl TemplateSet) (string, error) {
	entry := "recipe.typ"
	if d.Options.Bundle == BundleBook {
		entry = "book.typ"
	}
	if err := os.CopyFS(dir, tpl.FS); err != nil {
		return "", fmt.Errorf("copy template: %w", err)
	}
	// A copy, so the caller's recipes keep their QR and photo fields.
	recipes := make([]Recipe, len(d.Recipes))
	copy(recipes, d.Recipes)
	for i := range recipes {
		rec := &recipes[i]
		rec.QR = nil
		if d.Options.Footer {
			svg, err := qrSVG(rec.URL)
			if err != nil {
				return "", err
			}
			p := path.Join("qr", strconv.Itoa(i)+".svg")
			if err := writeFile(filepath.Join(dir, p), svg); err != nil {
				return "", err
			}
			rec.QR = &QR{URL: rec.URL, Path: p}
		}
		if rec.Photo != nil {
			src, ok := photos[rec.Photo.Path]
			if !ok || !strings.HasPrefix(rec.Photo.Path, "photos/") || !filepath.IsLocal(rec.Photo.Path) || path.Clean(rec.Photo.Path) != rec.Photo.Path {
				return "", fmt.Errorf("photo %q: no source or not under photos/", rec.Photo.Path)
			}
			if err := copyFile(src, filepath.Join(dir, rec.Photo.Path)); err != nil {
				return "", err
			}
		}
	}
	d.Recipes = recipes
	b, err := json.Marshal(d)
	if err != nil {
		return "", fmt.Errorf("encode data.json: %w", err)
	}
	if err := writeFile(filepath.Join(dir, "data.json"), b); err != nil {
		return "", err
	}
	return entry, nil
}

// compile runs typst sandboxed to root: no system fonts, packages only from
// root/packages, a dead proxy so a package download fails at once, and its
// own process group so a cancel kills everything it started.
func (r *Renderer) compile(ctx context.Context, bin string, fontPaths []string, root, entry, out string, extra []string) error {
	args := []string{"compile", "--root", root}
	for _, p := range fontPaths {
		args = append(args, "--font-path", p)
	}
	args = append(args, "--ignore-system-fonts",
		"--package-path", filepath.Join(root, "packages"),
		"--package-cache-path", filepath.Join(root, ".cache"))
	args = append(append(args, extra...), filepath.Join(root, entry), out)
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = []string{"HOME=" + root, "HTTPS_PROXY=http://127.0.0.1:9", "HTTP_PROXY=http://127.0.0.1:9", "NO_PROXY="}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 2 * time.Second
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		if errors.Is(err, syscall.EACCES) || errors.Is(err, syscall.ENOEXEC) {
			return fmt.Errorf("%w: start typst: %w", ErrCannotRun, err)
		}
		return fmt.Errorf("start typst: %w", err)
	}
	if r.started != nil {
		r.started(cmd.Process.Pid)
	}
	if err := cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("typst: %w", ctx.Err())
		}
		return fmt.Errorf("typst: %w: %s", err, bytes.TrimSpace(stderr.Bytes()))
	}
	return nil
}

func writeFile(p string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(p), err)
	}
	if err := os.WriteFile(p, b, 0o600); err != nil { //nolint:gosec // G703: prepare builds p inside the work dir and checks a photo's path
		return fmt.Errorf("write %s: %w", p, err)
	}
	return nil
}

// copyFile copies a cover photo, about 100 KB, into the work dir.
func copyFile(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read %s: %w", src, err)
	}
	return writeFile(dst, b)
}

func copyInto(w io.Writer, src string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open %s: %w", src, err)
	}
	defer in.Close()
	if _, err := io.Copy(w, in); err != nil {
		return fmt.Errorf("copy %s: %w", src, err)
	}
	return nil
}
