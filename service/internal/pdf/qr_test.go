//go:build unix

package pdf

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"rsc.io/qr"
)

// eccLevel reads the error correction level back from the format bits next
// to the top-left finder: 15 bits along row 8 and up column 8, masked with
// 0x5412, the level in the top two (01 L, 00 M, 11 Q, 10 H).
func eccLevel(c *qr.Code) string {
	var bits int
	add := func(x, y int) {
		bits <<= 1
		if c.Black(x, y) {
			bits |= 1
		}
	}
	for _, x := range []int{0, 1, 2, 3, 4, 5, 7, 8} {
		add(x, 8)
	}
	for _, y := range []int{7, 5, 4, 3, 2, 1, 0} {
		add(8, y)
	}
	return [4]string{"M", "L", "H", "Q"}[(bits^0x5412)>>13]
}

func TestECCLevelReader(t *testing.T) {
	for want, level := range map[string]qr.Level{"L": qr.L, "M": qr.M, "Q": qr.Q, "H": qr.H} {
		c, err := qr.Encode("https://rezepte.example.com/recipes/x", level)
		if err != nil {
			t.Fatal(err)
		}
		if got := eccLevel(c); got != want {
			t.Errorf("level %s read as %s", want, got)
		}
	}
}

// The footer code is level H, a bare code (the view box the modules alone,
// the quiet zone left to the page), a dot per dark module outside the
// finders and the 9 × 9 center, three rounded eyes, and the mascot over a
// white hole:
// the SVG holds exactly the modules the encoder set, so it scans like it.
// A long public URL with a long slug (about 80 characters) needs a larger
// version; the hole stays 9 × 9 and level H covers it.
func TestQRStyle(t *testing.T) {
	for _, url := range []string{
		"https://rezepte.example.com/recipes/koenigsberger-klopse",
		"https://rezepte.familie-mustermann.example.org/recipes/schwaebische-kaesespaetzle-2",
	} {
		t.Run(fmt.Sprint(len(url)), func(t *testing.T) { checkQRStyle(t, url) })
	}
}

func checkQRStyle(t *testing.T, url string) {
	svg, err := qrSVG(url)
	if err != nil {
		t.Fatal(err)
	}
	c, err := qr.Encode(url, qr.H)
	if err != nil {
		t.Fatal(err)
	}
	if got := eccLevel(c); got != "H" {
		t.Errorf("level %s, want H", got)
	}
	s, n := string(svg), c.Size
	if box := fmt.Sprintf(`viewBox="0 0 %d %d"`, n, n); !strings.Contains(s, box) {
		t.Errorf("no %s", box)
	}
	want := map[string]bool{}
	hole := 0
	for y := range n {
		for x := range n {
			if qrInHole(n, x, y) {
				hole++
			}
			if c.Black(x, y) && !qrFinder(n, x, y) && !qrInHole(n, x, y) {
				want[fmt.Sprintf("%g %g", float64(x)+0.5, float64(y)+0.5)] = true
			}
		}
	}
	if hole != qrHole*qrHole {
		t.Errorf("hole of %d modules, want %d", hole, qrHole*qrHole)
	}
	dots := regexp.MustCompile(`<circle cx="([\d.]+)" cy="([\d.]+)" r="0.5"/>`).FindAllStringSubmatch(s, -1)
	if len(dots) != len(want) {
		t.Errorf("%d dots, want %d", len(dots), len(want))
	}
	for _, d := range dots {
		if !want[d[1]+" "+d[2]] {
			t.Errorf("dot at %s %s is not a dark module outside the finders and the hole", d[1], d[2])
		}
	}
	if got := strings.Count(s, `width="6" height="6" rx="2" fill="none"`); got != 3 {
		t.Errorf("%d eye rings, want 3", got)
	}
	if got := strings.Count(s, `width="3" height="3" rx="1"`); got != 3 {
		t.Errorf("%d eye pupils, want 3", got)
	}
	// Every inked shape is pure black, so it prints as one layer of K.
	for _, ink := range regexp.MustCompile(`(?:fill|stroke)="([^"]+)"`).FindAllStringSubmatch(s, -1) {
		if ink[1] != "#000000" && ink[1] != "#fff" && ink[1] != "none" {
			t.Errorf("ink %s, want #000000", ink[1])
		}
	}
	if !strings.Contains(s, `rx="1.6" fill="#fff"`) || !strings.Contains(s, `href="data:image/png;base64,`) {
		t.Error("no white hole with the mascot")
	}
}

// The mascot in the code is a copy of the brand's round favicon; a changed
// brand file must be copied.
func TestQRMascotCopy(t *testing.T) {
	brand, err := os.ReadFile("../../../assets/brand/lockups/rezepte-favicon-192.png")
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile("templates/mascot.png")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b, brand) {
		t.Error("templates/mascot.png differs from assets/brand/lockups/rezepte-favicon-192.png")
	}
}
