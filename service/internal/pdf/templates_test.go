//go:build unix

package pdf

import (
	"bytes"
	"compress/zlib"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/s-frei/rezepte/service/internal/pdf/typstbin"
)

// loadFixture reads testdata/<name>.json.
func loadFixture(t *testing.T, name string) Data {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var d Data
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatal(err)
	}
	return d
}

// queryTemplate prepares d with set in a temp dir and returns the values of
// every metadata element labeled label.
func queryTemplate(t *testing.T, d Data, set TemplateSet, label string) []json.RawMessage {
	t.Helper()
	return evalTemplate(t, d, set, "query(<"+label+">).map(m => m.value)")
}

// evalTemplate evaluates expr, which yields an array, in the document's context.
func evalTemplate(t *testing.T, d Data, set TemplateSet, expr string) []json.RawMessage {
	t.Helper()
	bin, err := typstbin.Path(t.Context())
	if errors.Is(err, typstbin.ErrUnavailable) {
		t.Skip("typst unavailable")
	}
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	entry, err := (&Renderer{}).prepare(dir, d, fixturePhotos(t), set)
	if err != nil {
		t.Fatal(err)
	}
	fonts, err := filepath.Abs("fonts")
	if err != nil {
		t.Fatal(err)
	}
	// `typst eval` in the document's context; `typst query` is deprecated.
	cmd := exec.CommandContext(t.Context(), bin, "eval", "--root", dir, "--font-path", fonts,
		"--ignore-system-fonts", "--in", filepath.Join(dir, entry), expr)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("typst eval %s: %v: %s", expr, err, stderr.String())
	}
	// A layout that does not converge is only a warning; it must fail here.
	if strings.Contains(stderr.String(), "warning") {
		t.Fatalf("typst eval %s: %s", expr, stderr.String())
	}
	var vals []json.RawMessage
	if err := json.Unmarshal(out, &vals); err != nil {
		t.Fatalf("decode %s: %v", out, err)
	}
	return vals
}

func strs(t *testing.T, vals []json.RawMessage) []string {
	t.Helper()
	out := make([]string, len(vals))
	for i, v := range vals {
		if err := json.Unmarshal(v, &out[i]); err != nil {
			t.Fatalf("not a string: %s", v)
		}
	}
	return out
}

func pageCount(t *testing.T, d Data, set TemplateSet) int {
	t.Helper()
	vals := queryTemplate(t, d, set, "page-count")
	var n int
	if len(vals) != 1 || json.Unmarshal(vals[0], &n) != nil {
		t.Fatalf("page-count = %s", vals)
	}
	return n
}

func TestKaesespaetzleA4(t *testing.T) {
	d := fixture(t)
	set := builtin()
	if got := strs(t, queryTemplate(t, d, set, "recipe-title")); len(got) != 1 || got[0] != "Käsespätzle" {
		t.Errorf("recipe-title = %q", got)
	}
	if got := strs(t, queryTemplate(t, d, set, "quantity")); !contains(got, "200 g Mehl") {
		t.Errorf("quantity = %q", got)
	}
	if got := strs(t, queryTemplate(t, d, set, "source")); len(got) != 1 || got[0] != "example.com" {
		t.Errorf("source = %q", got)
	}
	if n := pageCount(t, d, set); n != 1 {
		t.Errorf("pages = %d", n)
	}
	want := 0
	for _, s := range d.Recipes[0].Steps {
		for _, seg := range s.Segments {
			if seg.Kind == "ingredient" {
				want++
			}
		}
	}
	if got := queryTemplate(t, d, set, "ingredient-ref"); len(got) != want {
		t.Errorf("ingredient-ref = %d, want %d", len(got), want)
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func TestSourceWithoutFooter(t *testing.T) {
	d := fixture(t)
	d.Options.Footer = false
	if got := strs(t, queryTemplate(t, d, builtin(), "source")); len(got) != 1 || got[0] != "example.com" {
		t.Errorf("source = %q", got)
	}
}

func TestSparseRecipeRenders(t *testing.T) {
	for _, paper := range []Paper{PaperA4, PaperA5} {
		for _, photo := range []bool{false, true} {
			for _, footer := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s photo=%v footer=%v", paper, photo, footer), func(t *testing.T) {
					d := loadFixture(t, "sparse")
					d.Options.Paper, d.Options.Photo, d.Options.Footer = paper, photo, footer
					if n := pageCount(t, d, builtin()); n != 1 {
						t.Errorf("pages = %d", n)
					}
					if got := queryTemplate(t, d, builtin(), "source"); len(got) != 0 {
						t.Errorf("source printed for a recipe without one: %s", got)
					}
				})
			}
		}
	}
}

func TestMarkupIsLiteral(t *testing.T) {
	d := loadFixture(t, "markup")
	title := `#set text(red) $x$ *a* _b_ <c> @d \ ]`
	d.Recipes[0].Title = title
	if got := strs(t, queryTemplate(t, d, builtin(), "recipe-title")); len(got) != 1 || got[0] != title {
		t.Errorf("recipe-title = %q", got)
	}
	// Each user string as placed on the page: one text element holding the
	// string, not markup parsed into styled pieces.
	r := d.Recipes[0]
	for label, want := range map[string]string{
		"title-text":      title,
		"description":     r.Description,
		"tags":            strings.Join(r.Tags, " · "),
		"ingredient-name": r.IngredientGroups[0].Ingredients[0].Name,
	} {
		vals := queryTemplate(t, d, builtin(), label)
		var placed struct{ Func, Text string }
		if len(vals) == 0 || json.Unmarshal(vals[0], &placed) != nil || placed.Func != "text" || placed.Text != want {
			t.Errorf("%s = %s, want the text %q", label, vals, want)
		}
	}
}

