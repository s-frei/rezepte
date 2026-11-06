package avatar

import (
	"bytes"
	stdimage "image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/s-frei/rezepte/service/internal/user"
)

func decodeMark(t *testing.T, b []byte) stdimage.Image {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if got := img.Bounds().Size(); got != stdimage.Pt(markSide, markSide) {
		t.Fatalf("size %v", got)
	}
	if _, _, _, a := img.At(0, 0).RGBA(); a != 0 {
		t.Errorf("corner alpha %d, want transparent", a)
	}
	return img
}

func near(c color.Color, want color.RGBA) bool {
	r, g, b, _ := c.RGBA()
	d := func(x uint32, y uint8) bool { v := int(x>>8) - int(y); return v > -12 && v < 12 }
	return d(r, want.R) && d(g, want.G) && d(b, want.B)
}

func TestMarkInitialOnColor(t *testing.T) {
	s := &Service{dir: t.TempDir()}
	b, etag, err := s.Mark(user.User{ID: "u1", DisplayName: "mila", Color: "sage"})
	if err != nil {
		t.Fatal(err)
	}
	img := decodeMark(t, b)
	// Inside the circle, left of the letter: sage's light background.
	if c := img.At(16, markSide/2); !near(c, hexColor("#dbe7d9")) {
		t.Errorf("background %v, want sage", c)
	}
	_, other, _ := s.Mark(user.User{ID: "u1", DisplayName: "mila", Color: "rose"})
	_, renamed, _ := s.Mark(user.User{ID: "u1", DisplayName: "Jonas", Color: "sage"})
	if etag == other || etag == renamed {
		t.Errorf("etag %s did not change with the color (%s) or the initial (%s)", etag, other, renamed)
	}
}

func TestMarkShowsThePicture(t *testing.T) {
	s := &Service{dir: t.TempDir()}
	id := "0190a3f0-0000-7000-8000-000000000001"
	uid := "0190a3f0-0000-7000-8000-000000000002"
	red := stdimage.NewRGBA(stdimage.Rect(0, 0, 320, 320))
	for i := range red.Pix {
		red.Pix[i] = []uint8{0xe0, 0x10, 0x10, 0xff}[i%4]
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, red, nil); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(s.dir, uid), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.path(uid, id), buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	u := user.User{ID: uid, DisplayName: "mila", Color: "sage", AvatarID: &id}
	b, etag, err := s.Mark(u)
	if err != nil {
		t.Fatal(err)
	}
	if c := decodeMark(t, b).At(markSide/2, markSide/2); !near(c, color.RGBA{R: 0xe0, G: 0x10, B: 0x10}) {
		t.Errorf("center %v, want the picture's red", c)
	}
	// A picture that is gone falls back to the initial, under its own etag.
	gone := "0190a3f0-0000-7000-8000-000000000003"
	u.AvatarID = &gone
	b, fallback, err := s.Mark(u)
	if err != nil {
		t.Fatal(err)
	}
	if c := decodeMark(t, b).At(16, markSide/2); !near(c, hexColor("#dbe7d9")) || fallback == etag {
		t.Errorf("fallback %v under %s", c, fallback)
	}
}
