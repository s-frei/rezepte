package recipeimport_test

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/s-frei/rezepte/service/internal/recipeimport"
)

func TestReadTextWithHeadings(t *testing.T) {
	p, ok := recipeimport.ReadText(`Käsespätzle
Ein Klassiker aus dem Allgäu.

Zutaten:
400 g Mehl
4 Eier
200 g Bergkäse

Zubereitung
1. Mehl und Eier verrühren.
Kräftig schlagen.
2. Spätzle ins kochende Wasser schaben.`)
	if !ok {
		t.Fatal("no recipe")
	}
	if p.Title != "Käsespätzle" || p.Description != "Ein Klassiker aus dem Allgäu." {
		t.Fatalf("title/description = %q / %q", p.Title, p.Description)
	}
	if want := []string{"400 g Mehl", "4 Eier", "200 g Bergkäse"}; !slices.Equal(lines(p), want) {
		t.Fatalf("ingredients = %q", lines(p))
	}
	if want := []string{"Mehl und Eier verrühren. Kräftig schlagen.", "Spätzle ins kochende Wasser schaben."}; !slices.Equal(p.Steps, want) {
		t.Fatalf("steps = %q", p.Steps)
	}
}

func TestReadTextEnglishParagraphSteps(t *testing.T) {
	p, ok := recipeimport.ReadText("Pancakes\n\nIngredients\n2 cups flour\n2 eggs\n\nMethod\nWhisk everything.\n\nFry in butter.")
	if !ok || p.Title != "Pancakes" || len(lines(p)) != 2 || len(p.Steps) != 2 {
		t.Fatalf("got %+v, %v", p, ok)
	}
}

func TestReadTextWithoutHeadings(t *testing.T) {
	p, ok := recipeimport.ReadText("Guacamole\n2 Avocados\n1 Limette\n½ TL Salz\n\nAlles zerdrücken und abschmecken.")
	if !ok || p.Title != "Guacamole" || len(lines(p)) != 3 || len(p.Steps) != 1 {
		t.Fatalf("got %+v, %v", p, ok)
	}
}

func TestReadTextBareIngredientList(t *testing.T) {
	p, ok := recipeimport.ReadText("Einkauf\n200 g Mehl\n3 Eier")
	if !ok || p.Title != "Einkauf" || len(lines(p)) != 2 || len(p.Steps) != 0 {
		t.Fatalf("got %+v, %v", p, ok)
	}
}

func TestReadTextStepKeepsLeadingDecimal(t *testing.T) {
	p, ok := recipeimport.ReadText("Suppe\n\nZubereitung\n1. Topf holen.\n1.5 l Wasser aufkochen.\n\n2) Salzen.")
	if !ok {
		t.Fatal("no recipe")
	}
	if want := []string{"Topf holen. 1.5 l Wasser aufkochen.", "Salzen."}; !slices.Equal(p.Steps, want) {
		t.Fatalf("steps = %q", p.Steps)
	}
}

func TestReadTextNoRecipe(t *testing.T) {
	for _, s := range []string{"", "   ", "Hallo, wie geht's?", "Nur eine Zeile"} {
		if _, ok := recipeimport.ReadText(s); ok {
			t.Errorf("ReadText(%q) found a recipe", s)
		}
	}
}

func TestReadTextLeadingNumberBeforeHeading(t *testing.T) {
	p, ok := recipeimport.ReadText("Pfannkuchen\n4 Portionen\n2 people love this\n\nZutaten\n2 Eier\n\nZubereitung\nBacken.")
	if !ok || p.Description != "4 Portionen 2 people love this" || !slices.Equal(lines(p), []string{"2 Eier"}) {
		t.Fatalf("got %+v, %v", p, ok)
	}
}

func TestReadTextHeadingEndsStep(t *testing.T) {
	p, ok := recipeimport.ReadText("Kuchen\n\nZubereitung\nTeig rühren.\nZutaten\n1 Ei\nZubereitung\nBacken.")
	if !ok {
		t.Fatal("no recipe")
	}
	if want := []string{"Teig rühren.", "Backen."}; !slices.Equal(p.Steps, want) {
		t.Fatalf("steps = %q", p.Steps)
	}
	if !slices.Equal(lines(p), []string{"1 Ei"}) {
		t.Fatalf("ingredients = %q", lines(p))
	}
}