func TestLongRecipeBreaks(t *testing.T) {
	d := longFixture(t)
	set := builtin()
	if n := pageCount(t, d, set); n < 2 {
		t.Errorf("pages = %d", n)
	}
	want := d.Recipes[0].Title + " (" + d.Labels.Continued + ")"
	if got := strs(t, queryTemplate(t, d, set, "running-header")); len(got) == 0 || got[0] != want {
		t.Errorf("running-header = %q, want %q", got, want)
	}
}

// A group over 12 rows that runs onto the next page repeats its name there
// with "(cont.)", on the page and on the card.
func TestOversizedGroupRepeatsHeading(t *testing.T) {
	for _, paper := range []Paper{PaperA4, PaperA5} {
		d := longFixture(t)
		d.Options.Paper = paper
		r := &d.Recipes[0]
		var all []PDFIngredient
		for _, g := range r.IngredientGroups {
			all = append(all, g.Ingredients...)
		}
		r.IngredientGroups = []Group{{Name: r.IngredientGroups[0].Name, Ingredients: all}}
		got := strs(t, queryTemplate(t, d, builtin(), "group-heading"))
		want := *r.IngredientGroups[0].Name + " (" + d.Labels.Continued + ")"
		if !contains(got, *r.IngredientGroups[0].Name) || !contains(got, want) {
			t.Errorf("%s: group-heading = %q, want the name and %q", paper, got, want)
		}
	}
}

// A group moved whole to the next page reads its plain name there; only a
// later page of the same group says "(cont.)". The small groups before it
// grow row by row, so some variants move it whole to page 2 and others split it.
func TestMovedGroupIsNotContinued(t *testing.T) {
	for m := 1; m <= 12; m++ {
		d := longFixture(t)
		r := &d.Recipes[0]
		r.Steps = r.Steps[:1]
		var all []PDFIngredient
		for _, g := range r.IngredientGroups {
			all = append(all, g.Ingredients...)
		}
		big, filler := "Groß", "Klein"
		r.IngredientGroups = []Group{
			{Name: &filler, Ingredients: all[:12]},
			{Name: &filler, Ingredients: all[12 : 12+m]},
			{Name: &big, Ingredients: all[30:60]},
		}
		got := strs(t, queryTemplate(t, d, builtin(), "group-heading"))
		for _, h := range got {
			if strings.HasPrefix(h, big) {
				if h != big {
					t.Errorf("filler of %d rows: the big group's first heading is %q", m, h)
				}
				break
			}
		}
	}
}

// longFixture is the Käsespätzle grown to 10 groups of 10 ingredients and 50
// long steps, so it needs several pages.
func longFixture(t *testing.T) Data {
	d := fixture(t)
	r := &d.Recipes[0]
	r.Title = "Sehr langes Rezept"
	r.IngredientGroups, r.Steps = nil, nil
	for g := range 10 {
		name := fmt.Sprintf("Gruppe %d", g+1)
		grp := Group{Name: &name}
		for i := range 10 {
			unit, n := "g", fmt.Sprintf("Zutat %d.%d", g+1, i)
			q := strconv.Itoa(i + 1)
			grp.Ingredients = append(grp.Ingredients, PDFIngredient{
				Quantity: &Quantity{Value: float64(i + 1), Display: q}, Unit: &unit, Name: n, Display: q + " g " + n,
			})
		}
		r.IngredientGroups = append(r.IngredientGroups, grp)
	}
	for i := range 50 {
		text := fmt.Sprintf("Schritt %d: ", i+1) + strings.Repeat("Lorem ipsum dolor sit amet, consectetur adipiscing elit. ", 3)
		r.Steps = append(r.Steps, PDFStep{Number: i + 1, Text: text, Segments: []Segment{{Kind: "text", Text: text}}})
	}
	return d
}

func bookFixture(t *testing.T) Data {
	d := fixture(t)
	d.Options.Bundle = BundleBook
	d.Book = &Book{Title: "Rezepte", Date: "2026-11-25", DateDisplay: "November 2026", CountDisplay: "3 Rezepte"}
	// Each copy has its own servings and total time, so a contents entry
	// that shows the wrong recipe's facts is caught.
	for i, title := range []string{"Flammkuchen", "Königsberger Klopse"} {
		rec := d.Recipes[0]
		rec.Title = title
		rec.Servings.Display = strconv.Itoa(2 + 4*i)
		rec.Times.Total = &Duration{Minutes: 30 * (i + 1), Display: strconv.Itoa(30*(i+1)) + " Min"}
		d.Recipes = append(d.Recipes, rec)
	}
	return d
}

func TestBookHasTOC(t *testing.T) {
	vals := queryTemplate(t, bookFixture(t), builtin(), "toc-entry")
	if len(vals) != 3 {
		t.Fatalf("toc-entry = %s", vals)
	}
	var first int
	if err := json.Unmarshal(vals[0], &first); err != nil || first != 3 {
		t.Errorf("first recipe on page %s, want 3", vals[0])
	}
}

func TestBookA5Renders(t *testing.T) {
	d := bookFixture(t)
	d.Options.Paper = PaperA5
	if n := pageCount(t, d, builtin()); n != 5 {
		t.Errorf("pages = %d, want title, contents and three cards", n)
	}
}

type placed struct{ Recipe, Number, Page int }

// pages returns the page of each recipe's metadata labeled label.
func pages(t *testing.T, d Data, label string) map[int]int {
	t.Helper()
	out := map[int]int{}
	for _, v := range queryTemplate(t, d, builtin(), label) {
		var p placed
		if err := json.Unmarshal(v, &p); err != nil {
			t.Fatalf("%s = %s", label, v)
		}
		out[p.Recipe] = p.Page
	}
	return out
}

// stepPages returns the page of every step, per recipe, in step order.
func stepPages(t *testing.T, d Data) map[int][]int {
	t.Helper()
	out := map[int][]int{}
	for _, v := range queryTemplate(t, d, builtin(), "step") {
		var p placed
		if err := json.Unmarshal(v, &p); err != nil {
			t.Fatalf("step = %s", v)
		}
		out[p.Recipe] = append(out[p.Recipe], p.Page)
	}
	return out
}

