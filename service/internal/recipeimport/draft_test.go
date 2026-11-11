package recipeimport_test

import (
	"encoding/json"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/s-frei/rezepte/service/internal/recipeimport"
	"github.com/s-frei/rezepte/service/internal/transfer"
)

func TestBuildSplitsTagsAndFlagsLines(t *testing.T) {
	d := recipeimport.Build(recipeimport.Page{
		Title:      "Käsespätzle",
		Groups:     []recipeimport.PageGroup{{Lines: []string{"400 g Mehl", "Salz und Pfeffer n.B."}}},
		Steps:      []string{"Verrühren."},
		Keywords:   []string{"Vegetarisch", "Party", "nudeln"},
		SourceURL:  "https://www.chefkoch.de/rezepte/1/kaesespaetzle.html",
		SourceName: "Chefkoch",
		PhotoURL:   "https://img.chefkoch.de/a.jpg",
	}, []string{"vegetarisch", "Nudeln", "Kuchen"})
	if got := d.Recipe.Tags; len(got) != 2 || got[0] != "vegetarisch" || got[1] != "Nudeln" {
		t.Fatalf("tags = %q", got)
	}
	if len(d.SuggestedTags) != 1 || d.SuggestedTags[0] != "Party" {
		t.Fatalf("suggested = %q", d.SuggestedTags)
	}
	if len(d.Review) != 1 || d.Review[0] != (recipeimport.DraftReview{Group: 0, Ingredient: 1, Line: "Salz und Pfeffer n.B."}) {
		t.Fatalf("review = %+v", d.Review)
	}
	if d.Recipe.Servings != 4 || *d.Recipe.SourceName != "Chefkoch" || d.PhotoURL == "" || d.Truncated {
		t.Fatalf("draft = %+v", d)
	}
}

func TestBuildClampsAndStillValidates(t *testing.T) {
	long := strings.Repeat("x ", 3000)
	var many []string
	for range 130 {
		many = append(many, "1 g Salz")
	}
	d := recipeimport.Build(recipeimport.Page{Title: long, Description: long, Groups: []recipeimport.PageGroup{{Lines: many}}, Steps: []string{long}}, nil)
	if !d.Truncated {
		t.Fatal("not marked truncated")
	}
	_, api := humatest.New(t)
	validate := transfer.InputValidator(api)
	raw, _ := json.Marshal(d.Recipe)
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	delete(m, "editPolicy")
	if errs := validate(m); len(errs) > 0 {
		t.Fatalf("clamped draft invalid: %s %s", errs[0].Location, errs[0].Message)
	}
}

func TestBuildWithoutTitleOpensAnyway(t *testing.T) {
	d := recipeimport.Build(recipeimport.Page{Steps: []string{"Kochen."}}, nil)
	if d.Recipe.Title != "" || len(d.Recipe.IngredientGroups) != 1 || len(d.Recipe.Steps) != 1 {
		t.Fatalf("draft = %+v", d.Recipe)
	}
}

func TestBuildCutsAtRunesNotBytes(t *testing.T) {
	// The only space sits at rune 80 (byte 160) of a 200-rune cut: too early
	// for a word boundary, so the cut is hard at 200 runes.
	title := strings.Repeat("ä", 80) + " " + strings.Repeat("b", 200)
	d := recipeimport.Build(recipeimport.Page{Title: title, Steps: []string{"x"}}, nil)
	if n := len([]rune(d.Recipe.Title)); n != 200 {
		t.Fatalf("title has %d runes, want 200", n)
	}
}

func TestBuildClampsTagsAndSource(t *testing.T) {
	var known []string
	for i := range 25 {
		known = append(known, "tag"+strconv.Itoa(i))
	}
	d := recipeimport.Build(recipeimport.Page{Title: "x", Keywords: known, Steps: []string{"x"}}, known)
	if len(d.Recipe.Tags) != 20 || !d.Truncated {
		t.Fatalf("tags = %d, truncated = %v", len(d.Recipe.Tags), d.Truncated)
	}
	d = recipeimport.Build(recipeimport.Page{Title: "x", SourceURL: "https://example.com/" + strings.Repeat("a", 500), Steps: []string{"x"}}, nil)
	if d.Recipe.SourceURL != nil || !d.Truncated {
		t.Fatalf("source = %v, truncated = %v", d.Recipe.SourceURL, d.Truncated)
	}
}

