// Package image stores recipe photos: it decodes uploads (JPEG, PNG, WebP),
// applies the JPEG EXIF orientation, resizes into three JPEG variants on
// disk and keeps the images table and the recipe cover in step with them.
package image

import "errors"

// Sentinel errors mapped to HTTP statuses by the handler.
var (
	ErrNotFound    = errors.New("recipe or image not found")
	ErrTooMany     = errors.New("recipe has 20 images")
	ErrUnsupported = errors.New("unsupported image format")
	ErrBadOrder    = errors.New("imageIds must list every image exactly once")
	ErrInvalid     = errors.New("image must be 64-12000 px per side")
	ErrTooLarge    = errors.New("image has more than 60 megapixels")
)

const (
	maxImages = 20
	minSide   = 64
	maxSide   = 12000
	// maxPixels caps the decoded pixel count. The per-side limits alone
	// still allow 12000x12000 = 144 MP, which costs ~576 MiB as RGBA
	// before a single variant is written; 60 MP (~240 MiB) leaves room for
	// any camera or phone photo while keeping a decode bomb bounded.
	maxPixels = 60_000_000
	// concurrentDecodes is how many uploads may decode and resize at the
	// same time. Each holds its pixels in memory, so the peak is bounded
	// at roughly concurrentDecodes x maxPixels x 4 bytes no matter how
	// many clients upload at once; further uploads wait for a slot.
	concurrentDecodes = 2
)