// withSteps gives every recipe of d n steps, cycling through its own.
func withSteps(d Data, n int) Data {
	d.Recipes = append([]Recipe(nil), d.Recipes...)
	for i := range d.Recipes {
		src := d.Recipes[i].Steps
		var steps []PDFStep
		for k := range n {
			s := src[k%len(src)]
			s.Number = k + 1
			steps = append(steps, s)
		}
		d.Recipes[i].Steps = steps
	}
	return d
}

// checkPagination asserts the rules every recipe keeps: the last step shares
// its page with the footer (unless the footer follows a list that runs on
// below the steps), the page a method starts on holds its first two steps,
// and on A4 the method starts on the recipe's first page, beside the list.
func checkPagination(t *testing.T, d Data, name string) map[int][]int {
	t.Helper()
	return checkPages(t, d, name, true)
}

func checkPages(t *testing.T, d Data, name string, footerWithStep bool) map[int][]int {
	t.Helper()
	steps, foot, start := stepPages(t, d), pages(t, d, "footer"), pages(t, d, "recipe-page")
	for i := range d.Recipes {
		p := steps[i]
		if len(p) == 0 {
			continue
		}
		last := p[len(p)-1]
		if footerWithStep && last != foot[i] || foot[i] < last {
			t.Errorf("%s: recipe %d's last step on page %d, its footer on page %d", name, i, last, foot[i])
		}
		if d.Options.Paper == PaperA4 && p[0] != start[i] {
			t.Errorf("%s: recipe %d starts on page %d, its method on page %d", name, i, start[i], p[0])
		}
		if len(p) > 1 && p[1] != p[0] {
			t.Errorf("%s: recipe %d's method starts on page %d with one step alone (pages %v)", name, i, p[0], p)
		}
		// R6: the footer's page holds the last two steps, never one alone.
		if footerWithStep && len(p) > 1 && p[len(p)-2] != foot[i] {
			t.Errorf("%s: recipe %d's footer on page %d, its second-to-last step on page %d", name, i, foot[i], p[len(p)-2])
		}
	}
	return steps
}

// The footer keeps with the last step: a recipe that grows a step at a time
// crosses the point where its steps still fit the page and its footer does
// not, and there the last step moves on with the footer, so no page holds
// the running header and the footer alone. With the ingredients doubled the
// list runs on below the steps on A4, like the Königsberger Klopse. Every
// size also keeps the first two steps together.
func TestFooterKeepsWithLastStep(t *testing.T) {
	for _, paper := range []Paper{PaperA4, PaperA5} {
		for _, variant := range []string{"recipe", "book", "long list"} {
			base := fixture(t)
			if variant == "book" {
				base = bookFixture(t)
			}
			if variant == "long list" {
				r := &base.Recipes[0]
				r.IngredientGroups = append(r.IngredientGroups, r.IngredientGroups...)
			}
			base.Options.Paper = paper
			for n := 1; n <= 22; n++ {
				// Up to three steps beside a longer list: the footer follows the list.
				few := paper == PaperA4 && variant == "long list" && n <= 3
				checkPages(t, withSteps(base, n), fmt.Sprintf("%s %s, %d steps", paper, variant, n), !few)
			}
		}
	}
}

// The card's method is never split when it fits a page: the first size that
// spills puts every step on page 2 with the footer, ingredients alone on
// page 1, and a few steps more still go there whole. In a book the same
// holds per recipe, and the next recipe starts on a page of its own.
func TestCardMethodMovesWhole(t *testing.T) {
	for _, book := range []bool{false, true} {
		base := fixture(t)
		if book {
			base = bookFixture(t)
		}
		base.Options.Paper = PaperA5
		// The first recipe starts on page 1, or 3 after the book's title
		// page and contents; it spills once its last step is past that page.
		start, spill := 1, 0
		if book {
			start = 3
		}
		for n := 1; n <= 30 && spill == 0; n++ {
			if p := stepPages(t, withSteps(base, n))[0]; p[len(p)-1] > start {
				spill = n
			}
		}
		if spill == 0 {
			t.Fatalf("book=%v: the card never spilled", book)
		}
		for n := spill; n < spill+4; n++ {
			d := withSteps(base, n)
			steps := checkPagination(t, d, fmt.Sprintf("book=%v, %d steps", book, n))
			for i, p := range steps {
				if p[0] != p[len(p)-1] {
					t.Errorf("book=%v, %d steps: recipe %d's method split over pages %v", book, n, i, p)
				}
				if i > 0 && p[0] <= steps[i-1][len(steps[i-1])-1] {
					t.Errorf("book=%v, %d steps: recipe %d starts on page %d, recipe %d ends there", book, n, i, p[0], i-1)
				}
			}
			if steps[0][0] != start+1 {
				t.Errorf("book=%v, %d steps: the method starts on page %d, want %d", book, n, steps[0][0], start+1)
			}
		}
	}
}

// A method longer than a page flows on: it starts where the ingredients end
// with at least two steps (on A4 under its heading, beside the list), and
// its last step keeps the footer. The Sauerbraten shape: a list with a group
// over 12 rows, many steps and one over 12 lines.
func TestLongMethodFlows(t *testing.T) {
	for _, paper := range []Paper{PaperA4, PaperA5} {
		d := longFixture(t)
		d.Options.Paper = paper
		r := &d.Recipes[0]
		long := strings.Repeat("Den Braten bei kleiner Hitze schmoren, gelegentlich wenden und mit dem Fond übergießen. ", 14)
		r.Steps[3] = PDFStep{Number: 4, Text: long, Segments: []Segment{{Kind: "text", Text: long}}}
		steps := checkPagination(t, d, string(paper))
		if p := steps[0]; p[len(p)-1] == p[0] {
			t.Errorf("%s: the method fits one page (%v), want it longer", paper, p)
		}
		if paper == PaperA4 && steps[0][0] != 1 {
			t.Errorf("A4: the method starts on page %d, beside the ingredients on page 1", steps[0][0])
		}
	}
}

