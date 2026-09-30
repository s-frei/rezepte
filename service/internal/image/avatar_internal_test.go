package image

import (
	"bytes"
	"context"
	"errors"
	stdimage "image"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
)

func TestParseCrop(t *testing.T) {
	if c, err := ParseCrop(""); c != nil || err != nil {
		t.Fatalf("empty: got %v, %v; want nil, nil", c, err)
	}
	c, err := ParseCrop("0.1,0.25,0.5")
	if err != nil || *c != (Crop{X: 0.1, Y: 0.25, Size: 0.5}) {
		t.Fatalf("got %v, %v", c, err)
	}
	for _, bad := range []string{"0.1,0.2", "0.1,0.2,0.3,0.4", "a,b,c", "-0.1,0,0.5", "0,0,1.5", "0,0,0", "NaN,0,0.5"} {
		if _, err := ParseCrop(bad); !errors.Is(err, ErrBadCrop) {
			t.Errorf("%q: got %v, want ErrBadCrop", bad, err)
		}
	}
}

func TestCropRectCentersWithoutCrop(t *testing.T) {
	got, err := cropRect(400, 200, nil)
	if err != nil || got != stdimage.Rect(100, 0, 300, 200) {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestCropRectConvertsFractions(t *testing.T) {
	got, err := cropRect(400, 200, &Crop{X: 0.25, Y: 0.1, Size: 0.5})
	if err != nil || got != stdimage.Rect(100, 20, 200, 120) {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestCropRectClampsRounding(t *testing.T) {
	// 0.5 * 300 + 0.75004 * 200 = 300.008: past the right edge by less than
	// half a pixel, which is rounding in the browser, not a bad request.
	got, err := cropRect(300, 200, &Crop{X: 0.5, Y: 0, Size: 0.75004})
	if err != nil {
		t.Fatal(err)
	}
	if got.Max.X > 300 || got.Dx() != got.Dy() {
		t.Fatalf("got %v, want a square inside 300x200", got)
	}
}

func TestCropRectRejects(t *testing.T) {
	for name, c := range map[string]*Crop{
		"past the right edge": {X: 0.8, Y: 0, Size: 0.5},
		"past the bottom":     {X: 0, Y: 0.6, Size: 0.5},
		"below 64 px":         {X: 0, Y: 0, Size: 0.1},
	} {
		if _, err := cropRect(400, 200, c); !errors.Is(err, ErrBadCrop) {
			t.Errorf("%s: got %v, want ErrBadCrop", name, err)
		}
	}
}

func TestRenderAvatarWritesSquare(t *testing.T) {
	svc := &Service{decodeSlots: make(chan struct{}, 1)}
	path := filepath.Join(t.TempDir(), "u", "a.jpg")
	if err := svc.RenderAvatar(context.Background(), bytes.NewReader(encodePNG(t, 800, 400)), nil, path); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	cfg, err := jpeg.DecodeConfig(f)
	if err != nil || cfg.Width != avatarSide || cfg.Height != avatarSide {
		t.Fatalf("got %dx%d, %v; want %dx%d", cfg.Width, cfg.Height, err, avatarSide, avatarSide)
	}
	if info, _ := os.Stat(path); info.Mode().Perm() != 0o600 {
		t.Errorf("mode %v, want 0600", info.Mode().Perm())
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("tmp file left behind: %v", err)
	}
}

func TestRenderAvatarNeverUpscales(t *testing.T) {
	svc := &Service{decodeSlots: make(chan struct{}, 1)}
	path := filepath.Join(t.TempDir(), "a.jpg")
	if err := svc.RenderAvatar(context.Background(), bytes.NewReader(encodePNG(t, 100, 100)), nil, path); err != nil {
		t.Fatal(err)
	}
	f, _ := os.Open(path)
	defer f.Close()
	cfg, _ := jpeg.DecodeConfig(f)
	if cfg.Width != 100 {
		t.Fatalf("got %d px, want 100", cfg.Width)
	}
}

func TestRenderAvatarRejectsBadCropBeforeWriting(t *testing.T) {
	svc := &Service{decodeSlots: make(chan struct{}, 1)}
	path := filepath.Join(t.TempDir(), "a.jpg")
	err := svc.RenderAvatar(context.Background(), bytes.NewReader(encodePNG(t, 400, 400)), &Crop{X: 0.9, Y: 0, Size: 0.5}, path)
	if !errors.Is(err, ErrBadCrop) {
		t.Fatalf("got %v, want ErrBadCrop", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("file written for a rejected crop")
	}
}
