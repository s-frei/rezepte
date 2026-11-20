package recipe

import (
	"encoding/json"
	"testing"
)

// render writes a Match as compact text: the section, then per find the
// field, its items with hits in [brackets], and +more.
func render(m *Match) string {
	if m == nil {
		return "<nil>"
	}
	out := m.Section
	if m.Title != nil {
		out += " ~ "
		for _, p := range m.Title {
			if p.Hit {
				out += "[" + p.Text + "]"
			} else {
				out += p.Text
			}
		}
	}
	for _, f := range m.Finds {
		out += " | " + f.Field + ":"
		for i, it := range f.Items {
			if i > 0 {
				out += ","
			}
			out += " "
			for _, p := range it.Parts {
				if p.Hit {
					out += "[" + p.Text + "]"
				} else {
					out += p.Text
				}
			}
		}
		if f.More > 0 {
			b, _ := json.Marshal(f.More)
			out += " +" + string(b)
		}
	}
	return out
}

func TestMatchCard(t *testing.T) {
	type recipe struct {
		title       string
		tags        []string
		ingredients []string
		description string
	}
	paprika := recipe{"Gefüllte Paprikaschoten", []string{"hauptgericht"}, []string{"Paprikaschoten", "Brötchen vom Vortag", "Butter"}, "Dazu passt frisches Brot mit Butter."}
	cases := []struct {
		q    string
		r    recipe
		want string
	}{
		// In the title: the title shows it, no find.
		{"brot", recipe{"Bauernbrot", nil, []string{"Mehl"}, ""}, "title ~ Bauern[brot]"},
		// The strongest field only: the ingredient, not the description too.
		{"brot", paprika, "ingredients | ingredients: [Bröt]chen vom Vortag"},
		// A find per field; the section is the strongest of them.
		{"brot butter", recipe{"Einfaches Brot", nil, []string{"Mehl", "Butter"}, ""}, "title ~ Einfaches [Brot] | ingredients: [Butter]"},
		{"Butter", recipe{"Suppe", nil, nil, "Dazu passt frisches Brot mit Butter."}, "description | description: … [Butter]."},
		// Spelled either way, and the find in the reader's spelling.
		{"suesskartoffel", recipe{"Pommes", nil, []string{"Süßkartoffeln"}, ""}, "ingredients | ingredients: [Süßkartoffel]n"},
		// Tags before ingredients; several tags, two named and one counted.
		{"dessert", recipe{"Grütze", []string{"dessert", "sommer-dessert", "winter-dessert"}, []string{"Dessertwein"}, ""}, "tags | tags: [dessert], sommer-[dessert] +1"},
		// A short word only at a word start.
		{"ei", recipe{"Rührei", nil, []string{"Eier", "Brei"}, ""}, "ingredients | ingredients: [Ei]er"},
		// Twice in the description: the first excerpt, the second counted.
		// Twice in the description, both in the excerpt: nothing more to count.
		{"knusprig", recipe{"Flammkuchen", nil, nil, "Dünner, knuspriger Teig mit Speck, knusprig aus dem Ofen."}, "description | description: … [knusprig]er Teig mit Speck, [knusprig] aus dem Ofen."},
		// The second beyond the excerpt's cut: counted.
		{"knusprig", recipe{"Flammkuchen", nil, nil, "Dünner, knuspriger Flammkuchen mit Crème fraîche, Zwiebeln und Speck, frisch und knusprig aus dem Ofen."}, "description | description: … [knusprig]er Flammkuchen mit Crème fraîche, Zwiebeln und Speck, … +1"},
		// Two words in one field: one find naming both, in the field's order.
		{"brot butter", recipe{"Klopse", nil, []string{"Toastbrot", "Kapern", "Butter"}, ""}, "ingredients | ingredients: Toast[brot], [Butter]"},
		// Both words in one item: the item named once, matches that meet
		// marked as one.
		{"brot butter", recipe{"Stulle", nil, []string{"Butterbrot"}, ""}, "ingredients | ingredients: [Butterbrot]"},
		// Two words in the description: one excerpt from the first of them.
		// A word another field holds is still marked where a find shows it.
		{"brot butter", recipe{"Schoten", nil, []string{"Toastbrot"}, "Mit Butter. Übriges Brot kommt hinein."}, "ingredients | ingredients: Toast[brot] | description: … [Butter]. Übriges [Brot] kommt hinein."},
		{"speck ofen", recipe{"Flammkuchen", nil, nil, "Dünner Teig mit Speck, knusprig aus dem Ofen."}, "description | description: … [Speck], knusprig aus dem [Ofen]."},
		// Nothing found anywhere.
		{"zzz", paprika, "<nil>"},
	}
	for _, c := range cases {
		got := render(matchCard(matchWords(searchWords(c.q)), c.r.title, c.r.tags, c.r.ingredients, c.r.description))
		if got != c.want {
			t.Errorf("%q in %q:\n got  %s\n want %s", c.q, c.r.title, got, c.want)
		}
	}
}

func TestExcerptCutsLongDescriptions(t *testing.T) {
	desc := "Ein Rezept aus der Familie, das wir seit Jahren am Sonntag kochen, mit Brot und viel Geduld, bis der Teig weich ist und die Küche duftet."
	got := render(matchCard(matchWords(searchWords("brot")), "Sonntag", nil, nil, desc))
	want := "description | description: … [Brot] und viel Geduld, bis der Teig weich ist und die Küche …"
	if got != want {
		t.Fatalf("\n got  %s\n want %s", got, want)
	}
}

// A space wider than a byte before the find - a no-break space pasted from
// the web - must not leave the excerpt starting inside it.
func TestExcerptStartsAfterAWideSpace(t *testing.T) {
	got := render(matchCard(matchWords(searchWords("brot")), "Sonntag", nil, nil, "Dazu Brotscheiben reichen."))
	want := "description | description: … [Brot]scheiben reichen."
	if got != want {
		t.Fatalf("\n got  %s\n want %s", got, want)
	}
}
