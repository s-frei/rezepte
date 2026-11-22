package recipe_test

import (
	"context"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/s-frei/rezepte/service/internal/ingredient"
	"github.com/s-frei/rezepte/service/internal/recipe"
)

var pantryChips = []string{"Kartoffel", "Zwiebel", "Speck"}

func listPantry(t *testing.T, svc *recipe.Service, p recipe.ListParams) recipe.Page {
	t.Helper()
	if p.Limit == 0 {
		p.Limit = 100
	}
	page, err := svc.List(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	return page
}

func cardByTitle(t *testing.T, page recipe.Page, title string) recipe.Card {
	t.Helper()
	for _, c := range page.Items {
		if c.Title == title {
			return c
		}
	}
	t.Fatalf("%s not in %v", title, recipeIDs(page.Items))
	return recipe.Card{}
}

func missingKeys(c recipe.Card) []string {
	var keys []string
	for _, m := range c.Pantry.Missing {
		keys = append(keys, m.Key)
	}
	return keys
}

func TestListPantryRanksByChipsThenMissing(t *testing.T) {
	svc := seedAll(t)
	p := listPantry(t, svc, recipe.ListParams{Ingredients: pantryChips})
	if len(p.Items) == 0 || p.Items[0].Title != "Linseneintopf" || p.Items[0].Pantry.Chips != 3 {
		t.Fatalf("first = %+v", p.Items[0])
	}
	prev := -1
	for _, c := range p.Items[1:] {
		if c.Pantry.Chips > 2 {
			t.Fatalf("%s: chips %d after the first", c.Title, c.Pantry.Chips)
		}
		if c.Pantry.Chips == 2 {
			if n := len(c.Pantry.Missing); n < prev {
				t.Fatalf("%s: missing %d after %d", c.Title, n, prev)
			} else {
				prev = n
			}
		}
	}
	if p.Pantry == nil || p.Pantry.Sections[3] != 1 {
		t.Fatalf("pantry = %+v", p.Pantry)
	}
	sum := 0
	for k, n := range p.Pantry.Sections {
		if k < 1 || k > 3 {
			t.Fatalf("section key %d", k)
		}
		sum += n
	}
	if sum != p.Total {
		t.Fatalf("sections %v sum %d, total %d", p.Pantry.Sections, sum, p.Total)
	}
	if p.Sections != nil {
		t.Fatalf("text sections in pantry mode: %v", p.Sections)
	}
}

func TestListPantryHaveLowersMissingNotChips(t *testing.T) {
	svc := seedAll(t)
	without := listPantry(t, svc, recipe.ListParams{Ingredients: pantryChips})
	have := []string{"Salz", "Pfeffer", "Öl"}
	with := listPantry(t, svc, recipe.ListParams{Ingredients: pantryChips, Have: have})
	if len(with.Items) != len(without.Items) {
		t.Fatalf("items %d vs %d", len(with.Items), len(without.Items))
	}
	buckets := func(p recipe.Page) map[int][]string {
		m := map[int][]string{}
		for _, c := range p.Items {
			m[c.Pantry.Chips] = append(m[c.Pantry.Chips], c.ID)
		}
		for k := range m {
			slices.Sort(m[k])
		}
		return m
	}
	for i := range with.Items {
		if with.Items[i].Pantry.Chips != without.Items[i].Pantry.Chips {
			t.Fatalf("chips order changed at %d", i)
		}
	}
	a, b := buckets(with), buckets(without)
	for k := range a {
		if !slices.Equal(a[k], b[k]) {
			t.Fatalf("chips %d: %v vs %v", k, a[k], b[k])
		}
	}
	// Within a chip count the ticks rank: fewest missing after them first.
	prev := map[int]int{}
	for _, c := range with.Items {
		n := 0
		for _, m := range c.Pantry.Missing {
			if !slices.ContainsFunc(have, func(h string) bool { return ingredient.Covers(ingredient.Key(h), m.Key) }) {
				n++
			}
		}
		if n < prev[c.Pantry.Chips] {
			t.Fatalf("%s: %d missing after the ticks, after %d", c.Title, n, prev[c.Pantry.Chips])
		}
		prev[c.Pantry.Chips] = n
	}
	// Flammkuchen misses 5 to Kartoffelsalat's 6; ticking broth and
	// vinegar leaves Kartoffelsalat 4, so the pair turns around.
	before := func(p recipe.Page, a, b string) bool {
		return slices.IndexFunc(p.Items, func(c recipe.Card) bool { return c.Title == a }) <
			slices.IndexFunc(p.Items, func(c recipe.Card) bool { return c.Title == b })
	}
	turned := listPantry(t, svc, recipe.ListParams{Ingredients: pantryChips, Have: []string{"Gemüsebrühe", "Essig"}})
	if !before(without, "Flammkuchen", "Kartoffelsalat") || !before(turned, "Kartoffelsalat", "Flammkuchen") {
		t.Fatalf("ticks did not reorder: without %v, with %v", titles(without), titles(turned))
	}
}

func titles(p recipe.Page) []string {
	var out []string
	for _, c := range p.Items {
		out = append(out, c.Title)
	}
	return out
}

func TestListPantryWaterNeverMissing(t *testing.T) {
	svc := seedAll(t)
	c := cardByTitle(t, listPantry(t, svc, recipe.ListParams{Ingredients: pantryChips}), "Käsespätzle")
	if slices.Contains(missingKeys(c), "wasser") {
		t.Fatalf("missing = %v", c.Pantry.Missing)
	}
}

func TestListPantryDedupesByKey(t *testing.T) {
	svc := seedAll(t)
	c := cardByTitle(t, listPantry(t, svc, recipe.ListParams{Ingredients: pantryChips}), "Königsberger Klopse")
	n := 0
	for _, k := range missingKeys(c) {
		if k == "salz" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("salz %d times in %v", n, c.Pantry.Missing)
	}
}

func TestListPantryOnlyRecipesWithAChip(t *testing.T) {
	svc := seedAll(t)
	p := listPantry(t, svc, recipe.ListParams{Ingredients: pantryChips})
	for _, c := range p.Items {
		if c.Pantry == nil || c.Pantry.Chips < 1 {
			t.Fatalf("%s: %+v", c.Title, c.Pantry)
		}
	}
	// Every sample but the dessert has an onion.
	if p.Total != len(p.Items) || p.Total != 11 {
		t.Fatalf("total %d, items %d", p.Total, len(p.Items))
	}
}

func TestListPantryTextFilters(t *testing.T) {
	svc := seedAll(t)
	text := listPantry(t, svc, recipe.ListParams{Query: "salat"})
	p := listPantry(t, svc, recipe.ListParams{Query: "salat", Ingredients: pantryChips})
	if p.Total == 0 {
		t.Fatal("no results")
	}
	// The chips reuse the ingredient names they ranked by for the finds,
	// which must come out as the text search alone finds them.
	byID := map[string]recipe.Card{}
	for _, c := range text.Items {
		byID[c.ID] = c
	}
	for _, c := range p.Items {
		t0, ok := byID[c.ID]
		if !ok {
			t.Fatalf("%s not found by the text alone", c.Title)
		}
		if c.Match == nil {
			t.Fatalf("%s: no match on a text search", c.Title)
		}
		if !reflect.DeepEqual(c.Match, t0.Match) {
			t.Fatalf("%s: match %+v, the text alone %+v", c.Title, c.Match, t0.Match)
		}
	}
}

func TestListPantryExplicitSortKeepsOrder(t *testing.T) {
	svc := seedAll(t)
	p := listPantry(t, svc, recipe.ListParams{Ingredients: pantryChips, Sort: "title"})
	var titles []string
	for _, c := range p.Items {
		if c.Pantry == nil {
			t.Fatalf("%s: no pantry", c.Title)
		}
		titles = append(titles, c.Title)
	}
	if !slices.IsSortedFunc(titles, func(a, b string) int { return strings.Compare(strings.ToLower(a), strings.ToLower(b)) }) {
		t.Fatalf("titles %v", titles)
	}
	if p.Pantry == nil || p.Pantry.Sections != nil {
		t.Fatalf("pantry = %+v", p.Pantry)
	}
}

func TestListPantryPaging(t *testing.T) {
	svc := seedAll(t)
	all := listPantry(t, svc, recipe.ListParams{Ingredients: pantryChips})
	var got []string
	for page := 1; ; page++ {
		p := listPantry(t, svc, recipe.ListParams{Ingredients: pantryChips, Limit: 2, Page: page})
		if len(p.Items) == 0 {
			break
		}
		if p.Total != all.Total {
			t.Fatalf("page %d total %d, want %d", page, p.Total, all.Total)
		}
		got = append(got, recipeIDs(p.Items)...)
	}
	if !slices.Equal(got, recipeIDs(all.Items)) {
		t.Fatalf("paged %v, want %v", got, recipeIDs(all.Items))
	}
	sorted := slices.Clone(got)
	slices.Sort(sorted)
	if len(slices.Compact(sorted)) != len(got) {
		t.Fatalf("duplicates in %v", got)
	}
}

func TestListPantryUnknownChip(t *testing.T) {
	svc := seedAll(t)
	p := listPantry(t, svc, recipe.ListParams{Ingredients: []string{"Drachenfrucht"}})
	if p.Total != 0 || len(p.Items) != 0 {
		t.Fatalf("page = %+v", p)
	}
}

func TestListPantryRowOrder(t *testing.T) {
	svc := seedAll(t)
	for _, have := range [][]string{nil, {"Möhre"}} {
		p := listPantry(t, svc, recipe.ListParams{Ingredients: pantryChips, Have: have})
		var want []recipe.PantryItem
		seen := map[string]bool{}
		for _, c := range p.Items {
			for _, m := range c.Pantry.Missing {
				if !seen[m.Key] {
					seen[m.Key] = true
					want = append(want, m)
				}
			}
		}
		if !slices.Equal(p.Pantry.Row, want) {
			t.Fatalf("have %v: row %v, want %v", have, p.Pantry.Row, want)
		}
		if have != nil && !slices.ContainsFunc(p.Pantry.Row, func(i recipe.PantryItem) bool { return i.Key == "möhre" }) {
			t.Fatalf("ticked Möhre left the row: %v", p.Pantry.Row)
		}
	}
}

func TestListPantryTickedEcho(t *testing.T) {
	svc := seedAll(t)
	p := listPantry(t, svc, recipe.ListParams{Ingredients: pantryChips, Have: []string{"Essig"}})
	if want := []recipe.PantryItem{{Name: "Essig", Key: "essig"}}; !slices.Equal(p.Pantry.Ticked, want) {
		t.Fatalf("ticked %v, want %v", p.Pantry.Ticked, want)
	}
}

func TestListWithoutIngredientsUnchanged(t *testing.T) {
	svc := seedAll(t)
	p := listPantry(t, svc, recipe.ListParams{Ingredients: []string{" ", ""}})
	if p.Pantry != nil || p.Total != 12 {
		t.Fatalf("page pantry = %+v, total %d", p.Pantry, p.Total)
	}
	for _, c := range p.Items {
		if c.Pantry != nil {
			t.Fatalf("%s: pantry without chips", c.Title)
		}
	}
}
