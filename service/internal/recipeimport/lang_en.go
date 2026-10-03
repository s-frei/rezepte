package recipeimport

import "regexp"

// english reads English recipes, American and British.
var english = &language{
	tag: "en",
	units: []string{
		"g", "kg", "mg", "ml", "l",
		"tsp", "tsp.", "tbsp", "tbsp.", "teaspoon", "teaspoons", "tablespoon", "tablespoons",
		"cup", "cups", "oz", "lb", "lbs", "pinch", "pinches", "clove", "cloves",
		"can", "cans", "slice", "slices", "bunch", "bunches", "sprig", "sprigs", "stick", "sticks",
	},
	rangeWords:         []string{"to"},
	pluralMarkers:      []string{"(s)", "(es)"},
	unitModifiers:      []string{"heaped", "heaping", "level", "rounded", "scant", "generous"},
	vague:              []string{"to taste", "some", "optional"},
	ingredientHeadings: []string{"Ingredients", "You'll need", "Youll need"},
	stepHeadings:       []string{"Method", "Instructions", "Directions", "Preparation"},
	stepWords:          []string{"Step"},
	promo: []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b\d[\d,.]*\s+ratings?\b`),
		regexp.MustCompile(`★`),
	},
}