func TestReadTextHeadingVariants(t *testing.T) {
	for _, text := range []string{
		"Suppe\nZutaten für 4 Personen\n1 l Wasser\n2 Karotten\nZubereitung\nKochen.",
		"Soup\nYou'll need:\n1 l water\n2 carrots\nMethod\nCook.",
		"Soup\nYoull need\n1 l water\n2 carrots\nInstructions\nCook.",
	} {
		p, ok := recipeimport.ReadText(text)
		if !ok || len(lines(p)) != 2 || len(p.Steps) != 1 {
			t.Errorf("ReadText(%q) = %+v, %v", text, p, ok)
		}
	}
}

// Without headings or a blank line, the first numbered line ends the
// ingredients; "1.5 l" is an amount, not a step number.
func TestReadTextNumberedStepsAfterGuessedIngredients(t *testing.T) {
	for _, tc := range []struct {
		text        string
		ingredients []string
		steps       []string
	}{
		{"Pfannkuchen\n200 g Mehl\n2 Eier\n1.5 l Milch\n1. Alles verrühren.\n2. Braten.", []string{"200 g Mehl", "2 Eier", "1.5 l Milch"}, []string{"Alles verrühren.", "Braten."}},
		{"Pancakes\n2 cups flour\n2 eggs\n1) Whisk.\n2) Fry.", []string{"2 cups flour", "2 eggs"}, []string{"Whisk.", "Fry."}},
	} {
		p, ok := recipeimport.ReadText(tc.text)
		if !ok || !slices.Equal(lines(p), tc.ingredients) || !slices.Equal(p.Steps, tc.steps) {
			t.Errorf("ReadText(%q) = %q / %q, %v", tc.text, lines(p), p.Steps, ok)
		}
	}
}

// A text that starts with a heading has no title; the editor asks for one.
func TestReadTextStartingWithHeading(t *testing.T) {
	for _, text := range []string{
		"Zutaten\n200 g Mehl\n2 Eier\nZubereitung\nBacken.",
		"Ingredients:\n2 cups flour\n2 eggs\nMethod\nBake.",
	} {
		p, ok := recipeimport.ReadText(text)
		if !ok || p.Title != "" || len(lines(p)) != 2 || len(p.Steps) != 1 {
			t.Errorf("ReadText(%q) = %+v, %v", text, p, ok)
		}
	}
}

// groupShape is a group's name and line count, for comparing groups briefly.
type groupShape struct {
	name  string
	lines int
}

func shapes(gs []recipeimport.PageGroup) []groupShape {
	var out []groupShape
	for _, g := range gs {
		out = append(out, groupShape{g.Name, len(g.Lines)})
	}
	return out
}

// lines flattens a page's groups, so the cases written before groups read
// as they did.
func lines(p recipeimport.Page) []string {
	var out []string
	for _, g := range p.Groups {
		out = append(out, g.Lines...)
	}
	return out
}

