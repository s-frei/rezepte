package image

import (
	"context"
	"fmt"
	stdimage "image"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/image/draw"
)

const (
	// avatarSide is the one size an account picture is rendered at: the
	// person card's 64 CSS px on a 3x screen is 192, and 320 leaves room
	// without costing the 20 px circles on the overview much.
	avatarSide    = 320
	avatarQuality = 85
)

// Crop is the square a person picked in the browser, as fractions of the
// upright image: X and Y place its top-left corner against the width and
// the height, Size is its side against the shorter side. Fractions rather
// than pixels, because the service downscales before it crops.
type Crop struct{ X, Y, Size float64 }

// ParseCrop reads the "x,y,size" query parameter. Empty means no crop was
// asked for, which RenderAvatar reads as the centered square.
func ParseCrop(s string) (*Crop, error) {
	if s == "" {
		return nil, nil
	}
	parts := strings.Split(s, ",")
	if len(parts) != 3 {
		return nil, ErrBadCrop
	}
	var v [3]float64
	for i, p := range parts {
		f, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil || math.IsNaN(f) || f < 0 || f > 1 {
			return nil, ErrBadCrop
		}
		v[i] = f
	}
	if v[2] == 0 {
		return nil, ErrBadCrop
	}
	return &Crop{X: v[0], Y: v[1], Size: v[2]}, nil
}

// cropRect turns c into pixels of a w×h image. A square that reaches past an
// edge by at most half a pixel - the browser rounding - is pulled back
// inside; one that reaches further, or is smaller than minSide, is
// ErrBadCrop. A nil c is the centered square over the shorter side.
func cropRect(w, h int, c *Crop) (stdimage.Rectangle, error) {
	short := min(w, h)
	if c == nil {
		x, y := (w-short)/2, (h-short)/2
		return stdimage.Rect(x, y, x+short, y+short), nil
	}
	side := c.Size * float64(short)
	x, y := c.X*float64(w), c.Y*float64(h)
	if x+side > float64(w)+0.5 || y+side > float64(h)+0.5 {
		return stdimage.Rectangle{}, ErrBadCrop
	}
	n := min(int(math.Round(side)), short)
	if n < minSide {
		return stdimage.Rectangle{}, ErrBadCrop
	}
	x0 := min(int(math.Round(x)), w-n)
	y0 := min(int(math.Round(y)), h-n)
	return stdimage.Rect(x0, y0, x0+n, y0+n), nil
}

// RenderAvatar decodes the upload in r the way a recipe photo is decoded -
// the same formats, limits, EXIF orientation and decode slots - crops it to
// c and writes one square JPEG of at most avatarSide px to path, through a
// ".tmp" sibling so a reader never sees a partial file. The directory is
// created 0700 and the file is 0600, like the recipe photos.
func (s *Service) RenderAvatar(ctx context.Context, r io.Reader, c *Crop, path string) error {
	data, err := io.ReadAll(io.LimitReader(r, MaxUploadBytes+1))
	if err != nil {
		return fmt.Errorf("read upload: %w", err)
	}
	if len(data) > MaxUploadBytes {
		return fmt.Errorf("%w: larger than %d bytes", ErrInvalid, MaxUploadBytes)
	}
	select {
	case s.decodeSlots <- struct{}{}:
	case <-ctx.Done():
		return fmt.Errorf("wait for decode slot: %w", ctx.Err())
	}
	defer func() { <-s.decodeSlots }()

	img, err := decode(data)
	if err != nil {
		return err
	}
	b := img.Bounds()
	rect, err := cropRect(b.Dx(), b.Dy(), c)
	if err != nil {
		return err
	}
	n := min(rect.Dx(), avatarSide)
	out := resize(subImage(img, rect.Add(b.Min)), n, n)

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create avatar dir: %w", err)
	}
	tmp := path + ".tmp"
	if _, err := writeJPEG(tmp, out, avatarQuality); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename %s: %w", path, err)
	}
	return nil
}

// subImage returns the part of img inside r. Every decoder and every image
// resize produces implements SubImage; the fallback copies, so an image
// type added later still crops correctly.
func subImage(img stdimage.Image, r stdimage.Rectangle) stdimage.Image {
	if s, ok := img.(interface {
		SubImage(stdimage.Rectangle) stdimage.Image
	}); ok {
		return s.SubImage(r)
	}
	dst := stdimage.NewRGBA(stdimage.Rect(0, 0, r.Dx(), r.Dy()))
	draw.Draw(dst, dst.Bounds(), img, r.Min, draw.Src)
	return dst
}
