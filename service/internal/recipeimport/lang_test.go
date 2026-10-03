package recipeimport_test

import (
	"encoding/json"
	"net/url"
	"os"
	"slices"
	"strings"
	"testing"
	"unicode"

	"github.com/s-frei/rezepte/service/internal/i18n"
	"github.com/s-frei/rezepte/service/internal/recipe"
	"github.com/s-frei/rezepte/service/internal/recipeimport"
)

type lineCase struct {
	line string
	want recipe.Ingredient
	sure bool
}

func checkLines(t *testing.T, name string, parse func(string) (recipe.Ingredient, bool), cases []lineCase) {
	t.Helper()
	for _, tc := range cases {
		got, sure := parse(tc.line)
		if !equalIngredient(got, tc.want) || sure != tc.sure {
			t.Errorf("%s: %q = %s, %v; want %s, %v", name, tc.line, show(got), sure, show(tc.want), tc.sure)
		}
		if sure && got.Name != "" && !unicode.IsLetter([]rune(got.Name)[0]) && !unicode.IsDigit([]rune(got.Name)[0]) {
			t.Errorf("%s: %q is sure with a name starting with punctuation: %q", name, tc.line, got.Name)
		}
	}
}

func withGerman(line string) (recipe.Ingredient, bool) {
	return recipeimport.ParseLineWith(line, recipeimport.LanguageByTag("de"))
}

func withEnglish(line string) (recipe.Ingredient, bool) {
	return recipeimport.ParseLineWith(line, recipeimport.LanguageByTag("en"))
}

// The ingredient lines of Chefkoch's "Japanisch-koreanischer Gurkensalat"
// and "Der perfekte Pfannkuchen", as the page serves them.
var chefkochLines = []lineCase{
	{"1 große Gurke(n)", recipe.Ingredient{Quantity: ptr(1.0), Name: "große Gurke"}, true},
	{"1 EL, gehäuft Salz (zum Entwässern der Gurke)", recipe.Ingredient{Quantity: ptr(1.0), Unit: ptr("EL"), Name: "Salz", Note: ptr("gehäuft, zum Entwässern der Gurke")}, true},
	{"2 Zehe/n Knoblauch", recipe.Ingredient{Quantity: ptr(2.0), Unit: ptr("Zehe"), Name: "Knoblauch"}, true},
	{"1 Stange/n Frühlingszwiebel(n)", recipe.Ingredient{Quantity: ptr(1.0), Unit: ptr("Stange"), Name: "Frühlingszwiebel"}, true}, //nolint:misspell // German unit
	{"1 EL Sojasauce", recipe.Ingredient{Quantity: ptr(1.0), Unit: ptr("EL"), Name: "Sojasauce"}, true},
	{"2 EL Reisessig", recipe.Ingredient{Quantity: ptr(2.0), Unit: ptr("EL"), Name: "Reisessig"}, true},
	{"1 EL Zucker", recipe.Ingredient{Quantity: ptr(1.0), Unit: ptr("EL"), Name: "Zucker"}, true},
	{"1 TL Chiliflocken (koreanische (Gochugaru))", recipe.Ingredient{Quantity: ptr(1.0), Unit: ptr("TL"), Name: "Chiliflocken", Note: ptr("koreanische (Gochugaru)")}, true},
	{"1 EL Sesam", recipe.Ingredient{Quantity: ptr(1.0), Unit: ptr("EL"), Name: "Sesam"}, true},
	{"1 TL Sesamöl", recipe.Ingredient{Quantity: ptr(1.0), Unit: ptr("TL"), Name: "Sesamöl"}, true},
	{"3 große Ei(er), Größe L", recipe.Ingredient{Quantity: ptr(3.0), Name: "große Ei", Note: ptr("Größe L")}, true},
	{"1 Prise(n) Salz", recipe.Ingredient{Quantity: ptr(1.0), Unit: ptr("Prise"), Name: "Salz"}, true},
	{"1 TL, gestrichen Natron", recipe.Ingredient{Quantity: ptr(1.0), Unit: ptr("TL"), Name: "Natron", Note: ptr("gestrichen")}, true},
	{"2 EL leicht gehäuft Mehl", recipe.Ingredient{Quantity: ptr(2.0), Unit: ptr("EL"), Name: "Mehl", Note: ptr("leicht gehäuft")}, true},
}

