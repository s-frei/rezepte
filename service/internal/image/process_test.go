package image

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	stdimage "image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func encodePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := stdimage.NewNRGBA(stdimage.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.NRGBA{200, 120, 40, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// pngHeaderOnly builds a PNG that consists of the signature and a valid
// IHDR claiming w×h - enough for image.DecodeConfig, never decodable in
// full. Lets the size guard be tested without allocating 20000² pixels.
func pngHeaderOnly(w, h uint32) []byte {
	ihdr := []byte("IHDR")
	ihdr = binary.BigEndian.AppendUint32(ihdr, w)
	ihdr = binary.BigEndian.AppendUint32(ihdr, h)
	ihdr = append(ihdr, 8, 2, 0, 0, 0) // depth 8, RGB, deflate, filter 0, no interlace
	out := []byte("\x89PNG\r\n\x1a\n")
	out = binary.BigEndian.AppendUint32(out, 13)
	out = append(out, ihdr...)
	return binary.BigEndian.AppendUint32(out, crc32.ChecksumIEEE(ihdr))
}

func TestDecodeAcceptsJPEGAndPNG(t *testing.T) {
	for name, data := range map[string][]byte{"jpeg": encodeJPEG(t, 80, 64), "png": encodePNG(t, 64, 80)} {
		img, err := decode(data)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if img.Bounds().Dx() < 64 || img.Bounds().Dy() < 64 {
			t.Fatalf("%s: bounds %v", name, img.Bounds())
		}
	}
}

func TestDecodeRejectsUnsupportedAndInvalid(t *testing.T) {
	cases := map[string]struct {
		data []byte
		want error
	}{
		"gif":             {[]byte("GIF89a\x01\x00\x01\x00\x00\x00\x00;"), ErrUnsupported},
		"text":            {[]byte("hello"), ErrUnsupported},
		"too small":       {encodePNG(t, 63, 64), ErrInvalid},
		"too large":       {pngHeaderOnly(12001, 100), ErrInvalid},
		"too many pixels": {pngHeaderOnly(12000, 6000), ErrTooLarge}, // 72 MP, both sides legal
		"truncated":       {encodeJPEG(t, 80, 80)[:200], ErrInvalid},
	}
	for name, c := range cases {
		_, err := decode(c.data)
		if !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", name, err, c.want)
		}
	}
}

func TestWebPDecoderRegistered(t *testing.T) {
	// No WebP encoder exists in the stdlib or x/image, so this only proves
	// the decoder is registered: a RIFF/WEBP prefix must reach the webp
	// decoder (and fail there) instead of falling through to ErrFormat.
	data := []byte("RIFF\x00\x00\x00\x00WEBPVP8 ")
	_, err := decode(data)
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid (webp decoder reached)", err)
	}
}

func TestFitNeverUpscales(t *testing.T) {
	cases := []struct{ w, h, max, ww, hh int }{
		{3000, 2000, 2400, 2400, 1600},
		{2000, 3000, 2400, 1600, 2400},
		{800, 600, 2400, 800, 600},
		{2400, 2400, 480, 480, 480},
		{3000, 2000, 1600, 1600, 1067},
	}
	for _, c := range cases {
		if w, h := fit(c.w, c.h, c.max); w != c.ww || h != c.hh {
			t.Errorf("fit(%d,%d,%d) = %d,%d want %d,%d", c.w, c.h, c.max, w, h, c.ww, c.hh)
		}
	}
}

func TestWriteVariants(t *testing.T) {
	dir := t.TempDir()
	src := stdimage.NewNRGBA(stdimage.Rect(0, 0, 3000, 2000))
	got, err := writeVariants(filepath.Join(dir, "r1"), "img1", src)
	if err != nil {
		t.Fatal(err)
	}
	if got.Width != 2400 || got.Height != 1600 || got.Size <= 0 {
		t.Fatalf("written = %+v", got)
	}
	want := map[string][2]int{"img1.jpg": {2400, 1600}, "img1_detail.jpg": {1600, 1067}, "img1_thumb.jpg": {960, 640}}
	for name, dims := range want {
		f, err := os.Open(filepath.Join(dir, "r1", name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		cfg, err := jpeg.DecodeConfig(f)
		_ = f.Close()
		if err != nil || cfg.Width != dims[0] || cfg.Height != dims[1] {
			t.Fatalf("%s: %dx%d (%v), want %v", name, cfg.Width, cfg.Height, err, dims)
		}
	}
	entries, _ := os.ReadDir(filepath.Join(dir, "r1"))
	if len(entries) != 3 {
		t.Fatalf("%d files in dir, want 3 (no temp files left)", len(entries))
	}
}

func TestWriteVariantsCleansUpOnError(t *testing.T) {
	t.Run("mkdir fails", func(t *testing.T) {
		dir := t.TempDir()
		// A file where the recipe directory should be makes MkdirAll fail.
		if err := os.WriteFile(filepath.Join(dir, "r1"), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := writeVariants(filepath.Join(dir, "r1"), "img1", stdimage.NewNRGBA(stdimage.Rect(0, 0, 64, 64))); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rename fails midway", func(t *testing.T) {
		recipeDir := filepath.Join(t.TempDir(), "r1")
		// A directory where the second variant's final file should be: the
		// first variant is written and renamed, then renaming the detail
		// temp file over a directory fails. Everything written so far must
		// be gone afterwards.
		blocker := filepath.Join(recipeDir, "img1_detail.jpg")
		if err := os.MkdirAll(blocker, 0o700); err != nil {
			t.Fatal(err)
		}
		_, err := writeVariants(recipeDir, "img1", stdimage.NewNRGBA(stdimage.Rect(0, 0, 64, 64)))
		if err == nil {
			t.Fatal("expected error")
		}
		// Not just any error: the first variant must already be on disk, so
		// cleanup has something to undo.
		if !strings.Contains(err.Error(), "rename") {
			t.Fatalf("err = %v, want the rename of the detail variant to fail", err)
		}
		entries, err := os.ReadDir(recipeDir)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			if e.Name() != "img1_detail.jpg" || !e.IsDir() {
				t.Errorf("leftover %q (dir: %v), want only the blocking directory", e.Name(), e.IsDir())
			}
		}
	})
}

func TestResizeFlattensTransparencyOntoWhite(t *testing.T) {
	src := stdimage.NewNRGBA(stdimage.Rect(0, 0, 100, 100)) // fully transparent
	out := resize(src, 50, 50)
	r, g, b, _ := out.At(25, 25).RGBA()
	if r>>8 != 255 || g>>8 != 255 || b>>8 != 255 {
		t.Fatalf("transparent pixel became %d,%d,%d, want white", r>>8, g>>8, b>>8)
	}
}
