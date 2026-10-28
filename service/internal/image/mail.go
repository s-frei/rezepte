package image

import (
	"bytes"
	"fmt"
	"image/jpeg"
	"io"
)

const (
	// mailPhotoWidth covers the 504 px sheet column at 1.5x; the 960 px thumb it is
	// made from is never scaled up.
	mailPhotoWidth   = 800
	mailPhotoQuality = 72
)

// MailPhoto renders a JPEG, a recipe's _thumb.jpg, for a mail: at most
// 800 px wide, never larger than it is, at quality 72.
func MailPhoto(r io.Reader) ([]byte, error) {
	src, err := jpeg.Decode(r)
	if err != nil {
		return nil, fmt.Errorf("decode mail photo: %w", err)
	}
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	if w > mailPhotoWidth {
		w, h = mailPhotoWidth, max(1, h*mailPhotoWidth/w)
	}
	var out bytes.Buffer
	if err := jpeg.Encode(&out, resize(src, w, h), &jpeg.Options{Quality: mailPhotoQuality}); err != nil {
		return nil, fmt.Errorf("encode mail photo: %w", err)
	}
	return out.Bytes(), nil
}