func TestParseLineGerman(t *testing.T) {
	checkLines(t, "german", withGerman, chefkochLines)
	checkLines(t, "german", withGerman, []lineCase{
		// The longest modifier wins; "gut" alone is no modifier.
		{"1 TL, gut gehäuft Zucker", recipe.Ingredient{Quantity: ptr(1.0), Unit: ptr("TL"), Name: "Zucker", Note: ptr("gut gehäuft")}, true},
		{"1 TL, leicht gestrichen Salz", recipe.Ingredient{Quantity: ptr(1.0), Unit: ptr("TL"), Name: "Salz", Note: ptr("leicht gestrichen")}, true},
		{"1 EL gut gekühlte Butter", recipe.Ingredient{Quantity: ptr(1.0), Unit: ptr("EL"), Name: "gut gekühlte Butter"}, true},
		// A comma after the unit without a known modifier: kept whole, flagged.
		{"1 TL, schwach gehäuft Salz", recipe.Ingredient{Name: "1 TL, schwach gehäuft Salz"}, false},
		{"1 EL, Salz", recipe.Ingredient{Name: "1 EL, Salz"}, false},
		// Every word list is case-insensitive.
		{"1 Bis 2 Zwiebeln", recipe.Ingredient{Quantity: ptr(1.0), Name: "Zwiebeln", Note: ptr("1 Bis 2")}, true},
		{"1 Gurke(N)", recipe.Ingredient{Quantity: ptr(1.0), Name: "Gurke"}, true},
		// A dot before exactly three digits groups thousands.
		{"1.000 g Mehl", recipe.Ingredient{Quantity: ptr(1000.0), Unit: ptr("g"), Name: "Mehl"}, true},
		{"1.250 kg Kartoffeln", recipe.Ingredient{Quantity: ptr(1250.0), Unit: ptr("kg"), Name: "Kartoffeln"}, true},
		{"1.5 l Milch", recipe.Ingredient{Quantity: ptr(1.5), Unit: ptr("l"), Name: "Milch"}, true},
		{"1,25 l Milch", recipe.Ingredient{Quantity: ptr(1.25), Unit: ptr("l"), Name: "Milch"}, true},
		{"0.125 l Sahne", recipe.Ingredient{Quantity: ptr(0.125), Unit: ptr("l"), Name: "Sahne"}, true},
		{"1.000g Mehl", recipe.Ingredient{Quantity: ptr(1000.0), Unit: ptr("g"), Name: "Mehl"}, true},
		{"1.000,5 g Mehl", recipe.Ingredient{Quantity: ptr(1000.5), Unit: ptr("g"), Name: "Mehl"}, true},
		// Abbreviations written with their dot.
		{"1 TL. Salz", recipe.Ingredient{Quantity: ptr(1.0), Unit: ptr("TL."), Name: "Salz"}, true},
		{"2 EL. Öl", recipe.Ingredient{Quantity: ptr(2.0), Unit: ptr("EL."), Name: "Öl"}, true},
	})
	// English has no thousands dot: "1.000" stays one.
	checkLines(t, "english", withEnglish, []lineCase{
		{"1.000 g flour", recipe.Ingredient{Quantity: ptr(1.0), Unit: ptr("g"), Name: "flour"}, true},
	})
}

// No line given to Build is dropped: a line the parser cannot split comes
// back whole and flagged.
func TestBuildKeepsEveryLine(t *testing.T) {
	lines := []string{"1 TL, schwach gehäuft Salz", "1 EL, Salz", "200 g Mehl", "1 , Zucker", "1 ()", "2 - "}
	for _, lang := range []string{"de", "en", ""} {
		d := recipeimport.Build(recipeimport.Page{Language: lang, Ingredients: lines}, nil)
		got := d.Recipe.IngredientGroups[0].Ingredients
		if len(got) != len(lines) {
			t.Fatalf("language %q: %d ingredients from %d lines: %+v", lang, len(got), len(lines), got)
		}
		// The English table knows no "TL"/"EL", so it reads those two lines
		// as name and note; the others know them and flag the lines.
		if lang != "en" && len(d.Review) != len(lines)-1 {
			t.Errorf("language %q: review = %+v", lang, d.Review)
		}
	}
}