// The card with more than 12 ingredient rows sets them in one column and
// keeps the rules.
func TestCardLongList(t *testing.T) {
	d := fixture(t)
	d.Options.Paper = PaperA5
	r := &d.Recipes[0]
	r.IngredientGroups = append(r.IngredientGroups, r.IngredientGroups...)
	for n := 1; n <= 12; n++ {
		checkPagination(t, withSteps(d, n), fmt.Sprintf("%d steps", n))
	}
}

// On A4 a list that runs on below up to three steps never pushes the method
// off the list's first page: beside long steps the rail grows a row at a time
// down to the page foot and past it, never more than `list-first` below the
// steps, and the steps stay beside its start. The footer then follows the
// list.
func TestLongListKeepsFewStepsBeside(t *testing.T) {
	for n := 1; n <= 3; n++ {
		base := fixture(t)
		r := &base.Recipes[0]
		// About 2,100 characters in all, most in the last step, which breaks.
		r.Steps = nil
		for k := range n {
			words := 4
			if k == n-1 {
				words = 75 - 4*k
			}
			r.Steps = append(r.Steps, textStep(k+1, strings.Repeat("Lorem ipsum dolor sit amet. ", words)))
		}
		var all []PDFIngredient
		for range 5 {
			for _, g := range r.IngredientGroups {
				all = append(all, g.Ingredients...)
			}
		}
		spilled := false
		for rows := 8; rows <= len(all); rows += 2 {
			d := base
			d.Recipes = append([]Recipe(nil), base.Recipes...)
			d.Recipes[0].IngredientGroups = []Group{{Ingredients: all[:rows]}}
			name := fmt.Sprintf("%d steps, %d rows", n, rows)
			if xs := groupColumns(t, d); xs[0] < 100 {
				t.Errorf("%s: the list went first (x %.0f mm)", name, xs[0])
			}
			checkPages(t, d, name, false)
			var rowPages []int
			for _, v := range evalTemplate(t, d, builtin(), "query(<ingredient-name>).map(m => m.location().page())") {
				var p int
				_ = json.Unmarshal(v, &p)
				rowPages = append(rowPages, p)
			}
			spilled = spilled || rowPages[len(rowPages)-1] > 1
		}
		if !spilled {
			t.Errorf("%d steps: the list never reached the page foot", n)
		}
	}
}

// rinderrouladen is the demo's Rinderrouladen: six steps, 7 + 6 rows.
func rinderrouladen(t *testing.T) Data { return loadFixture(t, "rinderrouladen") }

// A recipe of six steps and 13 rows fits one page with photo and footer, on
// A4 and on the card: the compact type and the band at the foot leave room.
func TestRinderrouladenFitsOnePage(t *testing.T) {
	for _, paper := range []Paper{PaperA4, PaperA5} {
		d := rinderrouladen(t)
		d.Options.Paper = paper
		if n := pageCount(t, d, builtin()); n != 1 {
			t.Errorf("%s: pages = %d, want 1", paper, n)
		}
	}
}

// The demo's Sauerbraten, 15 steps beside 28 rows, prints on these pages.
// With the photo on A4 it takes three: page 1 ends after step 6 with about
// 16 mm left, and step 7 needs four lines (24 mm with its gap); page 2 then
// lacks about 13 mm for steps 14 and 15 with the footer band, which the
// footer keeps together. Without the photo or the footer it takes two.
func TestSauerbratenPages(t *testing.T) {
	for _, c := range []struct {
		paper         Paper
		photo, footer bool
		pages         int
	}{
		{PaperA4, true, true, 3},
		{PaperA4, false, true, 2},
		{PaperA4, true, false, 2},
		{PaperA5, true, true, 3},
		{PaperA5, false, false, 3},
	} {
		d := loadFixture(t, "sauerbraten")
		d.Options.Paper, d.Options.Photo, d.Options.Footer = c.paper, c.photo, c.footer
		if n := pageCount(t, d, builtin()); n != c.pages {
			t.Errorf("%s photo=%v footer=%v: pages = %d, want %d", c.paper, c.photo, c.footer, n, c.pages)
		}
	}
}

type footBottom struct {
	Recipe, Page int
	Y            float64
}

// The footer band ends on the content's bottom edge of each recipe's last
// page (A4 297 - 14 mm, A5 210 - 11 mm), whether the recipe takes one page or
// several, with the footer on or off.
func TestFooterAtPageFoot(t *testing.T) {
	cases := map[string]Data{"one page": fixture(t), "rinderrouladen": rinderrouladen(t), "long": longFixture(t), "book": withSteps(bookFixture(t), 14)}
	for name, base := range cases {
		for _, paper := range []Paper{PaperA4, PaperA5} {
			for _, footer := range []bool{true, false} {
				d := base
				d.Options.Paper, d.Options.Footer = paper, footer
				label := fmt.Sprintf("%s %s footer=%v", name, paper, footer)
				want := 283.0
				if paper == PaperA5 {
					want = 199
				}
				ends := recipeEnds(t, d)
				vals := queryTemplate(t, d, builtin(), "footer-bottom")
				if len(vals) != len(d.Recipes) {
					t.Fatalf("%s: footer-bottom = %s", label, vals)
				}
				for _, v := range vals {
					var f footBottom
					if err := json.Unmarshal(v, &f); err != nil {
						t.Fatalf("%s: footer-bottom = %s", label, v)
					}
					if f.Y < want-0.3 || f.Y > want+0.3 {
						t.Errorf("%s: recipe %d's footer ends at %.1f mm, want %.0f", label, f.Recipe, f.Y, want)
					}
					if f.Page != ends[f.Recipe] {
						t.Errorf("%s: recipe %d's footer on page %d, its last page is %d", label, f.Recipe, f.Page, ends[f.Recipe])
					}
				}
			}
		}
	}
}

