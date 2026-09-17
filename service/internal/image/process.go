package image

import (
	"bytes"
	"errors"
	"fmt"
	stdimage "image"
	"image/color"
	"image/jpeg"
	_ "image/png" // registers the PNG decoder
	"os"
	"path/filepath"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp" // registers the WebP decoder
)

// variant is one of the JPEG renditions written per upload.
type variant struct {
	suffix  string // appended to the image id before ".jpg"
	maxSide int    // longest side after downscaling; never upscales
	quality int    // JPEG quality
}

// variants in write order. The first ("original") is also the source the
// other two are scaled from, and the one whose dimensions are stored.
var variants = []variant{
	{suffix: "", maxSide: 2400, quality: 90},
	{suffix: "_detail", maxSide: 1600, quality: 85},
	{suffix: "_thumb", maxSide: 480, quality: 80},
}

// supportedFormats are the image.Decode format names accepted for upload.
var supportedFormats = map[string]bool{"jpeg": true, "png": true, "webp": true}

// written describes the files writeVariants produced.
type written struct {
	Width, Height int   // dimensions of the original variant
	Size          int64 // bytes across all three files
}

// decode turns an upload into an upright image. The header is inspected
// first (image.DecodeConfig) so an oversized image is rejected before its
// pixels are allocated; an unknown format is ErrUnsupported, anything the
// decoder cannot finish is ErrInvalid. JPEGs are rotated per their EXIF
// orientation so the variants need no metadata to display correctly.
func decode(data []byte) (stdimage.Image, error) {
	cfg, format, err := stdimage.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		if errors.Is(err, stdimage.ErrFormat) {
			return nil, ErrUnsupported
		}
		return nil, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	if !supportedFormats[format] {
		return nil, ErrUnsupported
	}
	if cfg.Width < minSide || cfg.Height < minSide || cfg.Width > maxSide || cfg.Height > maxSide {
		return nil, ErrInvalid
	}
	img, _, err := stdimage.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	if format == "jpeg" {
		// Downscale before rotating: the per-pixel rotation costs far less on
		// ≤ 2400 px than on a 12000 px photo, and the two operations commute.
		w, h := fit(img.Bounds().Dx(), img.Bounds().Dy(), variants[0].maxSide)
		img = applyOrientation(resize(img, w, h), jpegOrientation(data))
	}
	return img, nil
}

// fit scales w×h down so the longest side is at most maxSide, keeping the
// aspect ratio (rounded to the nearest pixel, at least 1). It never scales up.
func fit(w, h, maxSide int) (int, int) {
	if w <= maxSide && h <= maxSide {
		return w, h
	}
	if w >= h {
		return maxSide, max(1, int(float64(h)*float64(maxSide)/float64(w)+0.5))
	}
	return max(1, int(float64(w)*float64(maxSide)/float64(h)+0.5)), maxSide
}

// resize renders src at w×h with Catmull-Rom filtering over a white
// background, so transparent PNG/WebP areas come out white in the JPEG
// rather than black. Same-size requests are returned as is.
func resize(src stdimage.Image, w, h int) stdimage.Image {
	// Opaque same-size sources (a JPEG already within bounds) pass through;
	// an alpha-capable one is still redrawn so transparency is flattened.
	if b := src.Bounds(); b.Dx() == w && b.Dy() == h && !hasAlpha(src) {
		return src
	}
	dst := stdimage.NewRGBA(stdimage.Rect(0, 0, w, h))
	draw.Draw(dst, dst.Bounds(), stdimage.NewUniform(color.White), stdimage.Point{}, draw.Src)
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)
	return dst
}

// hasAlpha reports whether img is a format that can carry transparency.
func hasAlpha(img stdimage.Image) bool {
	switch img.(type) {
	case *stdimage.NRGBA, *stdimage.RGBA, *stdimage.NRGBA64, *stdimage.RGBA64, *stdimage.Paletted:
		return true
	}
	return false
}

// variantPath is where one rendition of an image lives on disk.
func variantPath(dir, recipeID, imageID, suffix string) string {
	return filepath.Join(dir, recipeID, imageID+suffix+".jpg")
}

// writeVariants renders every variant of src into recipeDir as
// <id><suffix>.jpg. Each file is written to a ".tmp" sibling and renamed
// into place, so a reader never sees a partial file. On any error the files
// written so far (temp or final) are removed and the error returned.
func writeVariants(recipeDir, id string, src stdimage.Image) (written, error) {
	if err := os.MkdirAll(recipeDir, 0o750); err != nil {
		return written{}, fmt.Errorf("create image dir: %w", err)
	}
	var out written
	var finals []string
	cleanup := func() {
		for _, p := range finals {
			_ = os.Remove(p)
		}
	}
	for i, v := range variants {
		w, h := fit(src.Bounds().Dx(), src.Bounds().Dy(), v.maxSide)
		img := resize(src, w, h)
		if i == 0 {
			out.Width, out.Height = w, h
			src = img // detail and thumb scale from the ≤ 2400 px original
		}
		final := filepath.Join(recipeDir, id+v.suffix+".jpg")
		tmp := final + ".tmp"
		n, err := writeJPEG(tmp, img, v.quality)
		if err != nil {
			_ = os.Remove(tmp)
			cleanup()
			return written{}, err
		}
		if err := os.Rename(tmp, final); err != nil {
			_ = os.Remove(tmp)
			cleanup()
			return written{}, fmt.Errorf("rename %s: %w", final, err)
		}
		finals = append(finals, final)
		out.Size += n
	}
	return out, nil
}

// writeJPEG encodes img to path and returns the byte count.
func writeJPEG(path string, img stdimage.Image, quality int) (int64, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o640) //nolint:gosec // G302: 0640 matches the 0750 image dir, group-readable by design
	if err != nil {
		return 0, fmt.Errorf("create %s: %w", path, err)
	}
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: quality}); err != nil {
		_ = f.Close()
		return 0, fmt.Errorf("encode %s: %w", path, err)
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return 0, fmt.Errorf("stat %s: %w", path, err)
	}
	if err := f.Close(); err != nil {
		return 0, fmt.Errorf("close %s: %w", path, err)
	}
	return info.Size(), nil
}