// The Chefkoch Gurkensalat page, end to end: every ingredient read right,
// none to check.
func TestChefkochGurkensalatDraft(t *testing.T) {
	body, err := os.ReadFile("testdata/pages/chefkoch-gurkensalat.html")
	if err != nil {
		t.Fatal(err)
	}
	p, ok := recipeimport.ReadPage(body, &url.URL{Scheme: "https", Host: "www.chefkoch.de"})
	if !ok {
		t.Fatal("no recipe")
	}
	d := recipeimport.Build(p, nil)
	got := d.Recipe.IngredientGroups[0].Ingredients
	if len(got) != 10 || len(d.Review) != 0 {
		t.Fatalf("ingredients = %d, review = %+v", len(got), d.Review)
	}
	for i, ing := range got {
		if !equalIngredient(ing, chefkochLines[i].want) {
			t.Errorf("%q = %s, want %s", chefkochLines[i].line, show(ing), show(chefkochLines[i].want))
		}
	}
}

func TestParseLineEnglish(t *testing.T) {
	checkLines(t, "english", withEnglish, []lineCase{
		{"2 clove(s) garlic, minced", recipe.Ingredient{Quantity: ptr(2.0), Unit: ptr("clove"), Name: "garlic", Note: ptr("minced")}, true},
		{"1 tbsp, heaped sugar", recipe.Ingredient{Quantity: ptr(1.0), Unit: ptr("tbsp"), Name: "sugar", Note: ptr("heaped")}, true},
		{"1 (14-ounce) can coconut milk", recipe.Ingredient{Quantity: ptr(1.0), Unit: ptr("can"), Name: "coconut milk", Note: ptr("14-ounce")}, true},
		{"salt to taste", recipe.Ingredient{Name: "salt to taste"}, false},
		{"1 ½ cups flour", recipe.Ingredient{Quantity: ptr(1.5), Unit: ptr("cups"), Name: "flour"}, true},
		{"2 tsp level baking powder", recipe.Ingredient{Quantity: ptr(2.0), Unit: ptr("tsp"), Name: "baking powder", Note: ptr("level")}, true},
		{"1 TO 2 onions", recipe.Ingredient{Quantity: ptr(1.0), Name: "onions", Note: ptr("1 TO 2")}, true},
		{"2 Clove(S) garlic", recipe.Ingredient{Quantity: ptr(2.0), Unit: ptr("Clove"), Name: "garlic"}, true},
		// A parenthesis after the unit (King Arthur, RecipeTin Eats).
		{"1 can (14 oz) tomatoes", recipe.Ingredient{Quantity: ptr(1.0), Unit: ptr("can"), Name: "tomatoes", Note: ptr("14 oz")}, true},
		{"2/3 cup (142g) light brown sugar, packed", recipe.Ingredient{Quantity: ptr(2.0 / 3), Unit: ptr("cup"), Name: "light brown sugar", Note: ptr("142g, packed")}, true},
		{"150g (5oz) chicken breast", recipe.Ingredient{Quantity: ptr(150.0), Unit: ptr("g"), Name: "chicken breast", Note: ptr("5oz")}, true},
		// WP Recipe Maker writes an empty amount note as "(, …)".
		{"1 onion (, finely sliced into small pieces)", recipe.Ingredient{Quantity: ptr(1.0), Name: "onion", Note: ptr("finely sliced into small pieces")}, true},
		{"2 cloves garlic (, minced)", recipe.Ingredient{Quantity: ptr(2.0), Unit: ptr("cloves"), Name: "garlic", Note: ptr("minced")}, true},
	})
}

