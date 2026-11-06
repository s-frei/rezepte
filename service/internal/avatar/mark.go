package avatar

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	stdimage "image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"
	"strings"
	"sync"

	"golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	"github.com/s-frei/rezepte/service/internal/mail"
	"github.com/s-frei/rezepte/service/internal/user"
)

// markSide is the mark's size in pixels: a notification icon is shown at
// 64 to 96 device pixels, and 192 is the app icon's own size beside it.
const markSide = 192

// markFont parses the embedded Go Bold face once.
var markFont = sync.OnceValues(func() (*opentype.Font, error) {
	f, err := opentype.Parse(gobold.TTF)
	if err != nil {
		return nil, fmt.Errorf("parse Go Bold: %w", err)
	}
	return f, nil
})

// Mark renders u's circle as a PNG with a transparent corner: their picture
// cropped round, or their initial on the light pair of their color, the
// same circle a mail shows (mail.Circle). etag changes whenever what the
// mark shows does. A picture that cannot be read falls back to the initial.
func (s *Service) Mark(u user.User) (pngBytes []byte, etag string, err error) {
	bg, fg, initial := mail.Circle(u.Color, u.DisplayName)
	what := "i:" + string(u.Color) + ":" + initial
	img := stdimage.NewRGBA(stdimage.Rect(0, 0, markSide, markSide))
	if face := s.picture(u); face != nil {
		what = "p:" + *u.AvatarID
		draw.CatmullRom.Scale(img, img.Bounds(), face, face.Bounds(), draw.Src, nil)
	} else if err := drawInitial(img, hexColor(bg), hexColor(fg), initial); err != nil {
		return nil, "", err
	}
	out := stdimage.NewRGBA(img.Bounds())
	draw.DrawMask(out, out.Bounds(), img, stdimage.Point{}, roundMask(markSide), stdimage.Point{}, draw.Src)
	var buf bytes.Buffer
	if err := png.Encode(&buf, out); err != nil {
		return nil, "", fmt.Errorf("encode mark: %w", err)
	}
	sum := sha256.Sum256([]byte(what))
	return buf.Bytes(), `"` + hex.EncodeToString(sum[:8]) + `"`, nil
}

// picture decodes u's picture, or answers nil without one or when it
// cannot be read.
func (s *Service) picture(u user.User) stdimage.Image {
	if u.AvatarID == nil {
		return nil
	}
	f, err := s.Open(u.ID, *u.AvatarID)
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }()
	face, err := jpeg.Decode(f)
	if err != nil {
		return nil
	}
	return face
}

// drawInitial fills img with bg and centers initial on it in fg.
func drawInitial(img *stdimage.RGBA, bg, fg color.Color, initial string) error {
	draw.Draw(img, img.Bounds(), stdimage.NewUniform(bg), stdimage.Point{}, draw.Src)
	f, err := markFont()
	if err != nil {
		return err
	}
	face, err := opentype.NewFace(f, &opentype.FaceOptions{Size: markSide * 0.46, DPI: 72, Hinting: font.HintingNone})
	if err != nil {
		return fmt.Errorf("create font face: %w", err)
	}
	defer func() { _ = face.Close() }()
	d := &font.Drawer{Dst: img, Src: stdimage.NewUniform(fg), Face: face}
	// The ink box, not the advance, so I and W both sit in the middle.
	b, _ := d.BoundString(initial)
	d.Dot = fixed.Point26_6{
		X: fixed.I(markSide/2) - (b.Min.X+b.Max.X)/2,
		Y: fixed.I(markSide/2) - (b.Min.Y+b.Max.Y)/2,
	}
	d.DrawString(initial)
	return nil
}

// roundMask is a circle filling a side×side square, its edge antialiased
// over one pixel.
func roundMask(side int) *stdimage.Alpha {
	m := stdimage.NewAlpha(stdimage.Rect(0, 0, side, side))
	r := float64(side) / 2
	for y := range side {
		for x := range side {
			d := math.Hypot(float64(x)+0.5-r, float64(y)+0.5-r)
			m.SetAlpha(x, y, color.Alpha{A: uint8(255 * math.Max(0, math.Min(1, r-d+0.5)))})
		}
	}
	return m
}

// hexColor reads mail.Circle's "#rrggbb".
func hexColor(s string) color.RGBA {
	b, err := hex.DecodeString(strings.TrimPrefix(s, "#"))
	if err != nil || len(b) != 3 {
		return color.RGBA{A: 0xff}
	}
	return color.RGBA{R: b[0], G: b[1], B: b[2], A: 0xff}
}
