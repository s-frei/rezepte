package main

import (
	"bytes"
	"errors"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
)

// writeJPEG puts a w×h JPEG at path, creating its directory.
func writeJPEG(t *testing.T, path string, w, h int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, image.NewRGBA(image.Rect(0, 0, w, h)), nil); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

// dims decodes the JPEG header at path.
func dims(t *testing.T, path string) (int, int) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	cfg, err := jpeg.DecodeConfig(f)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return cfg.Width, cfg.Height
}

func TestRunShrinksEveryPhotoAndDropsStaleOnes(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	writeJPEG(t, filepath.Join(src, "en", "shepherd-s-pie", "1.jpg"), 2400, 1792)
	writeJPEG(t, filepath.Join(src, "en", "shepherd-s-pie", "2.jpg"), 1200, 900)
	if err := os.WriteFile(filepath.Join(src, "en", "shepherd-s-pie", "prompts.md"), []byte("# notes"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeJPEG(t, filepath.Join(dst, "en", "gone", "1.jpg"), 100, 100)

	if err := run(src, dst); err != nil {
		t.Fatalf("run: %v", err)
	}

	if w, h := dims(t, filepath.Join(dst, "en", "shepherd-s-pie", "1.jpg")); w != 1600 || h != 1195 {
		t.Errorf("1.jpg = %dx%d, want 1600x1195", w, h)
	}
	if w, h := dims(t, filepath.Join(dst, "en", "shepherd-s-pie", "2.jpg")); w != 1200 || h != 900 {
		t.Errorf("2.jpg = %dx%d, want 1200x900 (never upscaled)", w, h)
	}
	if _, err := os.Stat(filepath.Join(dst, "en", "shepherd-s-pie", "prompts.md")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("prompts.md copied into the embedded set: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "en", "gone")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("stale photo set kept: %v", err)
	}
}

func TestRunIsDeterministic(t *testing.T) {
	src := t.TempDir()
	writeJPEG(t, filepath.Join(src, "en", "x", "1.jpg"), 2000, 1500)
	a, b := t.TempDir(), t.TempDir()
	if err := run(src, a); err != nil {
		t.Fatal(err)
	}
	if err := run(src, b); err != nil {
		t.Fatal(err)
	}
	ga, _ := os.ReadFile(filepath.Join(a, "en", "x", "1.jpg"))
	gb, _ := os.ReadFile(filepath.Join(b, "en", "x", "1.jpg"))
	if !bytes.Equal(ga, gb) {
		t.Fatal("two runs over the same originals wrote different bytes")
	}
}

func TestRunNamesAnLFSPointer(t *testing.T) {
	src := t.TempDir()
	p := filepath.Join(src, "en", "x", "1.jpg")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	pointer := "version https://git-lfs.github.com/spec/v1\noid sha256:abc\nsize 3\n"
	if err := os.WriteFile(p, []byte(pointer), 0o644); err != nil {
		t.Fatal(err)
	}
	err := run(src, t.TempDir())
	if !errors.Is(err, errPointer) {
		t.Fatalf("err = %v, want errPointer", err)
	}
}
