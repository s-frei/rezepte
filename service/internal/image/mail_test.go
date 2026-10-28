package image_test

import (
	"bytes"
	stdimage "image"
	"image/jpeg"
	"testing"

	"github.com/s-frei/rezepte/service/internal/image"
)

func mailPhoto(t *testing.T, w, h int) stdimage.Rectangle {
	t.Helper()
	var in bytes.Buffer
	if err := jpeg.Encode(&in, stdimage.NewRGBA(stdimage.Rect(0, 0, w, h)), nil); err != nil {
		t.Fatal(err)
	}
	out, err := image.MailPhoto(&in)
	if err != nil {
		t.Fatal(err)
	}
	got, err := jpeg.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	return got.Bounds()
}

func TestMailPhotoNeverUpscales(t *testing.T) {
	if b := mailPhoto(t, 400, 300); b.Dx() != 400 || b.Dy() != 300 {
		t.Errorf("400x300 came out %v", b)
	}
}

func TestMailPhotoScalesDown(t *testing.T) {
	if b := mailPhoto(t, 1600, 900); b.Dx() != 800 || b.Dy() != 450 {
		t.Errorf("1600x900 came out %v", b)
	}
}
