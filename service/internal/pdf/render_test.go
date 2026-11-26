//go:build unix

package pdf

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"testing/fstest"
	"time"

	"github.com/s-frei/rezepte/service/internal/pdf/typstbin"
)

// slow keeps typst busy for well over a second.
const slow = "#for i in range(1000000000) {}\n"

func tree(src string) TemplateSet {
	return TemplateSet{FS: fstest.MapFS{"recipe.typ": {Data: []byte(src)}, "book.typ": {Data: []byte(src)}}}
}

func fixture(t *testing.T) Data {
	t.Helper()
	b, err := os.ReadFile("testdata/kaesespaetzle.json")
	if err != nil {
		t.Fatal(err)
	}
	var d Data
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatal(err)
	}
	return d
}

func fixturePhotos(t *testing.T) map[string]string {
	t.Helper()
	p, err := filepath.Abs("testdata/photos/1.jpg")
	if err != nil {
		t.Fatal(err)
	}
	return map[string]string{"photos/1.jpg": p}
}

// newTestRenderer skips when no typst exists and keeps the fonts in the
// test's temp dir; it allows one typst at a time.
func newTestRenderer(t *testing.T, tpl TemplateSet) *Renderer {
	t.Helper()
	if _, err := typstbin.Path(t.Context()); errors.Is(err, typstbin.ErrUnavailable) {
		t.Skip("typst unavailable")
	}
	r := NewRenderer(t.TempDir(), 1)
	r.recipe, r.book = tpl, tpl
	return r
}

func assertNoLeftovers(t *testing.T, r *Renderer) {
	t.Helper()
	if m, _ := filepath.Glob(filepath.Join(r.dataDir, "pdf-*")); len(m) > 0 {
		t.Errorf("left behind: %v", m)
	}
}

func assertPDF(t *testing.T, b []byte) {
	t.Helper()
	if !bytes.HasPrefix(b, []byte("%PDF-")) {
		t.Errorf("not a PDF: %q", b[:min(len(b), 16)])
	}
}

func TestRenderSingleProducesPDF(t *testing.T) {
	r := newTestRenderer(t, builtin())
	res, err := r.Render(t.Context(), fixture(t), fixturePhotos(t))
	if err != nil {
		t.Fatal(err)
	}
	if res.ContentType != "application/pdf" {
		t.Errorf("content type = %q", res.ContentType)
	}
	b, err := os.ReadFile(res.Path)
	if err != nil {
		t.Fatal(err)
	}
	assertPDF(t, b)
	if err := res.Close(); err != nil {
		t.Fatal(err)
	}
	assertNoLeftovers(t, r)
}

