package ingredient_test

import (
	"strings"
	"testing"

	"github.com/s-frei/rezepte/service/internal/ingredient"
)

func TestKeyMergesCaseSpaceAndPlural(t *testing.T) {
	for _, in := range []string{"Zwiebel", "Zwiebeln", " zwiebeln ", "ZWIEBEL"} {
		if got := ingredient.Key(in); got != ingredient.Key("zwiebel") {
			t.Errorf("%q → %q", in, got)
		}
	}
	if ingredient.Key("onions") != ingredient.Key("onion") {
		t.Error("english plural")
	}
	if ingredient.Key("Ei") != ingredient.Key("Eier") {
		t.Error("ei/eier")
	}
	if ingredient.Key("Reis") == ingredient.Key("Rei") {
		t.Error("reis must not lose its s")
	}
}

func TestKeyPlurals(t *testing.T) {
	same := [][2]string{{"Linsen", "Linse"}, {"Nudeln", "Nudel"}, {"Gurken", "Gurke"}, {"Tomaten", "Tomate"}, {"Kartoffeln", "Kartoffel"},
		{"Erbsen", "Erbse"}, {"Pilze", "Pilz"}, {"Äpfel", "Apfel"}, {"Nüsse", "Nuss"}, {"Eiern", "Ei"}, {"tomatoes", "tomato"}}
	for _, p := range same {
		if ingredient.Key(p[0]) != ingredient.Key(p[1]) {
			t.Errorf("%s → %q, %s → %q", p[0], ingredient.Key(p[0]), p[1], ingredient.Key(p[1]))
		}
	}
	for _, w := range []string{"glass", "asparagus", "Reis", "Mais", "Kürbis", "hummus", "Ananas"} {
		if got := ingredient.Key(w); got != strings.ToLower(w) {
			t.Errorf("%s → %q, want unchanged", w, got)
		}
	}
}

func TestKeyFoldsAccentsButKeepsUmlauts(t *testing.T) {
	if ingredient.Key("Crème fraîche") != ingredient.Key("creme fraiche") {
		t.Error("accents must fold")
	}
	if ingredient.Key("Mehl") == ingredient.Key("Möhl") || ingredient.Key("Öl") != "öl" {
		t.Error("umlauts must stay")
	}
}
