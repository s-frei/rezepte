//go:build !typst || !((linux || darwin) && (amd64 || arm64))

package typstbin

// binary is empty: a build without the typst tag, or for a platform Typst
// publishes no binary for, falls back to typst on PATH.
var binary []byte