func TestBuildKeepsAuthorsOutOfTags(t *testing.T) {
	d := recipeimport.Build(recipeimport.Page{
		Title:    "Pancakes",
		Authors:  []string{"Cassie Best", "Ann Lee"},
		Keywords: []string{"Party", " cassie best ", "ANN LEE", "Brunch"},
	}, []string{"Cassie Best", "Brunch"})
	if got := d.Recipe.Tags; len(got) != 1 || got[0] != "Brunch" {
		t.Fatalf("tags = %q", got)
	}
	if len(d.SuggestedTags) != 1 || d.SuggestedTags[0] != "Party" {
		t.Fatalf("suggested = %q", d.SuggestedTags)
	}
}

func TestBuildDroppingSuggestionsIsNotTruncation(t *testing.T) {
	kws := []string{strings.Repeat("k", 41)}
	for i := range 25 {
		kws = append(kws, "kw"+strconv.Itoa(i))
	}
	d := recipeimport.Build(recipeimport.Page{Title: "x", Keywords: kws, Steps: []string{"x"}}, nil)
	if len(d.SuggestedTags) != 20 || d.Truncated {
		t.Fatalf("suggested = %d, truncated = %v", len(d.SuggestedTags), d.Truncated)
	}
}

func TestBuildSendsEmptyStepLists(t *testing.T) {
	// The editor reads all three lists of every step; a null would break it.
	d := recipeimport.Build(recipeimport.Page{Title: "x", Steps: []string{"20 Minuten kochen."}}, nil)
	raw, err := json.Marshal(d.Recipe.Steps)
	if err != nil {
		t.Fatal(err)
	}
	if want := `[{"text":"20 Minuten kochen.","references":[],"times":[],"photos":[]}]`; string(raw) != want {
		t.Errorf("steps = %s, want %s", raw, want)
	}
}

func TestBuildClampsGroups(t *testing.T) {
	var groups []recipeimport.PageGroup
	for i := range 21 {
		groups = append(groups, recipeimport.PageGroup{Name: "Teil " + strconv.Itoa(i), Lines: []string{"1 g Salz", "etwas Öl"}})
	}
	d := recipeimport.Build(recipeimport.Page{Title: "x", Groups: groups, Steps: []string{"x"}}, nil)
	gs := d.Recipe.IngredientGroups
	if len(gs) != 20 || len(gs[19].Ingredients) != 4 || !d.Truncated {
		t.Fatalf("%d groups, last has %d, truncated = %v", len(gs), len(gs[len(gs)-1].Ingredients), d.Truncated)
	}
	// The 21st group's unsure line points into the 20th, where it now is.
	if last := d.Review[len(d.Review)-1]; last != (recipeimport.DraftReview{Group: 19, Ingredient: 3, Line: "etwas Öl"}) {
		t.Fatalf("last review = %+v", last)
	}
}

func TestBuildExplicitTags(t *testing.T) {
	d := recipeimport.Build(recipeimport.Page{
		Title: "x", Steps: []string{"x"},
		Tags: []string{"vegetarisch", "#neu", strings.Repeat("t", 41), "Neu", "#vegetarisch"},
		// A tag twice, in any case, is one: the editor keys tags by their text.
		Keywords: []string{"Vegetarisch", "Sommer", "sommer"},
	}, []string{"Vegetarisch"})
	if got := d.Recipe.Tags; !slices.Equal(got, []string{"Vegetarisch", "neu"}) || !slices.Equal(d.SuggestedTags, []string{"Sommer"}) || !d.Truncated {
		t.Fatalf("tags = %q, suggested = %q, truncated = %v", got, d.SuggestedTags, d.Truncated)
	}
}

func TestBuildReviewsPerGroup(t *testing.T) {
	d := recipeimport.Build(recipeimport.Page{Title: "x", Language: "de", Groups: []recipeimport.PageGroup{
		{Name: "Teig", Lines: []string{"200 g Mehl"}},
		{Name: "Guss", Lines: []string{"100 g Zucker", "etwas Zitronensaft"}},
	}}, nil)
	if want := []recipeimport.DraftReview{{Group: 1, Ingredient: 1, Line: "etwas Zitronensaft"}}; !slices.Equal(d.Review, want) {
		t.Fatalf("review = %+v", d.Review)
	}
	if g := d.Recipe.IngredientGroups[1]; g.Name == nil || *g.Name != "Guss" {
		t.Fatalf("group 1 = %+v", g)
	}
}
