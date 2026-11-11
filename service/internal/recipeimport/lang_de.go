package recipeimport

import "regexp"

// german reads German recipes, Chefkoch's included.
var german = &language{
	tag: "de",
	units: []string{
		"g", "gr", "Gramm", "kg", "mg", "ml", "cl", "dl", "l", "Liter",
		"TL", "TL.", "EL", "EL.", "Teelöffel", "Esslöffel", "Msp", "Msp.", "Prise", "Prisen",
		"Pck", "Pck.", "Päckchen", "Pkg", "Pkg.", "Packung", "Packungen", "Dose", "Dosen",
		"Flasche", "Flaschen", "Bund", "Bunde", "Bünde", "Stück", "Stücke", "Stk", "Stk.", "Zehe", "Zehen", "Scheibe", "Scheiben",
		"Becher", "Tasse", "Tassen", "Glas", "Gläser", "Handvoll",
		"Zweig", "Zweige", "Blatt", "Blätter", "Würfel",
		"Stange", "Stangen", //nolint:misspell // German unit, not "Strange"
	},
	thousandsSeparators: []string{"."},
	decimalSeparators:   []string{","},
	rangeWords:          []string{"bis"},
	pluralMarkers:       []string{"(n)", "(s)", "(e)", "(en)", "(er)", "/n", "/e", "/en", "/s", "/er"},
	// "groß", "klein" and "gut" are left out alone: after a unit they
	// usually start the name ("1 Bund klein geschnittene Petersilie", "1 EL
	// gut gekühlte Butter"); "gut gehäuft" is a phrase of its own.
	unitModifiers: []string{
		"gehäuft", "gut gehäuft", "leicht gehäuft",
		"gestrichen", "leicht gestrichen", "knapp",
	},
	vague:              []string{"n.B.", "nach Belieben", "nach Geschmack", "etwas", "optional"},
	ingredientHeadings: []string{"Zutaten", "Zutaten für …"},
	stepHeadings:       []string{"Zubereitung", "Anleitung", "Schritte"},
	stepWords:          []string{"Schritt"},
	detailKeys: map[string]string{
		"Portionen": "servings", "Vorbereitung": "prep", "Kochzeit": "cook", "Tags": "tags", "Quelle": "source",
	},
	minuteWords: []string{"Min.", "Min", "Minuten", "Minute"},
	hourWords:   []string{"Std.", "Std", "Stunden", "Stunde", "h"},
	promo: []*regexp.Regexp{
		regexp.MustCompile(`(?i)^Über \d[\d.]* Bewertungen`),
		regexp.MustCompile(`►`),
	},
}