func TestReadTextDetails(t *testing.T) {
	for _, tc := range []struct {
		text, title, desc, source string
		tags                      []string
		groups                    []groupShape
	}{
		{`Linsen-Dal
Cremiges Dal mit roten Linsen und Kokosmilch.

Portionen: 4
Vorbereitung: 15 Min.
Kochzeit: 30 Min.
Tags: vegetarisch, indisch
Quelle: Omas Kochbuch

Zutaten
Für das Dal:
250 g rote Linsen
1 Dose Kokosmilch, 400 ml
1 Zwiebel, fein gewürfelt
Zum Servieren:
1 Bund Koriander
Salz, nach Geschmack

Zubereitung
1. Zwiebel in Öl glasig dünsten.
2. Linsen und Kokosmilch zugeben und 20 Minuten köcheln.
3. Abschmecken und mit Koriander servieren.`,
			"Linsen-Dal", "Cremiges Dal mit roten Linsen und Kokosmilch.", "Omas Kochbuch",
			[]string{"vegetarisch", "indisch"}, []groupShape{{"Für das Dal", 3}, {"Zum Servieren", 2}}},
		{`Lentil Dal
Creamy dal with red lentils and coconut milk.

Servings: 4
Prep time: 15 min
Cook time: 30 min
Tags: vegetarian, indian
Source: Grandma's cookbook

Ingredients
For the dal:
250 g red lentils
1 can coconut milk, 400 ml
1 onion, finely diced
To serve:
1 bunch coriander
salt, to taste

Method
1. Soften the onion in oil.
2. Add the lentils and coconut milk and simmer for 20 minutes.
3. Season and serve with coriander.`,
			"Lentil Dal", "Creamy dal with red lentils and coconut milk.", "Grandma's cookbook",
			[]string{"vegetarian", "indian"}, []groupShape{{"For the dal", 3}, {"To serve", 2}}},
	} {
		p, ok := recipeimport.ReadText(tc.text)
		if !ok || p.Title != tc.title || p.Description != tc.desc || p.SourceName != tc.source || p.SourceURL != "" {
			t.Errorf("%s: title/description/source = %q / %q / %q %q, %v", tc.title, p.Title, p.Description, p.SourceName, p.SourceURL, ok)
		}
		if p.Servings != 4 || p.PrepMinutes == nil || *p.PrepMinutes != 15 || p.CookMinutes == nil || *p.CookMinutes != 30 {
			t.Errorf("%s: servings/prep/cook = %d / %v / %v", tc.title, p.Servings, p.PrepMinutes, p.CookMinutes)
		}
		if !slices.Equal(p.Tags, tc.tags) || !slices.Equal(shapes(p.Groups), tc.groups) || len(p.Steps) != 3 {
			t.Errorf("%s: tags %q, groups %v, steps %q", tc.title, p.Tags, shapes(p.Groups), p.Steps)
		}
		if d := recipeimport.Build(p, nil); len(d.Review) != 0 || len(d.Recipe.IngredientGroups) != 2 {
			t.Errorf("%s: reviews %+v, %d groups", tc.title, d.Review, len(d.Recipe.IngredientGroups))
		}
	}
}

// Structure words come from every table; only the ingredient lines pick the
// table their units are read with.
func TestReadTextMixedLanguage(t *testing.T) {
	for _, tc := range []struct{ text, unit string }{
		{"Pancakes\nPortionen: 2\nZutaten\nFür den Teig:\n2 cups flour\n2 eggs\nZubereitung\n1. Whisk.\n2. Fry.", "cups"},
		{"Pfannkuchen\nServings: 2\nIngredients\nFor the batter:\n2 EL Mehl\n2 Eier\nMethod\n1. Verquirlen.\n2. Braten.", "EL"},
	} {
		p, ok := recipeimport.ReadText(tc.text)
		if !ok || p.Servings != 2 || len(p.Groups) != 1 || p.Groups[0].Name == "" || len(p.Steps) != 2 {
			t.Fatalf("ReadText(%q) = %+v, %v", tc.text, p, ok)
		}
		d := recipeimport.Build(p, nil)
		if u := d.Recipe.IngredientGroups[0].Ingredients[0].Unit; u == nil || *u != tc.unit || len(d.Review) != 0 {
			t.Errorf("%q: unit %v, reviews %+v", tc.text, u, d.Review)
		}
	}
}

func TestReadDuration(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want int
	}{
		{"15 Min.", 15},
		{"1 Std. 30 Min.", 90},
		{"1,5 h", 90},
		{"90 minutes", 90},
		{"30 Min. (davon 20 Min. Backzeit)", 30},
		{"2000 Min.", 1440},
		{"99999999999999999999 Min.", 1440},
		{"1 hr 15 mins", 75},
		{"20 Min. 10 Min.", 20},
	} {
		if got, ok := recipeimport.ReadDuration(tc.in, recipeimport.AnyLanguage); !ok || got != tc.want {
			t.Errorf("ReadDuration(%q) = %d, %v; want %d", tc.in, got, ok, tc.want)
		}
	}
	for _, in := range []string{"", "lange", "ca. 20 Min."} {
		if got, ok := recipeimport.ReadDuration(in, recipeimport.AnyLanguage); ok {
			t.Errorf("ReadDuration(%q) = %d, want none", in, got)
		}
	}
}

