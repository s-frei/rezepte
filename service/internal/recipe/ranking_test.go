package recipe_test

import (
	"context"
	"maps"
	"slices"
	"testing"

	"github.com/s-frei/rezepte/service/internal/recipe"
)

func rankInput(title, desc string, ingredients ...string) recipe.Input {
	g := recipe.IngredientGroup{}
	for _, n := range ingredients {
		g.Ingredients = append(g.Ingredients, recipe.Ingredient{Name: n})
	}
	return recipe.Input{Title: title, Description: desc, Servings: 2,
		Tags: []string{}, IngredientGroups: []recipe.IngredientGroup{g}, Steps: []recipe.Step{}}
}

// seedRanking creates the recipes in order, so the last one is the most
// recently updated: the order the overview used for a search before it ranked.
func seedRanking(t *testing.T, ins ...recipe.Input) *recipe.Service {
	t.Helper()
	svc, uid := setup(t)
	for _, in := range ins {
		if _, err := svc.Create(context.Background(), uid, in); err != nil {
			t.Fatal(err)
		}
	}
	return svc
}

func search(t *testing.T, svc *recipe.Service, p recipe.ListParams) []string {
	t.Helper()
	page, err := svc.List(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	got := []string{}
	for _, c := range page.Items {
		got = append(got, c.Title)
	}
	if p.Limit == 0 && page.Total != len(got) {
		t.Fatalf("total %d for %d items", page.Total, len(got))
	}
	return got
}

func TestSearchRanksTitleHitsFirst(t *testing.T) {
	svc := seedRanking(t,
		rankInput("Bauernbrot", "Mit Sauerteig.", "Roggenmehl", "Sauerteig"),
		rankInput("Brotsalat", "Toskanisch.", "Tomaten", "Weißbrot"),
		rankInput("Brot", "Das Grundrezept.", "Mehl", "Hefe", "Wasser"),
		// One hit more than "Brot", whose title is the whole search.
		rankInput("Einfaches Brot", "Ein schnelles Hefebrot.", "Mehl", "Hefe"),
		rankInput("Gefüllte Paprikaschoten", "Dazu passt Brot.", "Paprikaschoten", "Hackfleisch", "Brötchen vom Vortag"),
		rankInput("Linsensuppe", "Ohne Brot kein Genuss.", "Linsen"),
		rankInput("Tomatensalat", "Frisch.", "Tomaten", "Zwiebeln"),
	)
	got := search(t, svc, recipe.ListParams{Query: "Brot"})
	want := []string{"Brot", "Einfaches Brot", "Brotsalat", "Bauernbrot", "Gefüllte Paprikaschoten", "Linsensuppe"}
	if !slices.Equal(got, want) {
		t.Fatalf("Brot → %v, want %v", got, want)
	}
}

func TestSearchFindsPartsOfWordsBelowWholeWords(t *testing.T) {
	svc := seedRanking(t,
		rankInput("Risotto", "", "Reis", "Parmesan"),
		rankInput("Wildragout", "", "Preiselbeeren", "Hirsch"),
	)
	if got, want := search(t, svc, recipe.ListParams{Query: "reis"}), []string{"Risotto", "Wildragout"}; !slices.Equal(got, want) {
		t.Fatalf("reis → %v, want %v", got, want)
	}
	// Spelled without the umlaut, and a word inside another.
	svc = seedRanking(t, rankInput("Käsespätzle", "", "Bergkäse"))
	for _, q := range []string{"kaese", "spaetzle", "bergkae", "KÄSE"} {
		if got := search(t, svc, recipe.ListParams{Query: q}); len(got) != 1 {
			t.Fatalf("%q → %v", q, got)
		}
	}
}

func TestSearchFindsShortWordsByTheirStart(t *testing.T) {
	svc := seedRanking(t,
		rankInput("Rührei", "", "Eier", "Butter"),
		rankInput("Salat", "Mit Öl.", "Feldsalat", "Öl"),
	)
	if got, want := search(t, svc, recipe.ListParams{Query: "ei"}), []string{"Rührei"}; !slices.Equal(got, want) {
		t.Fatalf("ei → %v, want %v", got, want)
	}
	if got, want := search(t, svc, recipe.ListParams{Query: "öl"}), []string{"Salat"}; !slices.Equal(got, want) {
		t.Fatalf("öl → %v, want %v", got, want)
	}
	// Every word has to be found, the short one at a word start.
	if got := search(t, svc, recipe.ListParams{Query: "salat ei"}); len(got) != 0 {
		t.Fatalf("salat ei → %v, want nothing", got)
	}
}

func TestSearchKeepsAnOrderTheReaderPicked(t *testing.T) {
	svc := seedRanking(t,
		rankInput("Zwiebelbrot", "", "Mehl"),
		rankInput("Brot", "", "Mehl"),
		rankInput("Apfelbrot", "", "Äpfel"),
	)
	if got, want := search(t, svc, recipe.ListParams{Query: "brot", Sort: "title"}), []string{"Apfelbrot", "Brot", "Zwiebelbrot"}; !slices.Equal(got, want) {
		t.Fatalf("by title → %v, want %v", got, want)
	}
	// Equal hits go to the shorter title, then the most recently updated.
	if got, want := search(t, svc, recipe.ListParams{Query: "brot"}), []string{"Brot", "Apfelbrot", "Zwiebelbrot"}; !slices.Equal(got, want) {
		t.Fatalf("by relevance → %v, want %v", got, want)
	}
}

// Every word scores where it is found, and the scores add up: the bread
// with butter among its ingredients beats a description naming both.
func TestSearchRanksEachWordWhereItIsFound(t *testing.T) {
	svc := seedRanking(t,
		rankInput("Brot", "Das Grundrezept.", "Mehl", "Butter"),
		rankInput("Gefüllte Paprikaschoten", "Dazu Brot und Butter.", "Paprika"),
	)
	if got, want := search(t, svc, recipe.ListParams{Query: "brot butter"}), []string{"Brot", "Gefüllte Paprikaschoten"}; !slices.Equal(got, want) {
		t.Fatalf("brot butter → %v, want %v", got, want)
	}
	// A short word does not take the part-of-word score from the other.
	svc = seedRanking(t,
		rankInput("Bauernbrot", "Mit Ei.", "Mehl"),
		rankInput("Rührkuchen", "Mit Ei und Brot.", "Mehl"),
	)
	if got, want := search(t, svc, recipe.ListParams{Query: "ei brot"}), []string{"Bauernbrot", "Rührkuchen"}; !slices.Equal(got, want) {
		t.Fatalf("ei brot → %v, want %v", got, want)
	}
}

func TestSearchFindsSpellingsAndPlurals(t *testing.T) {
	svc := seedRanking(t,
		rankInput("Salat", "", "Feldsalat", "Olivenöl"),
		rankInput("Tomatensuppe", "", "Tomate", "Zwiebel"),
		rankInput("Süsskartoffel-Pommes", "", "Süßkartoffel"),
		rankInput("Apfelkuchen", "", "Apfel", "Mehl"),
	)
	for q, want := range map[string][]string{
		"oel":            {"Salat"},
		"öl":             {"Salat"},
		"tomaten":        {"Tomatensuppe"},
		"zwiebeln":       {"Tomatensuppe"},
		"suesskartoffel": {"Süsskartoffel-Pommes"},
		"süßkartoffel":   {"Süsskartoffel-Pommes"},
		"Äpfel":          {"Apfelkuchen"},
		"Brot!":          {},
	} {
		if got := search(t, svc, recipe.ListParams{Query: q}); !slices.Equal(got, want) {
			t.Fatalf("%q → %v, want %v", q, got, want)
		}
	}
	// Punctuation does not cost the exact title its lead.
	svc = seedRanking(t,
		rankInput("Brot", "", "Mehl"),
		rankInput("Einfaches Brot", "Ein Hefebrot.", "Mehl"),
	)
	if got := search(t, svc, recipe.ListParams{Query: "Brot!"}); got[0] != "Brot" {
		t.Fatalf("Brot! → %v", got)
	}
	svc = seedRanking(t,
		rankInput("Spätzle Käse", "", "Mehl"),
		rankInput("Käse-Spätzle", "", "Mehl"),
		rankInput("Spätzle mit Käse", "", "Mehl"),
	)
	if got := search(t, svc, recipe.ListParams{Query: "Käse-Spätzle"}); got[0] != "Käse-Spätzle" {
		t.Fatalf("Käse-Spätzle → %v", got)
	}
}

// A search comes grouped by where its strongest find is - title, tags,
// ingredients, description - page after page, and each card says so.
func TestSearchGroupsBySection(t *testing.T) {
	tagged := rankInput("Obatzda", "", "Camembert")
	tagged.Tags = []string{"brotzeit"}
	svc := seedRanking(t,
		rankInput("Suppe", "Mit Brot.", "Wasser"),
		rankInput("Frikadellen", "", "Brötchen"),
		tagged,
		rankInput("Bauernbrot", "", "Mehl"),
	)
	var got []string
	for page := 1; page <= 4; page++ {
		p, err := svc.List(context.Background(), recipe.ListParams{Query: "brot", Page: page, Limit: 1})
		if err != nil {
			t.Fatal(err)
		}
		c := p.Items[0]
		if c.Match == nil {
			t.Fatalf("%s: no match", c.Title)
		}
		got = append(got, c.Title+":"+c.Match.Section)
	}
	want := []string{"Bauernbrot:title", "Obatzda:tags", "Frikadellen:ingredients", "Suppe:description"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	p, err := svc.List(context.Background(), recipe.ListParams{Query: "brot", Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if want := map[string]int{"title": 1, "tags": 1, "ingredients": 1, "description": 1}; !maps.Equal(p.Sections, want) {
		t.Fatalf("sections %v, want %v", p.Sections, want)
	}
	// Another order is not grouped, so it carries no sections either: a
	// client groups by the list it shows, not by the order it asks for next.
	p, err = svc.List(context.Background(), recipe.ListParams{Query: "brot", Sort: "title", Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if p.Sections != nil {
		t.Fatalf("sections in A-Z order: %v", p.Sections)
	}
	// Without a search there is nothing to match or count.
	p, _ = svc.List(context.Background(), recipe.ListParams{})
	if p.Sections != nil {
		t.Fatalf("sections without a search: %v", p.Sections)
	}
	for _, c := range p.Items {
		if c.Match != nil {
			t.Fatalf("%s: match without a search", c.Title)
		}
	}
}
