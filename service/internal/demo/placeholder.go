package demo

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

const (
	placeholderWidth   = 1200
	placeholderHeight  = 900
	plateRadius        = 320
	letterSize         = 400 // px at 72 DPI
	placeholderQuality = 85
)

// tints are the warm surface colors of the design system (see
// docs/memory/content/architecture/design-system), rotated per recipe.
var tints = []color.RGBA{
	{R: 0xf0, G: 0xdf, B: 0xc4, A: 0xff},
	{R: 0xe6, G: 0xe3, B: 0xcf, A: 0xff},
	{R: 0xef, G: 0xd9, B: 0xcf, A: 0xff},
}

// ink is the primary brown the initial is drawn in.
var ink = color.RGBA{R: 0x7c, G: 0x3d, B: 0x0a, A: 0xff}

// boldFont parses the embedded Go Bold face once; parsing is the only
// non-trivial cost and the result is immutable.
var boldFont = sync.OnceValues(func() (*opentype.Font, error) {
	f, err := opentype.Parse(gobold.TTF)
	if err != nil {
		return nil, fmt.Errorf("parse Go Bold: %w", err)
	}
	return f, nil
})

// Placeholder renders the stand-in photo for the sample recipe at index
// (which must not be negative): a tinted background, a lighter plate and the
// title's initial. The output depends only on index and title, so reseeding
// yields identical files and screenshots stay stable. Real photos can replace
// these through the normal upload at any time.
func Placeholder(index int, title string) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, placeholderWidth, placeholderHeight))
	tint := tintFor(index)
	draw.Draw(img, img.Bounds(), image.NewUniform(tint), image.Point{}, draw.Src)
	fillCircle(img, placeholderWidth/2, placeholderHeight/2, plateRadius, lighten(tint))

	f, err := boldFont()
	if err != nil {
		return nil, err
	}
	face, err := opentype.NewFace(f, &opentype.FaceOptions{Size: letterSize, DPI: 72, Hinting: font.HintingNone})
	if err != nil {
		return nil, fmt.Errorf("create font face: %w", err)
	}
	defer face.Close()

	letter := initial(title)
	d := &font.Drawer{Dst: img, Src: image.NewUniform(ink), Face: face}
	// Center the glyph's ink box rather than its advance box so narrow and
	// wide letters both sit in the middle of the plate.
	bounds, _ := d.BoundString(letter)
	d.Dot = fixed.Point26_6{
		X: fixed.I(placeholderWidth/2) - (bounds.Min.X+bounds.Max.X)/2,
		Y: fixed.I(placeholderHeight/2) - (bounds.Min.Y+bounds.Max.Y)/2,
	}
	d.DrawString(letter)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: placeholderQuality}); err != nil {
		return nil, fmt.Errorf("encode placeholder: %w", err)
	}
	return buf.Bytes(), nil
}

// initial returns the upper-cased first rune of title, or "?" when empty.
func initial(title string) string {
	r, size := utf8.DecodeRuneInString(strings.TrimSpace(title))
	if size == 0 || r == utf8.RuneError {
		return "?"
	}
	return string(unicode.ToUpper(r))
}

// tintFor picks the background for the recipe at index. The three tints
// rotate, and every full rotation steps one shade darker, so that samples
// sharing a tint and an initial (Königsberger Klopse, Käsespätzle and
// Kartoffelsalat all do) still get distinguishable tiles.
func tintFor(index int) color.RGBA {
	c := tints[index%len(tints)]
	for range index / len(tints) {
		c = darken(c)
	}
	return c
}

// darken moves c one shade towards black, gently enough that the whole
// sample set still reads as one warm palette.
func darken(c color.RGBA) color.RGBA {
	d := func(v uint8) uint8 { return v - v/24 }
	return color.RGBA{R: d(c.R), G: d(c.G), B: d(c.B), A: 0xff}
}

// lighten moves c halfway towards white.
func lighten(c color.RGBA) color.RGBA {
	l := func(v uint8) uint8 { return v + (255-v)/2 }
	return color.RGBA{R: l(c.R), G: l(c.G), B: l(c.B), A: 0xff}
}

// fillCircle paints a filled disc; a plain scanline test is enough for a
// single shape and keeps the package free of a vector rasterizer.
func fillCircle(img *image.RGBA, cx, cy, r int, c color.RGBA) {
	for y := cy - r; y <= cy+r; y++ {
		for x := cx - r; x <= cx+r; x++ {
			dx, dy := x-cx, y-cy
			if dx*dx+dy*dy <= r*r {
				img.SetRGBA(x, y, c)
			}
		}
	}
}