func TestReadTextServings(t *testing.T) {
	for _, tc := range []struct {
		line string
		want int
	}{
		{"Portionen: 4", 4},
		{"Portionen: 4 Personen", 4},
		{"Servings : 4-6", 4},
		{"Portionen: 0", 1},
		{"Portionen: 120", 99},
		{"", 0},
	} {
		p, ok := recipeimport.ReadText("Suppe\n" + tc.line + "\nZutaten\n1 l Wasser\nZubereitung\nKochen.")
		if !ok || p.Servings != tc.want {
			t.Errorf("%q: servings %d, want %d", tc.line, p.Servings, tc.want)
		}
	}
	p, _ := recipeimport.ReadText("Suppe\nZutaten\n1 l Wasser\nZubereitung\nKochen.")
	if d := recipeimport.Build(p, nil); d.Recipe.Servings != 4 {
		t.Errorf("default servings = %d, want 4", d.Recipe.Servings)
	}
}

func TestReadTextSource(t *testing.T) {
	for _, tc := range []struct{ line, url, name string }{
		{"Quelle: Chefkoch https://www.chefkoch.de/r/1", "https://www.chefkoch.de/r/1", "Chefkoch"},
		{"Quelle: Omas Kochbuch", "", "Omas Kochbuch"},
		{"Source: https://example.com/dal", "https://example.com/dal", ""},
	} {
		p, ok := recipeimport.ReadText("Suppe\n" + tc.line + "\nZutaten\n1 l Wasser\nZubereitung\nKochen.")
		if !ok || p.SourceURL != tc.url || p.SourceName != tc.name {
			t.Errorf("%q: source %q / %q", tc.line, p.SourceURL, p.SourceName)
		}
	}
}

func TestReadTextUnknownDetailStaysInDescription(t *testing.T) {
	p, ok := recipeimport.ReadText("Dal\nCremig.\nSchwierigkeit: leicht\nPortionen: 2\nMein Tipp: mit Joghurt\n\nZutaten\n200 g Linsen\nZubereitung\nKochen.")
	if !ok || p.Description != "Cremig. Schwierigkeit: leicht Mein Tipp: mit Joghurt" || p.Servings != 2 {
		t.Fatalf("description %q, servings %d", p.Description, p.Servings)
	}
}

func TestReadTextBulletSteps(t *testing.T) {
	p, ok := recipeimport.ReadText("Dal\nZutaten\n200 g Linsen\n1 Zwiebel\nZubereitung\n- Zwiebel dünsten\n- Linsen zugeben")
	if want := []string{"Zwiebel dünsten", "Linsen zugeben"}; !ok || !slices.Equal(p.Steps, want) {
		t.Fatalf("steps = %q, %v", p.Steps, ok)
	}
}

func TestReadTextFirstLineGroup(t *testing.T) {
	p, ok := recipeimport.ReadText("Kuchen\nZutaten\nFür den Teig:\n200 g Mehl\n2 Eier\nFür den Guss:\n100 g Zucker\nZubereitung\nBacken.")
	if want := []groupShape{{"Für den Teig", 2}, {"Für den Guss", 1}}; !ok || !slices.Equal(shapes(p.Groups), want) {
		t.Fatalf("groups = %v, %v", shapes(p.Groups), ok)
	}
	// An unnamed group holds the lines before the first group line.
	p, _ = recipeimport.ReadText("Kuchen\nZutaten\n200 g Mehl\nFür den Guss:\n100 g Zucker\nZubereitung\nBacken.")
	if want := []groupShape{{"", 1}, {"Für den Guss", 1}}; !slices.Equal(shapes(p.Groups), want) {
		t.Fatalf("groups = %v", shapes(p.Groups))
	}
}