// recipeEnds returns each recipe's last page: the page before the next
// recipe starts, or the document's last.
func recipeEnds(t *testing.T, d Data) map[int]int {
	t.Helper()
	start, n := pages(t, d, "recipe-page"), pageCount(t, d, builtin())
	ends := map[int]int{}
	for i := range d.Recipes {
		ends[i] = n
		if s, ok := start[i+1]; ok {
			ends[i] = s - 1
		}
	}
	return ends
}

// A recipe's pages 2 and later carry "<title> (cont.)" at the head, its first
// page nothing.
func TestRunningTitleFromPageTwo(t *testing.T) {
	for _, paper := range []Paper{PaperA4, PaperA5} {
		for name, d := range map[string]Data{"one page": fixture(t), "long": longFixture(t)} {
			d.Options.Paper = paper
			got := strs(t, queryTemplate(t, d, builtin(), "running-header"))
			n := pageCount(t, d, builtin())
			if name == "long" && n < 2 {
				t.Fatalf("%s long: %d page, want several", paper, n)
			}
			if len(got) != n-1 {
				t.Errorf("%s %s: %d running titles on %d pages, want one on every page but the first", paper, name, len(got), n)
			}
			for _, h := range got {
				if want := d.Recipes[0].Title + " (" + d.Labels.Continued + ")"; h != want {
					t.Errorf("%s %s: running title %q, want %q", paper, name, h, want)
				}
			}
		}
	}
}

// Every page a recipe goes on from carries the "next page" hint at its foot;
// the recipe's last page does not. In a book each recipe counts for itself.
func TestNextPageHint(t *testing.T) {
	for _, paper := range []Paper{PaperA4, PaperA5} {
		for name, d := range map[string]Data{"one page": fixture(t), "long": longFixture(t), "book": withSteps(bookFixture(t), 14)} {
			d.Options.Paper = paper
			if n := pageCount(t, d, builtin()); name != "one page" && n < 2 {
				t.Fatalf("%s %s: %d page, want several", paper, name, n)
			}
			start, ends := pages(t, d, "recipe-page"), recipeEnds(t, d)
			want := map[int]bool{}
			for i := range d.Recipes {
				for p := start[i]; p < ends[i]; p++ {
					want[p] = true
				}
			}
			got := map[int]bool{}
			for _, v := range queryTemplate(t, d, builtin(), "next-page") {
				var p placed
				if err := json.Unmarshal(v, &p); err != nil {
					t.Fatalf("next-page = %s", v)
				}
				got[p.Page] = true
			}
			if fmt.Sprint(got) != fmt.Sprint(want) {
				t.Errorf("%s %s: hint on pages %v, want %v", paper, name, got, want)
			}
		}
	}
}

// A book numbers its pages "x / y" at the head from the contents on, x
// continuous over the whole book, and the contents point at the same numbers.
func TestBookFolios(t *testing.T) {
	d := withSteps(bookFixture(t), 14)
	n := pageCount(t, d, builtin())
	var got []int
	for _, v := range queryTemplate(t, d, builtin(), "folio") {
		var f struct {
			Page int
			Text string
		}
		if err := json.Unmarshal(v, &f); err != nil {
			t.Fatalf("folio = %s", v)
		}
		if want := fmt.Sprintf("%d / %d", f.Page, n); f.Text != want {
			t.Errorf("folio on page %d reads %q, want %q", f.Page, f.Text, want)
		}
		got = append(got, f.Page)
	}
	if len(got) != n-1 || got[0] != 2 || got[len(got)-1] != n {
		t.Errorf("folios on pages %v, want 2 to %d", got, n)
	}
	start := pages(t, d, "recipe-page")
	for i, v := range queryTemplate(t, d, builtin(), "toc-entry") {
		var p int
		if err := json.Unmarshal(v, &p); err != nil || p != start[i] {
			t.Errorf("contents entry %d points at page %s, the recipe starts on %d", i, v, start[i])
		}
	}
	if n <= 2+len(d.Recipes) {
		t.Errorf("pages = %d, want recipes over several pages", n)
	}
}

type stepAt struct {
	Recipe, Number, Page int
	Y                    float64
	Long                 bool
}

func stepsAt(t *testing.T, d Data) []stepAt {
	t.Helper()
	var out []stepAt
	for _, v := range queryTemplate(t, d, builtin(), "step") {
		var s stepAt
		if err := json.Unmarshal(v, &s); err != nil {
			t.Fatalf("step = %s", v)
		}
		out = append(out, s)
	}
	return out
}

// textStep is a step of one text segment.
func textStep(n int, text string) PDFStep {
	return PDFStep{Number: n, Text: text, Segments: []Segment{{Kind: "text", Text: text}}}
}

// A step may break across pages past 640 characters on A4 and 800 on the
// card, counted as characters (not bytes), amounts included.
func TestLongStepRule(t *testing.T) {
	for paper, limit := range map[Paper]int{PaperA4: 640, PaperA5: 800} {
		d := fixture(t)
		d.Options.Paper = paper
		amount := "5 g"
		withAmount := PDFStep{Number: 3, Segments: []Segment{
			{Kind: "text", Text: strings.Repeat("ü", limit-3)},
			{Kind: "ingredient", Text: "x", Amount: &amount},
		}}
		d.Recipes[0].Steps = []PDFStep{textStep(1, strings.Repeat("ü", limit)), textStep(2, strings.Repeat("ü", limit+1)), withAmount}
		var got []bool
		for _, s := range stepsAt(t, d) {
			got = append(got, s.Long)
		}
		if fmt.Sprint(got) != "[false true true]" {
			t.Errorf("%s: long = %v, want [false true true]", paper, got)
		}
	}
}

