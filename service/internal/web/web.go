// Package web embeds the built frontend. The frontend build writes into
// the dist directory; it is git-ignored except for .gitkeep.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// Dist returns the embedded frontend build rooted at dist/.
func Dist() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic("web: dist directory missing from embed: " + err.Error())
	}
	return sub
}
