package shopping_test

import (
	"testing"

	"github.com/s-frei/rezepte/service/internal/recipe"
	"github.com/s-frei/rezepte/service/internal/shopping"
	"github.com/s-frei/rezepte/service/internal/user"
)

func TestDefaultDepartment(t *testing.T) {
	cases := map[string]shopping.Department{"Zwiebeln": shopping.Produce, "Sahne": shopping.Chilled, "Rinderhackfleisch": shopping.Meat,
		"Kapern": shopping.Pantry, "Toastbrot": shopping.Bakery, "onion": shopping.Produce, "heavy cream": shopping.Chilled, "Spülmittel": shopping.Household, "Quatschwort": shopping.Other}
	for name, want := range cases {
		if got := shopping.DefaultDepartment(shopping.NameKey(name)); got != want {
			t.Errorf("%s: %s, want %s", name, got, want)
		}
	}
}

func TestDepartmentValid(t *testing.T) {
	if !shopping.Frozen.Valid() || shopping.Department("x").Valid() {
		t.Error("Valid")
	}
}

func TestCommonIngredientDepartments(t *testing.T) {
	cases := map[string]shopping.Department{
		"Schweineschnitzel": shopping.Meat, "pork chop": shopping.Meat, "Rinderbraten": shopping.Meat,
		"Pfeffer": shopping.Pantry, "black pepper": shopping.Pantry, "Paprikapulver": shopping.Pantry, "Muskat": shopping.Pantry,
		"bell pepper": shopping.Produce, "Basilikum": shopping.Produce, "Schnittlauch": shopping.Produce, "Dill": shopping.Produce,
		"Semmelbrösel": shopping.Pantry, "breadcrumbs": shopping.Pantry, "Butterschmalz": shopping.Chilled, "lard": shopping.Chilled,
		"Crème fraîche": shopping.Chilled, "Creme fraiche": shopping.Chilled, "Mascarpone": shopping.Chilled,
		"Kokosmilch": shopping.Pantry, "Vollmilch": shopping.Chilled, "Sonnenblumenöl": shopping.Pantry, "vegetable oil": shopping.Pantry,
		"Weißweinessig": shopping.Pantry, "Dinkelmehl": shopping.Pantry, "Bergkäse": shopping.Chilled, "Orangensaft": shopping.Drinks,
		"Bauernbrot": shopping.Bakery, "Leberwurst": shopping.Meat,
		"Rinderhackfleisch, gemischt (halb und halb)": shopping.Meat, "Hackfleisch (gemischt)": shopping.Meat,
		"Paprikaschoten": shopping.Produce, "Chilischote": shopping.Produce, "Vanilleschote": shopping.Pantry,
		"Lasagneplatten": shopping.Pantry, "Dinkel-Brötchen": shopping.Bakery, "Dinkel-Vollkornbrötchen": shopping.Bakery,
	}
	for in, want := range cases {
		if got := shopping.DefaultDepartment(shopping.NameKey(in)); got != want {
			t.Errorf("%s → %s, want %s", in, got, want)
		}
	}
}

// Ingredients of the demo recipes that stay in Other, with the reason.
var demoOther = map[string]string{}

func TestDemoIngredientsAreSorted(t *testing.T) {
	for _, l := range []user.Locale{"en", "de"} {
		samples, err := recipe.Samples(l)
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range samples {
			for _, g := range s.IngredientGroups {
				for _, i := range g.Ingredients {
					if _, ok := demoOther[i.Name]; !ok && shopping.DefaultDepartment(shopping.NameKey(i.Name)) == shopping.Other {
						t.Errorf("%s: %q is in Other", l, i.Name)
					}
				}
			}
		}
	}
}
