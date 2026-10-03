package recipeimport_test

import (
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
	if want := []string{"400 g Mehl", "4 Eier", "200 g Bergkäse"}; !slices.Equal(p.Ingredients, want) {
		t.Fatalf("ingredients = %q", p.Ingredients)
	}
	if want := []string{"Mehl und Eier verrühren. Kräftig schlagen.", "Spätzle ins kochende Wasser schaben."}; !slices.Equal(p.Steps, want) {
		t.Fatalf("steps = %q", p.Steps)
	}
}

func TestReadTextEnglishParagraphSteps(t *testing.T) {
	p, ok := recipeimport.ReadText("Pancakes\n\nIngredients\n2 cups flour\n2 eggs\n\nMethod\nWhisk everything.\n\nFry in butter.")
	if !ok || p.Title != "Pancakes" || len(p.Ingredients) != 2 || len(p.Steps) != 2 {
		t.Fatalf("got %+v, %v", p, ok)
	}
}

func TestReadTextWithoutHeadings(t *testing.T) {
	p, ok := recipeimport.ReadText("Guacamole\n2 Avocados\n1 Limette\n½ TL Salz\n\nAlles zerdrücken und abschmecken.")
	if !ok || p.Title != "Guacamole" || len(p.Ingredients) != 3 || len(p.Steps) != 1 {
		t.Fatalf("got %+v, %v", p, ok)
	}
}

func TestReadTextBareIngredientList(t *testing.T) {
	p, ok := recipeimport.ReadText("Einkauf\n200 g Mehl\n3 Eier")
	if !ok || p.Title != "Einkauf" || len(p.Ingredients) != 2 || len(p.Steps) != 0 {
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
	if !ok || p.Description != "4 Portionen 2 people love this" || !slices.Equal(p.Ingredients, []string{"2 Eier"}) {
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
	if !slices.Equal(p.Ingredients, []string{"1 Ei"}) {
		t.Fatalf("ingredients = %q", p.Ingredients)
	}
}

func TestReadTextHeadingVariants(t *testing.T) {
	for _, text := range []string{
		"Suppe\nZutaten für 4 Personen\n1 l Wasser\n2 Karotten\nZubereitung\nKochen.",
		"Soup\nYou'll need:\n1 l water\n2 carrots\nMethod\nCook.",
		"Soup\nYoull need\n1 l water\n2 carrots\nInstructions\nCook.",
	} {
		p, ok := recipeimport.ReadText(text)
		if !ok || len(p.Ingredients) != 2 || len(p.Steps) != 1 {
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
		if !ok || !slices.Equal(p.Ingredients, tc.ingredients) || !slices.Equal(p.Steps, tc.steps) {
			t.Errorf("ReadText(%q) = %q / %q, %v", tc.text, p.Ingredients, p.Steps, ok)
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
		if !ok || p.Title != "" || len(p.Ingredients) != 2 || len(p.Steps) != 1 {
			t.Errorf("ReadText(%q) = %+v, %v", text, p, ok)
		}
	}
}
