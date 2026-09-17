package image

import (
	"encoding/binary"
	stdimage "image"
	"image/color"
)

const (
	markerSOI      = 0xD8
	markerAPP1     = 0xE1
	markerSOS      = 0xDA
	tagOrientation = 0x0112
	typeShort      = 3
)

// jpegOrientation returns the EXIF orientation (1-8) from a JPEG's APP1
// segment, or 1 when data is not a JPEG, has no EXIF block, or the block is
// malformed. It walks segment headers only, up to the first APP1 or the
// start of scan, and never touches image data. The layout it reads: APP1
// payload "Exif\0\0" + TIFF header (byte order "II"/"MM", magic 0x002A,
// IFD0 offset) + IFD0 (entry count, 12-byte entries: tag, type, count,
// value). Orientation is tag 0x0112, type SHORT, count 1, value in the first
// two bytes of the value slot.
func jpegOrientation(data []byte) int {
	if len(data) < 4 || data[0] != 0xFF || data[1] != markerSOI {
		return 1
	}
	pos := 2
	for pos+4 <= len(data) {
		if data[pos] != 0xFF {
			return 1
		}
		marker := data[pos+1]
		if marker == 0xFF { // fill byte before a marker
			pos++
			continue
		}
		if marker == markerSOS {
			return 1
		}
		segLen := int(binary.BigEndian.Uint16(data[pos+2:]))
		if segLen < 2 || pos+2+segLen > len(data) {
			return 1
		}
		if marker == markerAPP1 {
			return orientationFromAPP1(data[pos+4 : pos+2+segLen])
		}
		pos += 2 + segLen
	}
	return 1
}

// orientationFromAPP1 parses the payload of an APP1 segment (after the
// marker and length).
func orientationFromAPP1(seg []byte) int {
	if len(seg) < 6 || string(seg[:6]) != "Exif\x00\x00" {
		return 1
	}
	tiff := seg[6:]
	if len(tiff) < 8 {
		return 1
	}
	var order binary.ByteOrder
	switch string(tiff[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return 1
	}
	if order.Uint16(tiff[2:]) != 0x002A {
		return 1
	}
	ifd := int(order.Uint32(tiff[4:]))
	if ifd < 8 || ifd+2 > len(tiff) {
		return 1
	}
	n := int(order.Uint16(tiff[ifd:]))
	for i := 0; i < n; i++ {
		e := ifd + 2 + i*12
		if e+12 > len(tiff) {
			return 1
		}
		if order.Uint16(tiff[e:]) != tagOrientation {
			continue
		}
		if order.Uint16(tiff[e+2:]) != typeShort || order.Uint32(tiff[e+4:]) != 1 {
			return 1
		}
		v := int(order.Uint16(tiff[e+8:]))
		if v < 1 || v > 8 {
			return 1
		}
		return v
	}
	return 1
}

// applyOrientation returns img redrawn so it displays upright without EXIF
// metadata (EXIF 2.32, Orientation): 1 as is, 2 mirror horizontal, 3 rotate
// 180°, 4 mirror vertical, 5 transpose, 6 rotate 90° clockwise, 7
// transverse, 8 rotate 90° counter-clockwise. Orientations 5-8 swap width
// and height. Anything outside 2-8 returns img unchanged.
func applyOrientation(img stdimage.Image, orientation int) stdimage.Image {
	if orientation < 2 || orientation > 8 {
		return img
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	var dst *stdimage.NRGBA
	if orientation <= 4 {
		dst = stdimage.NewNRGBA(stdimage.Rect(0, 0, w, h))
	} else {
		dst = stdimage.NewNRGBA(stdimage.Rect(0, 0, h, w))
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var dx, dy int
			switch orientation {
			case 2:
				dx, dy = w-1-x, y
			case 3:
				dx, dy = w-1-x, h-1-y
			case 4:
				dx, dy = x, h-1-y
			case 5:
				dx, dy = y, x
			case 6:
				dx, dy = h-1-y, x
			case 7:
				dx, dy = h-1-y, w-1-x
			case 8:
				dx, dy = y, w-1-x
			}
			dst.Set(dx, dy, color.NRGBAModel.Convert(img.At(b.Min.X+x, b.Min.Y+y)))
		}
	}
	return dst
}
