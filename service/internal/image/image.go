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
)

const (
	maxImages = 20
	minSide   = 64
	maxSide   = 12000
)
