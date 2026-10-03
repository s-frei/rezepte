package recipeimport_test

import (
	"strings"
	"testing"

	"github.com/s-frei/rezepte/service/internal/recipe"
	"github.com/s-frei/rezepte/service/internal/recipeimport"
)

func TestParseLine(t *testing.T) {
	for _, tc := range []struct {
		line string
		want recipe.Ingredient
		sure bool
	}{
		{"200 g Mehl", recipe.Ingredient{Quantity: ptr(200.0), Unit: ptr("g"), Name: "Mehl"}, true},
		{"200g Mehl, gesiebt", recipe.Ingredient{Quantity: ptr(200.0), Unit: ptr("g"), Name: "Mehl", Note: ptr("gesiebt")}, true},
		{"2 EL Olivenöl", recipe.Ingredient{Quantity: ptr(2.0), Unit: ptr("EL"), Name: "Olivenöl"}, true},
		{"1,5 l Milch", recipe.Ingredient{Quantity: ptr(1.5), Unit: ptr("l"), Name: "Milch"}, true},
		{"½ TL Salz", recipe.Ingredient{Quantity: ptr(0.5), Unit: ptr("TL"), Name: "Salz"}, true},
		{"1 1/2 cups flour", recipe.Ingredient{Quantity: ptr(1.5), Unit: ptr("cups"), Name: "flour"}, true},
		{"1 ½ cup flour", recipe.Ingredient{Quantity: ptr(1.5), Unit: ptr("cup"), Name: "flour"}, true},
		{"2 wholesome rolls", recipe.Ingredient{Quantity: ptr(2.0), Name: "wholesome rolls"}, true},
		{"1 awesome sauce", recipe.Ingredient{Quantity: ptr(1.0), Name: "awesome sauce"}, true},
		{"some salt", recipe.Ingredient{Name: "some salt"}, false},
		{"Salz (optional)", recipe.Ingredient{Name: "Salz (optional)"}, false},
		{"1½ Tassen Reis", recipe.Ingredient{Quantity: ptr(1.5), Unit: ptr("Tassen"), Name: "Reis"}, true},
		{"1–2 Knoblauchzehen", recipe.Ingredient{Quantity: ptr(1.0), Name: "Knoblauchzehen", Note: ptr("1–2")}, true},
		{"1 bis 2 Zwiebeln", recipe.Ingredient{Quantity: ptr(1.0), Name: "Zwiebeln", Note: ptr("1 bis 2")}, true},
		{"4 Eier", recipe.Ingredient{Quantity: ptr(4.0), Name: "Eier"}, true},
		{"2 rote Zwiebeln", recipe.Ingredient{Quantity: ptr(2.0), Name: "rote Zwiebeln"}, true},
		{"1 Prise Muskat", recipe.Ingredient{Quantity: ptr(1.0), Unit: ptr("Prise"), Name: "Muskat"}, true},
		{"1 Pck. Backpulver", recipe.Ingredient{Quantity: ptr(1.0), Unit: ptr("Pck."), Name: "Backpulver"}, true},
		{"3 tbsp butter (softened)", recipe.Ingredient{Quantity: ptr(3.0), Unit: ptr("tbsp"), Name: "butter", Note: ptr("softened")}, true},
		{"Salz", recipe.Ingredient{Name: "Salz"}, true},
		{"Salz und Pfeffer n.B.", recipe.Ingredient{Name: "Salz und Pfeffer n.B."}, false},
		{"salt to taste", recipe.Ingredient{Name: "salt to taste"}, false},
		{"Mehl (ca. 200 g)", recipe.Ingredient{Name: "Mehl (ca. 200 g)"}, false},
		{"  ", recipe.Ingredient{}, false},
	} {
		got, sure := parseAny(tc.line)
		if !equalIngredient(got, tc.want) || sure != tc.sure {
			t.Errorf("parseAny(%q) = %s, %v; want %s, %v", tc.line, show(got), sure, show(tc.want), tc.sure)
		}
	}
}

func TestParseLineClampsLengths(t *testing.T) {
	got, _ := parseAny("1 " + strings.Repeat("a", 300))
	if len([]rune(got.Name)) > 120 {
		t.Fatalf("name has %d runes", len([]rune(got.Name)))
	}
	got, _ = parseAny("1 g Mehl (" + strings.Repeat("b", 300) + ")")
	if got.Note == nil || len([]rune(*got.Note)) > 200 || got.Name != "Mehl" {
		t.Fatalf("note not clamped: %s", show(got))
	}
}

func parseAny(line string) (recipe.Ingredient, bool) {
	return recipeimport.ParseLineWith(line, recipeimport.AnyLanguage)
}
