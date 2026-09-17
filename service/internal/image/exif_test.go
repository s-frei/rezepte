package image

import (
	"bytes"
	"encoding/binary"
	stdimage "image"
	"image/color"
	"image/jpeg"
	"testing"
)

// encodeJPEG renders a w×h image whose top-left pixel is red and everything
// else is blue, as a baseline JPEG without any APP segments.
func encodeJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := stdimage.NewRGBA(stdimage.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{0, 0, 255, 255})
		}
	}
	img.Set(0, 0, color.RGBA{255, 0, 0, 255})
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 100}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// buildAPP1 crafts an EXIF APP1 segment holding one IFD0 entry: the
// Orientation tag (0x0112, SHORT, count 1).
func buildAPP1(orientation uint16, little bool) []byte {
	var order binary.AppendByteOrder = binary.BigEndian
	tiff := []byte{'M', 'M'}
	if little {
		order = binary.LittleEndian
		tiff = []byte{'I', 'I'}
	}
	tiff = order.AppendUint16(tiff, 0x002A)
	tiff = order.AppendUint32(tiff, 8) // IFD0 starts right after the 8-byte header
	tiff = order.AppendUint16(tiff, 1) // one entry
	tiff = order.AppendUint16(tiff, 0x0112)
	tiff = order.AppendUint16(tiff, 3) // SHORT
	tiff = order.AppendUint32(tiff, 1) // count
	tiff = order.AppendUint16(tiff, orientation)
	tiff = append(tiff, 0, 0)          // SHORT value padded to the 4-byte slot
	tiff = order.AppendUint32(tiff, 0) // no next IFD
	payload := append([]byte("Exif\x00\x00"), tiff...)
	seg := []byte{0xFF, 0xE1}
	seg = binary.BigEndian.AppendUint16(seg, uint16(len(payload)+2))
	return append(seg, payload...)
}

// withAPP1 splices an APP1 segment right after the SOI marker.
func withAPP1(jpg, app1 []byte) []byte {
	out := append([]byte{}, jpg[:2]...)
	out = append(out, app1...)
	return append(out, jpg[2:]...)
}

func TestJpegOrientationReadsBothByteOrders(t *testing.T) {
	base := encodeJPEG(t, 4, 2)
	for _, little := range []bool{false, true} {
		for want := 1; want <= 8; want++ {
			data := withAPP1(base, buildAPP1(uint16(want), little))
			if got := jpegOrientation(data); got != want {
				t.Fatalf("little=%v want %d got %d", little, want, got)
			}
			// The decoder must still accept the spliced file.
			if _, _, err := stdimage.Decode(bytes.NewReader(data)); err != nil {
				t.Fatalf("decode with APP1: %v", err)
			}
		}
	}
}

func TestJpegOrientationDefaultsToOne(t *testing.T) {
	cases := map[string][]byte{
		"no exif":        encodeJPEG(t, 4, 2),
		"not a jpeg":     []byte("\x89PNG\r\n\x1a\n"),
		"empty":          nil,
		"truncated app1": withAPP1(encodeJPEG(t, 4, 2), buildAPP1(6, false)[:12]),
		"bad value":      withAPP1(encodeJPEG(t, 4, 2), buildAPP1(9, false)),
		"zero value":     withAPP1(encodeJPEG(t, 4, 2), buildAPP1(0, true)),
	}
	for name, data := range cases {
		if got := jpegOrientation(data); got != 1 {
			t.Errorf("%s: got %d, want 1", name, got)
		}
	}
}

func TestApplyOrientation(t *testing.T) {
	// 4×2 source, red pixel at (0,0). Expected size and red position per
	// EXIF orientation.
	src := stdimage.NewRGBA(stdimage.Rect(0, 0, 4, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 4; x++ {
			src.Set(x, y, color.RGBA{0, 0, 255, 255})
		}
	}
	src.Set(0, 0, color.RGBA{255, 0, 0, 255})
	cases := []struct{ o, w, h, rx, ry int }{
		{1, 4, 2, 0, 0}, {2, 4, 2, 3, 0}, {3, 4, 2, 3, 1}, {4, 4, 2, 0, 1},
		{5, 2, 4, 0, 0}, {6, 2, 4, 1, 0}, {7, 2, 4, 1, 3}, {8, 2, 4, 0, 3},
	}
	for _, c := range cases {
		out := applyOrientation(src, c.o)
		b := out.Bounds()
		if b.Dx() != c.w || b.Dy() != c.h {
			t.Fatalf("orientation %d: size %dx%d, want %dx%d", c.o, b.Dx(), b.Dy(), c.w, c.h)
		}
		r, g, _, _ := out.At(c.rx, c.ry).RGBA()
		if r>>8 != 255 || g != 0 {
			t.Fatalf("orientation %d: red pixel not at (%d,%d)", c.o, c.rx, c.ry)
		}
	}
}