// A wrong split is flagged in every language: the German lines read with
// the English table, and splits no table can make sense of.
func TestParseLineFlagsWrongSplits(t *testing.T) {
	checkLines(t, "english", withEnglish, []lineCase{
		{"2 Zehe/n Knoblauch", recipe.Ingredient{Quantity: ptr(2.0), Name: "Zehe/n Knoblauch"}, false},
		{"1 - Salz", recipe.Ingredient{Quantity: ptr(1.0), Name: "- Salz"}, false},
		{"1 Chiliflocken (koreanische", recipe.Ingredient{Quantity: ptr(1.0), Name: "Chiliflocken (koreanische"}, false},
		{"1 Chiliflocken koreanische)", recipe.Ingredient{Quantity: ptr(1.0), Name: "Chiliflocken koreanische)"}, false},
	})
}

func TestSplitTrailingNote(t *testing.T) {
	for _, tc := range []struct{ in, name, note string }{
		{"Chiliflocken (koreanische (Gochugaru))", "Chiliflocken", "koreanische (Gochugaru)"},
		{"Salz (zum Entwässern der Gurke)", "Salz", "zum Entwässern der Gurke"},
		{"Mehl, gesiebt", "Mehl", "gesiebt"},
		// The last balanced group; the name it leaves is flagged by the safety net.
		{"Chiliflocken (koreanische (Gochugaru)", "Chiliflocken (koreanische", "Gochugaru"},
		{"Chiliflocken koreanische)", "Chiliflocken koreanische)", ""},
		{"(optional)", "(optional)", ""},
		{"Butter", "Butter", ""},
	} {
		if name, note := recipeimport.SplitTrailingNote(tc.in); name != tc.name || note != tc.note {
			t.Errorf("SplitTrailingNote(%q) = %q, %q; want %q, %q", tc.in, name, note, tc.name, tc.note)
		}
	}
}

func TestDetectLanguage(t *testing.T) {
	for _, tc := range []struct {
		tag   string
		lines []string
		want  string // "" is every table at once
	}{
		{"de-DE", nil, "de"},
		{"de", []string{"2 cups flour"}, "de"},
		{"en_GB", nil, "en"},
		{"EN", nil, "en"},
		{"fr-FR", []string{"1 EL Salz", "2 TL Zucker"}, "de"},
		{"", []string{"2 cups flour", "1 tsp salt"}, "en"},
		{"", []string{"Zutaten", "2 Zehe/n Knoblauch"}, "de"},
		{"", []string{"200 g flour"}, ""},
		{"", nil, ""},
	} {
		if got := recipeimport.DetectLanguage(tc.tag, tc.lines).Tag(); got != tc.want {
			t.Errorf("DetectLanguage(%q, %q) = %q, want %q", tc.tag, tc.lines, got, tc.want)
		}
	}
}

func TestStripAuthorFromTitle(t *testing.T) {
	for _, tc := range []struct{ title, author, want string }{
		{"Japanisch-koreanischer Gurkensalat von KochFan42", "KochFan42", "Japanisch-koreanischer Gurkensalat"},
		{"Easy Pasta by Jamie Oliver", "Jamie Oliver", "Easy Pasta"},
		{"Soupe à l'oignon de Marie", "Marie", "Soupe à l'oignon"},
		{"Pancakes – Jane", "Jane", "Pancakes"},
		{"Kartoffelsalat Oma Erna", "Erna", "Kartoffelsalat Oma"},
		{"Gurkensalat von KochFan422", "KochFan42", "Gurkensalat von KochFan422"},
		{"Salat von XAnna", "Anna", "Salat von XAnna"},
		{"KochFan42", "KochFan42", "KochFan42"},
		{"von KochFan42", "KochFan42", "von KochFan42"},
		{"Gurkensalat", "", "Gurkensalat"},
	} {
		if got := recipeimport.StripAuthorFromTitle(tc.title, tc.author); got != tc.want {
			t.Errorf("StripAuthorFromTitle(%q, %q) = %q, want %q", tc.title, tc.author, got, tc.want)
		}
	}
}