// TestPreviewIsPageOnePNG renders a two-page document: typst refuses to
// write two pages to one PNG, so a pass means page 1 alone, at 96 ppi.
func TestPreviewIsPageOnePNG(t *testing.T) {
	r := newTestRenderer(t, tree("#set page(width: 210mm, height: 297mm)\nA #pagebreak() B"))
	res, err := r.Preview(t.Context(), fixture(t), fixturePhotos(t))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Close()
	f, err := os.Open(res.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	cfg, err := png.DecodeConfig(f)
	if err != nil || res.ContentType != "image/png" || cfg.Width != 794 || cfg.Height != 1123 {
		t.Errorf("%s %dx%d: %v", res.ContentType, cfg.Width, cfg.Height, err)
	}
}

func TestRenderZip(t *testing.T) {
	r := newTestRenderer(t, builtin())
	d := fixture(t)
	d.Options.Bundle = BundleZip
	second := d.Recipes[0]
	second.Photo = nil
	d.Recipes = append(d.Recipes, second)
	res, err := r.Render(t.Context(), d, fixturePhotos(t))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Close()
	if res.ContentType != "application/zip" {
		t.Errorf("content type = %q", res.ContentType)
	}
	zr, err := zip.OpenReader(res.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	var names []string
	for _, f := range zr.File {
		names = append(names, f.Name)
		if f.Method != zip.Store {
			t.Errorf("%s is compressed", f.Name)
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		var b bytes.Buffer
		_, _ = b.ReadFrom(rc)
		_ = rc.Close()
		assertPDF(t, b.Bytes())
	}
	if got := strings.Join(names, "|"); got != "Käsespätzle.pdf|Käsespätzle (2).pdf" {
		t.Errorf("entries = %s", got)
	}
}

func TestRenderBookUsesBookTemplate(t *testing.T) {
	r := newTestRenderer(t, tree(""))
	r.book = TemplateSet{FS: fstest.MapFS{"book.typ": {Data: []byte("book")}}}
	d := fixture(t)
	d.Options.Bundle = BundleBook
	res, err := r.Render(t.Context(), d, fixturePhotos(t))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Close()
	if _, err := os.Stat(filepath.Join(filepath.Dir(res.Path), "recipe.typ")); err == nil {
		t.Error("the recipe tree was copied into a book render")
	}
}

func TestNoLeftoversAfterSuccess(t *testing.T) {
	r := newTestRenderer(t, tree("ok"))
	res, err := r.Render(t.Context(), fixture(t), fixturePhotos(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := res.Close(); err != nil {
		t.Fatal(err)
	}
	assertNoLeftovers(t, r)
}

func TestNoLeftoversAfterTypstError(t *testing.T) {
	r := newTestRenderer(t, tree(`#panic("x")`))
	_, err := r.Render(t.Context(), fixture(t), fixturePhotos(t))
	if err == nil || !strings.Contains(err.Error(), "panicked") {
		t.Errorf("err = %v, want typst's stderr in it", err)
	}
	assertNoLeftovers(t, r)
}

func TestNoLeftoversAfterCancel(t *testing.T) {
	r := newTestRenderer(t, tree(slow))
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	time.AfterFunc(200*time.Millisecond, cancel)
	start := time.Now()
	if _, err := r.Render(ctx, fixture(t), fixturePhotos(t)); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Errorf("cancel took %v", d)
	}
	assertNoLeftovers(t, r)
}

// TestCancelStopsProcess runs a fake typst that forks a child and waits: a
// cancel must kill the child too, which only the process-group kill does.
func TestCancelStopsProcess(t *testing.T) {
	r := newTestRenderer(t, tree(""))
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "child.pid")
	r.bin = filepath.Join(dir, "typst")
	script := "#!/bin/sh\n/bin/sleep 60 &\necho $! > " + pidFile + ".tmp && mv " + pidFile + ".tmp " + pidFile + "\nwait\n"
	if err := os.WriteFile(r.bin, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	// Cancel only once the script has published its child's pid.
	go func() {
		for end := time.Now().Add(20 * time.Second); time.Now().Before(end) && ctx.Err() == nil; time.Sleep(10 * time.Millisecond) {
			if b, err := os.ReadFile(pidFile); err == nil && len(bytes.TrimSpace(b)) > 0 {
				break
			}
		}
		cancel()
	}()
	if _, err := r.Render(ctx, fixture(t), fixturePhotos(t)); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
	b, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	child, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = syscall.Kill(child, syscall.SIGKILL) })
	// The orphaned child is reaped by init; until then kill(pid, 0) succeeds.
	deadline := time.Now().Add(20 * time.Second)
	for {
		err := syscall.Kill(child, 0)
		if errors.Is(err, syscall.ESRCH) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("child %d survived the cancel (kill: %v)", child, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestNoLeftoversAfterTimeout(t *testing.T) {
	r := newTestRenderer(t, tree(slow))
	r.timeout = 100 * time.Millisecond
	_, err := r.Render(t.Context(), fixture(t), fixturePhotos(t))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want DeadlineExceeded", err)
	}
	assertNoLeftovers(t, r)
}

func TestParallelLimit(t *testing.T) {
	r := newTestRenderer(t, tree("#for i in range(1000000) {}\n"))
	var mu sync.Mutex
	var pids []int
	r.started = func(pid int) {
		mu.Lock()
		defer mu.Unlock()
		if len(pids) == 1 {
			if err := syscall.Kill(pids[0], 0); !errors.Is(err, syscall.ESRCH) {
				t.Errorf("second typst started while the first runs (kill: %v)", err)
			}
		}
		pids = append(pids, pid)
	}
	var wg sync.WaitGroup
	for range 2 {
		wg.Go(func() {
			res, err := r.Render(t.Context(), fixture(t), fixturePhotos(t))
			if err != nil {
				t.Error(err)
				return
			}
			_ = res.Close()
		})
	}
	wg.Wait()
	if len(pids) != 2 {
		t.Errorf("%d renders started, want 2", len(pids))
	}
}

func TestPackageDownloadBlocked(t *testing.T) {
	r := newTestRenderer(t, tree(`#import "@preview/tiaoma:0.3.0"`))
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	_, err := r.Render(ctx, fixture(t), fixturePhotos(t))
	if err == nil || !strings.Contains(err.Error(), "failed to download package") {
		t.Errorf("err = %v, want a fast download failure", err)
	}
	assertNoLeftovers(t, r)
}

func TestSweepTemp(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"pdf-1", "pdf-2/sub", "export-1.zip", "keep"} {
		if err := os.MkdirAll(filepath.Join(dir, name), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "pdf-3"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := SweepTemp(dir); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	var left []string
	for _, e := range entries {
		left = append(left, e.Name())
	}
	if got := strings.Join(left, ","); got != "export-1.zip,keep" {
		t.Errorf("left = %s", got)
	}
}

func TestQROnlyWithFooter(t *testing.T) {
	r := NewRenderer(t.TempDir(), 1)
	for _, footer := range []bool{true, false} {
		dir := t.TempDir()
		d := fixture(t)
		d.Options.Footer = footer
		if _, err := r.prepare(dir, d, fixturePhotos(t), tree("")); err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(filepath.Join(dir, "data.json"))
		if err != nil {
			t.Fatal(err)
		}
		var got Data
		if err := json.Unmarshal(b, &got); err != nil {
			t.Fatal(err)
		}
		qr := got.Recipes[0].QR
		_, statErr := os.Stat(filepath.Join(dir, "qr"))
		if !footer {
			if qr != nil || statErr == nil {
				t.Errorf("footer off: qr = %+v, qr dir exists: %v", qr, statErr == nil)
			}
			continue
		}
		if qr == nil || qr.Path != "qr/0.svg" || qr.URL != d.Recipes[0].URL {
			t.Fatalf("footer on: qr = %+v", qr)
		}
		svg, err := os.ReadFile(filepath.Join(dir, qr.Path))
		if err != nil || !bytes.HasPrefix(svg, []byte("<svg")) {
			t.Errorf("qr svg = %.40q, %v", svg, err)
		}
		if _, err := os.Stat(filepath.Join(dir, "photos/1.jpg")); err != nil {
			t.Errorf("photo not copied: %v", err)
		}
	}
}

func TestPrepareRejectsUnknownPhoto(t *testing.T) {
	r := NewRenderer(t.TempDir(), 1)
	d := fixture(t)
	d.Recipes[0].Photo.Path = "../escape.jpg"
	if _, err := r.prepare(t.TempDir(), d, map[string]string{"../escape.jpg": "/etc/hosts"}, tree("")); err == nil {
		t.Error("a photo path outside the work dir was accepted")
	}
}
