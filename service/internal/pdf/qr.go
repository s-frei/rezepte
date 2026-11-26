//go:build unix

package pdf

import (
	"encoding/base64"
	"fmt"
	"strings"

	"rsc.io/qr"
)

// qrMascot is the Rezepte mascot in the code's center (and on the book's
// title page), a copy of assets/brand/lockups/rezepte-favicon-192.png, read
// from the embedded template tree.
var qrMascot = func() []byte {
	b, err := templateFiles.ReadFile("templates/mascot.png")
	if err != nil {
		panic(err) // an embedded constant path, so a programming error
	}
	return b
}()

// The code's geometry: a bare code, the view box the modules alone (the
// template's gaps and the page margin are the quiet zone), round dots a
// module wide (tangent
// dots decode far more reliably than gapped ones at 400 and 600 dpi), rounded
// finder eyes, and a blank hole of qrHole × qrHole modules in the center for
// the mascot, which level H's error correction covers. The ink is pure
// black, not the page's warm ink: office drivers print a non-neutral color
// from CMY and K, which misregisters by a fraction of a module on a color
// laser and bleeds on an inkjet; pure black prints as one sharp layer.
const (
	qrHole = 9
	qrInk  = "#000000"
)

// qrFinder reports whether module x, y belongs to one of the three finder
// patterns, which are drawn as eyes instead of dots.
func qrFinder(n, x, y int) bool {
	return (x < 7 && y < 7) || (x >= n-7 && y < 7) || (x < 7 && y >= n-7)
}

// qrInHole reports whether module x, y lies in the blank center.
func qrInHole(n, x, y int) bool {
	c := float64(n) / 2
	in := func(v int) bool { d := float64(v) + 0.5 - c; return d > -qrHole/2.0 && d < qrHole/2.0 }
	return in(x) && in(y)
}

// qrSVG encodes url at level H as an SVG of round dots with the mascot in a
// white rounded hole in the center.
func qrSVG(url string) ([]byte, error) {
	c, err := qr.Encode(url, qr.H)
	if err != nil {
		return nil, fmt.Errorf("encode qr: %w", err)
	}
	n := c.Size
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d">`, n, n)
	fmt.Fprintf(&b, `<g fill="%s">`, qrInk)
	for y := range n {
		for x := range n {
			if c.Black(x, y) && !qrFinder(n, x, y) && !qrInHole(n, x, y) {
				fmt.Fprintf(&b, `<circle cx="%g" cy="%g" r="0.5"/>`, float64(x)+0.5, float64(y)+0.5)
			}
		}
	}
	b.WriteString(`</g>`)
	for _, at := range [][2]int{{0, 0}, {n - 7, 0}, {0, n - 7}} {
		x, y := at[0], at[1]
		fmt.Fprintf(&b, `<rect x="%g" y="%g" width="6" height="6" rx="2" fill="none" stroke="%s" stroke-width="1"/>`, float64(x)+0.5, float64(y)+0.5, qrInk)
		fmt.Fprintf(&b, `<rect x="%d" y="%d" width="3" height="3" rx="1" fill="%s"/>`, x+2, y+2, qrInk)
	}
	h := float64(n)/2 - qrHole/2.0
	fmt.Fprintf(&b, `<rect x="%g" y="%g" width="%g" height="%g" rx="1.6" fill="#fff"/>`, h+0.3, h+0.3, qrHole-0.6, qrHole-0.6)
	fmt.Fprintf(&b, `<image x="%g" y="%g" width="%g" height="%g" href="data:image/png;base64,%s"/>`,
		h+0.7, h+0.7, qrHole-1.4, qrHole-1.4, base64.StdEncoding.EncodeToString(qrMascot))
	b.WriteString("</svg>\n")
	return []byte(b.String()), nil
}