func TestRemovePromo(t *testing.T) {
	const gurke = "Japanisch-koreanischer Gurkensalat - frisch, ein wenig scharf und köstlich. Über 112 Bewertungen und für köstlich befunden. Mit ► Portionsrechner ► Kochbuch ► Video-Tipps!"
	if got := recipeimport.RemovePromo(gurke, recipeimport.LanguageByTag("de")); got != "Japanisch-koreanischer Gurkensalat - frisch, ein wenig scharf und köstlich." {
		t.Errorf("german: %q", got)
	}
	if got := recipeimport.RemovePromo("Fluffy pancakes for Sunday. Rated 4.8 from 1,234 ratings. ★★★★★", recipeimport.LanguageByTag("en")); got != "Fluffy pancakes for Sunday." {
		t.Errorf("english: %q", got)
	}
	for _, s := range []string{"Ein Klassiker. Mit 3 Eiern.", "Takes 20 minutes. Serves 4!", ""} {
		if got := recipeimport.RemovePromo(s, recipeimport.AnyLanguage); got != s {
			t.Errorf("RemovePromo(%q) = %q", s, got)
		}
	}
}

func TestReadPageChefkochTitleDescriptionSteps(t *testing.T) {
	body := []byte(`<html lang="de"><script type="application/ld+json">{"@context":"https://schema.org","@graph":[
		{"@type":"Recipe","inLanguage":"de-DE","author":{"@id":"https://www.chefkoch.de/user/profil/x#author"},
		 "name":"Japanisch-koreanischer Gurkensalat von KochFan42",
		 "description":"Japanisch-koreanischer Gurkensalat - frisch, ein wenig scharf und köstlich. Über 112 Bewertungen und für köstlich befunden. Mit ► Portionsrechner ► Kochbuch ► Video-Tipps!",
		 "recipeIngredient":["2 Zehe/n Knoblauch"],
		 "recipeInstructions":[{"@type":"HowToSection","name":"Zubereitung","itemListElement":[{"@type":"HowToStep","text":"Gurke waschen."},{"@type":"HowToStep","text":"Mischen."}]}]},
		{"@type":"Person","@id":"https://www.chefkoch.de/user/profil/x#author","name":"KochFan42"}]}</script></html>`)
	p, ok := recipeimport.ReadPage(body, &url.URL{Scheme: "https", Host: "www.chefkoch.de"})
	if !ok {
		t.Fatal("no recipe")
	}
	if p.Title != "Japanisch-koreanischer Gurkensalat" {
		t.Errorf("title = %q", p.Title)
	}
	if p.Description != "Japanisch-koreanischer Gurkensalat - frisch, ein wenig scharf und köstlich." {
		t.Errorf("description = %q", p.Description)
	}
	if !slices.Equal(p.Steps, []string{"Gurke waschen.", "Mischen."}) {
		t.Errorf("steps = %q", p.Steps)
	}
	if p.Language != "de" {
		t.Errorf("language = %q", p.Language)
	}
}

func TestReadPageSectionNamesOnlyWithSeveralSections(t *testing.T) {
	body := []byte(`<script type="application/ld+json">{"@type":"Recipe","name":"Torte","recipeInstructions":[
		{"@type":"HowToSection","name":"Boden","itemListElement":[{"@type":"HowToStep","text":"Backen."},{"@type":"HowToStep","text":"Kühlen."}]},
		{"@type":"HowToSection","name":"Creme","itemListElement":[{"@type":"HowToStep","text":"Schlagen."}]}]}</script>`)
	p, _ := recipeimport.ReadPage(body, &url.URL{Scheme: "https", Host: "example.com"})
	if want := []string{"Boden: Backen.", "Kühlen.", "Creme: Schlagen."}; !slices.Equal(p.Steps, want) {
		t.Errorf("steps = %q, want %q", p.Steps, want)
	}
}

