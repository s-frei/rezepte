package recipeimport_test

import (
	"encoding/json"
	"flag"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/s-frei/rezepte/service/internal/recipeimport"
)

var update = flag.Bool("update", false, "rewrite golden files")

var sourceComment = regexp.MustCompile(`^<!-- source: (\S+) -->`)

func TestReadPageGolden(t *testing.T) {
	files, _ := filepath.Glob("testdata/pages/*.html")
	if len(files) < 8 {
		t.Fatalf("expected at least 8 fixtures, found %d", len(files))
	}
	for _, f := range files {
		t.Run(filepath.Base(f), func(t *testing.T) {
			body, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			// Real fixtures name their page, so canonical and og:url resolve
			// against the address they came from; synthetic ones get a stand-in.
			raw := "https://example.com/recipe"
			if m := sourceComment.FindSubmatch(body); m != nil {
				raw = string(m[1])
			}
			base, _ := url.Parse(raw)
			page, ok := recipeimport.ReadPage(body, base)
			got, _ := json.MarshalIndent(struct {
				Found bool
				Page  recipeimport.Page
			}{ok, page}, "", "  ")
			golden := strings.TrimSuffix(f, ".html") + ".json"
			if *update {
				if err := os.WriteFile(golden, append(got, '\n'), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("%v (run with -update)", err)
			}
			if strings.TrimSpace(string(want)) != strings.TrimSpace(string(got)) {
				t.Fatalf("mismatch with %s:\n%s", golden, got)
			}
		})
	}
}

func TestReadPageGraphAndTypeList(t *testing.T) {
	body, _ := os.ReadFile("testdata/pages/graph-typelist.html")
	base, _ := url.Parse("https://example.com/graph-soup?utm_source=x")
	p, ok := recipeimport.ReadPage(body, base)
	if !ok || p.Title != "Graph Soup" || p.Servings != 4 || p.CookMinutes == nil || *p.CookMinutes != 45 {
		t.Fatalf("got %+v, %v", p, ok)
	}
	if p.PhotoURL != "https://example.com/img/soup.jpg" || p.SourceURL != "https://example.com/graph-soup" || p.SourceName != "Example Kitchen" {
		t.Fatalf("photo/source = %q %q %q", p.PhotoURL, p.SourceURL, p.SourceName)
	}
	if len(p.Steps) != 1 || p.Steps[0] != "Aufkochen." { // one section: its name is no news
		t.Fatalf("steps = %q", p.Steps)
	}
	if len(p.Keywords) != 2 {
		t.Fatalf("keywords = %q", p.Keywords)
	}
}

func TestReadPageBrokenJSON(t *testing.T) {
	body, _ := os.ReadFile("testdata/pages/broken-json.html")
	if _, ok := recipeimport.ReadPage(body, &url.URL{Scheme: "https", Host: "example.com"}); !ok {
		t.Fatal("repairable JSON-LD not read")
	}
}

func TestReadPageNoRecipe(t *testing.T) {
	body, _ := os.ReadFile("testdata/pages/no-recipe.html")
	if _, ok := recipeimport.ReadPage(body, &url.URL{Scheme: "https", Host: "example.com"}); ok {
		t.Fatal("found a recipe on a page without one")
	}
}

func TestReadPageInstructionStrings(t *testing.T) {
	page := func(instr string) recipeimport.Page {
		b, _ := json.Marshal(map[string]any{"@type": "Recipe", "name": "X", "recipeInstructions": instr})
		body := []byte(`<script type="application/ld+json">` + string(b) + `</script>`)
		p, ok := recipeimport.ReadPage(body, &url.URL{Scheme: "https", Host: "example.com"})
		if !ok {
			t.Fatal("no recipe")
		}
		return p
	}
	for name, tc := range map[string]struct {
		in   string
		want []string
	}{
		"lines":    {"Mix.\nBake.\n\nServe.", []string{"Mix.", "Bake.", "Serve."}},
		"numbered": {"1. Mix.\n2. Bake.\n3. Serve.", []string{"Mix.", "Bake.", "Serve."}},
		"inline":   {"1. a 2. b 3. c", []string{"a", "b", "c"}},
		"html":     {"<ul><li>Mix.</li><li>Bake.</li></ul>First<br>Second", []string{"Mix.", "Bake.", "First", "Second"}},
		"one":      {"Just stir 2 cups.", []string{"Just stir 2 cups."}},
		"decimal":  {"2.5 cups flour\nStir.", []string{"2.5 cups flour", "Stir."}},
	} {
		if got := page(tc.in).Steps; strings.Join(got, "|") != strings.Join(tc.want, "|") {
			t.Errorf("%s: steps = %q, want %q", name, got, tc.want)
		}
	}
}

func TestParseDurationLenient(t *testing.T) {
	for in, want := range map[string]int{"PT1H30M": 90, "PT45M": 45, "P0DT1H": 60, "PT0D1H": 60, "PT90M": 90, "P1D": 1440, "PT30S": 0, "pt1h": 60, " PT5M ": 5, "": -1, "P": -1, "PT": -1, "45 Minuten": -1} {
		got := recipeimport.ParseDuration(in)
		if (got == nil && want != -1) || (got != nil && *got != want) {
			t.Errorf("ParseDuration(%q) = %v, want %d", in, got, want)
		}
	}
}

func TestReadPageCleansMarkup(t *testing.T) {
	body := []byte(`<script type="application/ld+json">{"@type":"Recipe","name":"<b>Ba</b>con &amp; <i>Eggs</i>","description":"One<br>two<p>three</p><div>four</div><ul><li>five</li><li>six</li></ul>"}</script>`)
	p, ok := recipeimport.ReadPage(body, &url.URL{Scheme: "https", Host: "example.com"})
	if !ok || p.Title != "Bacon & Eggs" || p.Description != "One two three four five six" {
		t.Fatalf("got %q / %q, %v", p.Title, p.Description, ok)
	}
}

func TestReadPageRepairsControlCharacters(t *testing.T) {
	body := []byte("<script type=\"application/ld+json\">{\"@type\":\"Recipe\",\"name\":\"So\x01up\x7f\",\"description\":\"A\x0bB\tC\"}</script>")
	p, ok := recipeimport.ReadPage(body, &url.URL{Scheme: "https", Host: "example.com"})
	if !ok || p.Title != "Soup" || p.Description != "A B C" {
		t.Fatalf("got %q / %q, %v", p.Title, p.Description, ok)
	}
}