// paellaShape is the fixture with five groups of 6, 5, 8, 7 and 5 rows
// beside four short steps. With notes, the first group's rows wrap.
func paellaShape(t *testing.T, notes bool) Data {
	d := longFixture(t)
	r := &d.Recipes[0]
	r.Title = "Paella"
	note := "in grobe Stücke geschnitten und beiseitegestellt"
	for i, n := range []int{6, 5, 8, 7, 5} {
		g := r.IngredientGroups[i]
		g.Ingredients = append([]PDFIngredient(nil), g.Ingredients[:n]...)
		if i == 0 && notes {
			for k := range g.Ingredients {
				g.Ingredients[k].Note = &note
			}
		}
		r.IngredientGroups[i] = g
	}
	r.IngredientGroups = r.IngredientGroups[:5]
	short := "Zwiebeln, Knoblauch, Paprika und Bohnen zugeben und 8 Minuten mitbraten."
	r.Steps = []PDFStep{textStep(1, short), textStep(2, short), textStep(3, short), textStep(4, short)}
	return d
}

// groupColumns returns the left edge of each group's column, in mm.
func groupColumns(t *testing.T, d Data) []float64 {
	t.Helper()
	var xs []float64
	for _, v := range queryTemplate(t, d, builtin(), "group-start") {
		var g struct{ X float64 }
		if err := json.Unmarshal(v, &g); err != nil {
			t.Fatalf("group-start = %s", v)
		}
		xs = append(xs, math.Round(g.X))
	}
	return xs
}

// On A4 a list that runs on far below a short method goes above it in three
// columns, and the recipe keeps every pagination rule, the footer with the
// last two steps included: Paella's 31 rows beside four short steps. The
// Rinderrouladen's list, shorter than its method, stays the rail beside it.
func TestIngredientsFirst(t *testing.T) {
	for name, c := range map[string]struct {
		d    Data
		cols int
	}{"paella": {paellaShape(t, false), 3}, "rinderrouladen": {rinderrouladen(t), 1}} {
		cols := map[float64]bool{}
		for _, x := range groupColumns(t, c.d) {
			cols[x] = true
		}
		if len(cols) != c.cols {
			t.Errorf("%s: groups in %d columns, want %d", name, len(cols), c.cols)
		}
		checkPagination(t, c.d, name)
	}
	if steps := stepsAt(t, paellaShape(t, false)); steps[0].Y < 150 {
		t.Errorf("paella: step 1 at %.0f mm, want it below the list", steps[0].Y)
	}
}

// The card balances its two columns by the groups' set heights, as CSS
// columns do: with the first group's rows wrapping, two groups fill the left
// column, where counting rows would put three there.
func TestCardColumnsBalanceByHeight(t *testing.T) {
	d := paellaShape(t, true)
	d.Options.Paper = PaperA5
	xs := groupColumns(t, d)
	left := 0
	for _, x := range xs {
		if x == xs[0] {
			left++
		}
	}
	if left != 2 {
		t.Errorf("left column holds %d groups (columns %v), want 2", left, xs)
	}
}

// One long group without a name is cut once over two of the card's
// columns, never set whole in the first.
func TestLongGroupSplitsOverColumns(t *testing.T) {
	d := fixture(t)
	d.Options.Paper = PaperA5
	r := &d.Recipes[0]
	var all []PDFIngredient
	for range 2 {
		for _, g := range r.IngredientGroups {
			all = append(all, g.Ingredients...)
		}
	}
	r.IngredientGroups = []Group{{Ingredients: all[:14]}}
	if xs := groupColumns(t, d); len(xs) != 2 || xs[0] == xs[1] {
		t.Errorf("group parts at %v mm, want two columns", xs)
	}
}

// A method that runs on to the next page starts at the content's top there,
// on A4 and on the card: nothing of the page before repeats above it.
func TestContinuationStartsAtTop(t *testing.T) {
	for paper, top := range map[Paper]float64{PaperA4: 14, PaperA5: 11} {
		d := withSteps(fixture(t), 40)
		d.Options.Paper = paper
		seen := map[int]bool{}
		for _, s := range stepsAt(t, d) {
			if seen[s.Page] {
				continue
			}
			seen[s.Page] = true
			if s.Page > 1 && math.Abs(s.Y-top) > 0.1 {
				t.Errorf("%s: step %d opens page %d at %.1f mm, want %.0f", paper, s.Number, s.Page, s.Y, top)
			}
		}
		if len(seen) < 2 {
			t.Fatalf("%s: the method fits one page", paper)
		}
	}
}

// With photos the covers fill the page down to the title block at its
// foot; without, the title stands in the upper third. Both formats.
func TestBookTitlePlacement(t *testing.T) {
	for paper, height := range map[Paper]float64{PaperA4: 297, PaperA5: 210} {
		for _, photo := range []bool{false, true} {
			d := bookFixture(t)
			d.Options.Paper, d.Options.Photo = paper, photo
			vals := queryTemplate(t, d, builtin(), "book-title")
			var y float64
			if len(vals) != 1 || json.Unmarshal(vals[0], &y) != nil {
				t.Fatalf("book-title = %s", vals)
			}
			if photo && y < 0.7*height || !photo && y > height/2 {
				t.Errorf("%s photo=%v: the title at %.0f mm of %.0f", paper, photo, y, height)
			}
		}
	}
}

// The covers grow down to the title block, leaving no void between.
func TestBookCollageFillsThePage(t *testing.T) {
	for paper, content := range map[Paper]float64{PaperA4: 269, PaperA5: 188} {
		d := bookFixture(t)
		d.Options.Paper = paper
		vals := queryTemplate(t, d, builtin(), "book-collage")
		var h float64
		if len(vals) != 1 || json.Unmarshal(vals[0], &h) != nil {
			t.Fatalf("book-collage = %s", vals)
		}
		if h < 0.65*content {
			t.Errorf("%s: the covers are %.0f mm of %.0f", paper, h, content)
		}
	}
}