// Text that was never meant for this format reads as it did before the
// format had details and groups. The expected pages were recorded from the
// reader before that change.
func TestReadTextOrdinaryPastesUnchanged(t *testing.T) {
	for _, tc := range []struct {
		text string
		want recipeimport.Page
	}{
		{`Omas Apfelkuchen
Saftiger Rührkuchen mit Äpfeln vom Blech. Über 112 Bewertungen und für köstlich befunden.
Arbeitszeit: ca. 20 Min.
Koch-/Backzeit: ca. 45 Min.

Zutaten für 4 Personen
200 g Mehl
3 Ei(er)
150 g Butter
1 Pck. Backpulver
4 Äpfel
etwas Zimt

Zubereitung
Butter und Zucker schaumig rühren, die Eier nach und nach unterrühren.

Mehl mit Backpulver mischen und unterheben. Äpfel schälen und auf dem Teig verteilen.

Bei 180 °C etwa 45 Minuten backen.`, recipeimport.Page{
			Title:       "Omas Apfelkuchen",
			Description: "Saftiger Rührkuchen mit Äpfeln vom Blech. Arbeitszeit: ca. 20 Min. Koch-/Backzeit: ca. 45 Min.",
			Groups:      []recipeimport.PageGroup{{Lines: []string{"200 g Mehl", "3 Ei(er)", "150 g Butter", "1 Pck. Backpulver", "4 Äpfel", "etwas Zimt"}}},
			Steps: []string{
				"Butter und Zucker schaumig rühren, die Eier nach und nach unterrühren.",
				"Mehl mit Backpulver mischen und unterheben. Äpfel schälen und auf dem Teig verteilen.",
				"Bei 180 °C etwa 45 Minuten backen.",
			},
			Language: "de",
		}},
		{`Slow-Roast Lamb Shoulder
Falling off the bone after a long afternoon in the oven. Serves 6.

Ingredients
1.250 kg lamb shoulder, bone in
500 ml chicken stock
30 g butter
2 lemons, juiced

Method
1. Heat the oven to 160 °C.
2. Brown the lamb in the butter.
3. Pour over the stock and lemon juice, cover and roast for 4 hours.`, recipeimport.Page{
			Title:       "Slow-Roast Lamb Shoulder",
			Description: "Falling off the bone after a long afternoon in the oven. Serves 6.",
			Groups:      []recipeimport.PageGroup{{Lines: []string{"1.250 kg lamb shoulder, bone in", "500 ml chicken stock", "30 g butter", "2 lemons, juiced"}}},
			Steps:       []string{"Heat the oven to 160 °C.", "Brown the lamb in the butter.", "Pour over the stock and lemon juice, cover and roast for 4 hours."},
			Language:    "en",
		}},
		{`Kürbissuppe
Hallo Anna, hier das Rezept von gestern:
1 Hokkaido-Kürbis
2 Zwiebeln
1 l Gemüsebrühe
200 ml Sahne

Kürbis und Zwiebeln würfeln und in Butter andünsten.
Mit der Brühe aufgießen und 20 Minuten köcheln.

Pürieren und die Sahne einrühren.

Liebe Grüße, Maria`, recipeimport.Page{
			Title:       "Kürbissuppe",
			Description: "Hallo Anna, hier das Rezept von gestern:",
			Groups:      []recipeimport.PageGroup{{Lines: []string{"1 Hokkaido-Kürbis", "2 Zwiebeln", "1 l Gemüsebrühe", "200 ml Sahne"}}},
			Steps:       []string{"Kürbis und Zwiebeln würfeln und in Butter andünsten. Mit der Brühe aufgießen und 20 Minuten köcheln.", "Pürieren und die Sahne einrühren.", "Liebe Grüße, Maria"},
		}},
	} {
		if p, ok := recipeimport.ReadText(tc.text); !ok || !reflect.DeepEqual(p, tc.want) {
			t.Errorf("ReadText(%.20q) =\n%#v\nwant\n%#v", tc.text, p, tc.want)
		}
	}
	// "1.250 kg" in an English text is 1.25 kg, not German 1250 kg.
	p, _ := recipeimport.ReadText(metricLamb)
	if q := recipeimport.Build(p, nil).Recipe.IngredientGroups[0].Ingredients[0].Quantity; q == nil || *q != 1.25 {
		t.Errorf("lamb quantity = %v, want 1.25", q)
	}
}

const metricLamb = "Lamb\nIngredients\n1.250 kg lamb shoulder\n500 ml stock\nMethod\nRoast."

// Real answers of an AI chat to the recipe instruction, copied as they came.
func TestReadTextAIAnswers(t *testing.T) {
	files, _ := filepath.Glob("testdata/ai/*recipe*.txt")
	if len(files) < 4 {
		t.Fatalf("found %d answers in testdata/ai", len(files))
	}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		p, ok := recipeimport.ReadText(string(raw))
		if !ok || p.Title == "" || p.Servings == 0 || p.PrepMinutes == nil || p.CookMinutes == nil ||
			len(p.Tags) == 0 || len(p.Groups) < 2 || len(p.Steps) == 0 {
			t.Errorf("%s: %+v, %v", f, p, ok)
			continue
		}
		if d := recipeimport.Build(p, nil); len(d.Review) != 0 {
			t.Errorf("%s: reviews %+v", f, d.Review)
		}
	}
}