func TestReadPageLanguageSources(t *testing.T) {
	for _, tc := range []struct{ body, want string }{
		{`<html lang="en-GB"><script type="application/ld+json">{"@type":"Recipe","name":"X"}</script>`, "en"},
		{`<html lang="en"><script type="application/ld+json">{"@graph":[{"@type":"WebPage","inLanguage":"de-AT"},{"@type":"Recipe","name":"X"}]}</script>`, "de"},
		{`<html lang="en"><script type="application/ld+json">{"@type":"Recipe","inLanguage":"de","name":"X"}</script>`, "de"},
		{`<script type="application/ld+json">{"@type":"Recipe","name":"X","recipeIngredient":["1 EL Salz"]}</script>`, "de"},
		{`<script type="application/ld+json">{"@type":"Recipe","name":"X","recipeIngredient":["1 g Salz"]}</script>`, ""},
	} {
		p, _ := recipeimport.ReadPage([]byte(tc.body), &url.URL{Scheme: "https", Host: "example.com"})
		if p.Language != tc.want {
			t.Errorf("%s: language = %q, want %q", tc.body, p.Language, tc.want)
		}
	}
}

func TestReadTextDetectsLanguage(t *testing.T) {
	p, ok := recipeimport.ReadText("Gurkensalat\n\nZutaten\n2 Zehe/n Knoblauch\n1 große Gurke(n)\n\nZubereitung\nSchritt 1 Mischen.")
	if !ok || p.Language != "de" || !slices.Equal(p.Steps, []string{"Mischen."}) {
		t.Fatalf("got %+v, %v", p, ok)
	}
	d := recipeimport.Build(p, nil)
	if len(d.Review) != 0 || *d.Recipe.IngredientGroups[0].Ingredients[0].Unit != "Zehe" {
		t.Fatalf("draft = %+v", d)
	}
}

func TestBuildReadsWithThePageLanguage(t *testing.T) {
	var lines []string
	for _, c := range chefkochLines {
		lines = append(lines, c.line)
	}
	d := recipeimport.Build(recipeimport.Page{Title: "Gurkensalat", Language: "de", Ingredients: lines}, nil)
	if len(d.Review) != 0 {
		t.Fatalf("review = %+v", d.Review)
	}
	for i, ing := range d.Recipe.IngredientGroups[0].Ingredients {
		if !equalIngredient(ing, chefkochLines[i].want) {
			t.Errorf("%q = %s", lines[i], show(ing))
		}
	}
}

// Every interface language needs an import table, or a recipe in that
// language imports with confidently wrong ingredients.
func TestEveryAppLanguageHasAnImportTable(t *testing.T) {
	for _, l := range i18n.Locales() {
		found := false
		for _, table := range recipeimport.Languages {
			found = found || table.Tag() == l
		}
		if !found {
			t.Errorf("app language %q has no import language table: add service/internal/recipeimport/lang_%s.go, see docs/memory/content/howtos/add-an-import-language.mdx", l, strings.ToLower(l))
		}
	}
}

// The import dialog shows import_example as how a pasted recipe should look,
// so in every interface language it must read as a clean recipe with that
// language's table.
func TestEveryAppLanguageExampleImportsCleanly(t *testing.T) {
	const howto = "see docs/memory/content/howtos/add-an-import-language.mdx"
	for _, l := range i18n.Locales() {
		body, err := os.ReadFile("../../../frontend/messages/" + l + ".json")
		if err != nil {
			t.Fatal(err)
		}
		var catalog map[string]any
		if err := json.Unmarshal(body, &catalog); err != nil {
			t.Fatalf("frontend/messages/%s.json: %v", l, err)
		}
		example, _ := catalog["import_example"].(string)
		lang := recipeimport.LanguageByTag(l)
		if example == "" || lang == nil {
			t.Errorf("%s: import_example is missing, or the language has no import table; %s", l, howto)
			continue
		}
		p, ok := recipeimport.ReadTextWith(example, lang)
		if !ok || p.Title == "" || len(p.Ingredients) < 2 || len(p.Steps) < 1 {
			t.Errorf("%s: import_example reads as title %q, %d ingredients, %d steps; it needs a title, two ingredients and a step under the table's headings; %s", l, p.Title, len(p.Ingredients), len(p.Steps), howto)
			continue
		}
		for _, line := range p.Ingredients {
			if _, sure := recipeimport.ParseLineWith(line, lang); !sure {
				t.Errorf("%s: import_example ingredient %q needs review; put a known amount and unit first; %s", l, line, howto)
			}
		}
	}
}