// The product name signs the title page only when it is not the title.
func TestBookSignatureOnlyWhenNotTheTitle(t *testing.T) {
	for title, want := range map[string]int{"Rezepte": 0, "Unsere Familienrezepte": 1} {
		d := bookFixture(t)
		d.Book.Title = title
		if n := len(queryTemplate(t, d, builtin(), "book-signature")); n != want {
			t.Errorf("title %q: %d signatures, want %d", title, n, want)
		}
	}
}

// The title page is signed only when the instance credits Rezepte, as the
// recipe footer is.
func TestBookSignatureNeedsAttribution(t *testing.T) {
	d := bookFixture(t)
	d.Book.Title = "Unsere Familienrezepte"
	d.Instance.Attribution = false
	if got := queryTemplate(t, d, builtin(), "book-signature"); len(got) != 0 {
		t.Errorf("signed without attribution: %s", got)
	}
}

// Each contents entry carries the recipe's servings and total time.
func TestContentsShowServingsAndTime(t *testing.T) {
	d := bookFixture(t)
	vals := queryTemplate(t, d, builtin(), "toc-meta")
	if len(vals) != len(d.Recipes) {
		t.Fatalf("toc-meta = %s", vals)
	}
	for i, r := range d.Recipes {
		want, _ := json.Marshal([]string{r.Servings.Display, r.Times.Total.Display})
		if string(vals[i]) != string(want) {
			t.Errorf("entry %d: toc-meta = %s, want %s", i, vals[i], want)
		}
	}
}

type nameSize struct {
	Width, Height, Column float64
	Broken                bool
}

// nameSizes returns each ingredient name cell's set size, in mm, and
// whether `fit` broke a word in it between any two letters.
func nameSizes(t *testing.T, d Data) []nameSize {
	t.Helper()
	var out []nameSize
	for _, v := range queryTemplate(t, d, builtin(), "name-size") {
		var n nameSize
		if err := json.Unmarshal(v, &n); err != nil {
			t.Fatalf("name-size = %s", v)
		}
		out = append(out, n)
	}
	return out
}

// A word that hyphenation cannot fit breaks between any two letters onto
// more lines, never running past its cell into the next column or the
// clip, in the rail and on the card.
func TestLongIngredientNameIsNeverClipped(t *testing.T) {
	for _, paper := range []Paper{PaperA4, PaperA5} {
		d := fixture(t)
		if paper == PaperA5 {
			d.Options.Paper = PaperA5
		}
		g := &d.Recipes[0].IngredientGroups[0]
		g.Ingredients[0].Name, g.Ingredients[0].Note = strings.Repeat("x", 60), nil
		n := nameSizes(t, d)
		if len(n) < 2 {
			t.Fatalf("%s: name-size = %v", paper, n)
		}
		if !n[0].Broken || n[0].Width >= n[0].Column || n[0].Height < 2.5*n[1].Height {
			t.Errorf("%s: long name %+v, a short one %+v", paper, n[0], n[1])
		}
	}
}

// A name a hair too wide for its column hyphenates rather than overflowing
// or leaving its comma alone: Paella's three columns leave
// "Artischockenherzen," a hair too narrow, and the name takes two lines.
func TestIngredientNameHyphenates(t *testing.T) {
	d := paellaShape(t, false)
	note := "geviertelt"
	in := &d.Recipes[0].IngredientGroups[1].Ingredients[0]
	in.Name, in.Note = "Artischockenherzen", &note
	n := nameSizes(t, d)
	if len(n) < 7 {
		t.Fatalf("name-size = %v", n)
	}
	line := n[0].Height
	for _, s := range n {
		line = min(line, s.Height)
	}
	if a := n[6]; a.Broken || a.Width >= a.Column || math.Abs(a.Height-2*line) > 0.01 {
		t.Errorf("Artischockenherzen, geviertelt: %+v, one line %.2f mm", a, line)
	}
}

// A short contents stands a third down its page, not in the top quarter,
// and a long one starts at the top. Both formats.
func TestContentsPlacement(t *testing.T) {
	for paper, height := range map[Paper]float64{PaperA4: 297, PaperA5: 210} {
		for n, low := range map[int]bool{3: true, 40: false} {
			d := bookFixture(t)
			d.Options.Paper = paper
			for len(d.Recipes) < n {
				d.Recipes = append(d.Recipes, d.Recipes[0])
			}
			vals := queryTemplate(t, d, builtin(), "toc-title")
			var y float64
			if len(vals) != 1 || json.Unmarshal(vals[0], &y) != nil {
				t.Fatalf("toc-title = %s", vals)
			}
			if low && y < height/4 || !low && y > height/6 {
				t.Errorf("%s, %d recipes: contents at %.0f mm of %.0f", paper, n, y, height)
			}
		}
	}
}

// The quantity and unit columns are one pair per recipe, each as wide as its
// widest cell across every group: the names of the Spätzle group ("200 g")
// and the Röstzwiebeln group ("1 Stück", "0,5 Prise") start on one vertical,
// and with no long unit that vertical sits left of the old 9 mm + 12 mm
// floors (A5: margin 18 mm).
func TestIngredientColumnsPerRecipe(t *testing.T) {
	d := fixture(t)
	d.Options.Paper = PaperA5
	vals := evalTemplate(t, d, builtin(), "query(<ingredient-name>).map(m => m.location().position().x.mm())")
	var xs []float64
	for _, v := range vals {
		var x float64
		if json.Unmarshal(v, &x) != nil {
			t.Fatalf("ingredient-name x = %s", vals)
		}
		xs = append(xs, x)
	}
	if len(xs) < 2 {
		t.Fatalf("ingredient-name x = %v", xs)
	}
	// Names in the second column sit one column width and gap to the right.
	col := (120.0 - 7) / 2 // the card's content width, two columns
	for _, x := range xs {
		if x > 18+col {
			x -= col + 7
		}
		if math.Abs(x-xs[0]) > 0.05 {
			t.Errorf("names start at %v, want one vertical", xs)
			break
		}
	}
	if xs[0] > 18+9+12-3 {
		t.Errorf("names start at %.2f mm, want the columns sized by their text", xs[0])
	}
}

// floats evaluates expr, which yields an array of numbers.
func floats(t *testing.T, d Data, expr string) []float64 {
	t.Helper()
	vals := evalTemplate(t, d, builtin(), expr)
	out := make([]float64, len(vals))
	for i, v := range vals {
		if err := json.Unmarshal(v, &out[i]); err != nil {
			t.Fatalf("not a number: %s", v)
		}
	}
	return out
}

// Step numbers hang right-aligned in their column, so "9" and "12" share a
// right edge, and the text starts a fixed gap after it (A4: margin 22 mm,
// column 6 mm, gap 2.5 mm; A5: 18, 5 and 2 mm).
func TestStepNumbersHang(t *testing.T) {
	for paper, x := range map[Paper][3]float64{PaperA4: {22, 6, 2.5}, PaperA5: {18, 5, 2}} {
		d := withSteps(rinderrouladen(t), 12)
		d.Options.Paper = paper
		// Where each number's box starts plus the number's width.
		ends := floats(t, d, "query(<step-number>).map(m => m.location().position().x.mm() + m.value)")
		starts := floats(t, d, "query(<step-text>).map(m => m.location().position().x.mm())")
		if len(ends) != 12 || len(starts) != 12 {
			t.Fatalf("%s: %d numbers, %d texts", paper, len(ends), len(starts))
		}
		for i := range ends {
			if math.Abs(ends[i]-(x[0]+x[1])) > 0.05 || math.Abs(starts[i]-(x[0]+x[1]+x[2])) > 0.05 {
				t.Errorf("%s: step %d's number ends at %.2f mm, its text starts at %.2f mm, want %.1f and %.1f",
					paper, i+1, ends[i], starts[i], x[0]+x[1], x[0]+x[1]+x[2])
			}
		}
	}
}

// Servings printed at another count than the recipe's say so: A4 under the
// value in the facts row, the card in its meta. A recipe at its own count
// prints no note.
func TestScaledFromNote(t *testing.T) {
	for paper, want := range map[Paper]string{PaperA4: "umgerechnet von 4", PaperA5: "von 4"} {
		d := rinderrouladen(t)
		d.Options.Paper = paper
		if got := queryTemplate(t, d, builtin(), "scaled-from"); len(got) != 0 {
			t.Errorf("%s at its own servings: %s", paper, got)
		}
		d.Recipes[0].Servings.Value, d.Recipes[0].Servings.Display = 8, "8"
		if got := strs(t, queryTemplate(t, d, builtin(), "scaled-from")); len(got) != 1 || got[0] != want {
			t.Errorf("%s scaled to 8: %q, want %q", paper, got, want)
		}
	}
}

// The A4 head breathes: 3 mm under the title (one line, 30 pt at 1.05)
// before the description.
func TestHeadSpacing(t *testing.T) {
	d := rinderrouladen(t)
	title := floats(t, d, "query(<title-text>).map(m => m.location().position().y.mm())")
	desc := floats(t, d, "query(<description>).map(m => m.location().position().y.mm())")
	if len(title) != 1 || len(desc) != 1 {
		t.Fatalf("title %v, description %v", title, desc)
	}
	if want := 30*1.05*25.4/72 + 3; math.Abs(desc[0]-title[0]-want) > 0.05 {
		t.Errorf("description %.2f mm below the title's top, want %.2f", desc[0]-title[0], want)
	}
}

// The A4 contents keep the steps' solo measure of 120 mm, so the leaders
// stay short; the card's take its full width, 120 mm as well.
func TestContentsMeasure(t *testing.T) {
	for _, paper := range []Paper{PaperA4, PaperA5} {
		d := bookFixture(t)
		d.Options.Paper = paper
		widths := floats(t, d, "query(<toc-width>).map(m => m.value)")
		if len(widths) != len(d.Recipes) {
			t.Fatalf("%s: toc-width = %v", paper, widths)
		}
		for _, w := range widths {
			if math.Abs(w-120) > 0.05 {
				t.Errorf("%s: contents entry %.2f mm wide, want 120", paper, w)
			}
		}
	}
}

// The page prints for office printers: hairlines in the print rule color,
// which prints solid rather than as a dotted tint, and small text in a
// neutral near-black that drivers print with black toner alone.
func TestPrintColors(t *testing.T) {
	r := newTestRenderer(t, builtin())
	res, err := r.Render(t.Context(), rinderrouladen(t), fixturePhotos(t))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Close()
	b, err := os.ReadFile(res.Path)
	if err != nil {
		t.Fatal(err)
	}
	var content []byte
	for _, m := range regexp.MustCompile(`(?s)stream\r?\n(.*?)\r?\nendstream`).FindAllSubmatch(b, -1) {
		if z, err := zlib.NewReader(bytes.NewReader(m[1])); err == nil {
			if s, err := io.ReadAll(z); err == nil {
				content = append(content, s...)
			}
		}
	}
	rgb := func(hex string) string {
		v, _ := strconv.ParseUint(hex, 16, 32)
		f := func(c uint64) string { return strconv.FormatFloat(float64(c)/255, 'f', -1, 32) }
		return f(v>>16) + " " + f(v>>8&0xff) + " " + f(v&0xff) + " "
	}
	for name, op := range map[string]string{
		"rule stroke":   rgb("b3a28a") + "SCN",
		"text-ink fill": rgb("1e1e1e") + "scn",
	} {
		if !bytes.Contains(content, []byte(op)) {
			t.Errorf("no %s (%q) in the page", name, op)
		}
	}
}
