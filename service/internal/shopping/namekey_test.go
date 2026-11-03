package shopping_test

import (
	"strings"
	"testing"

	"github.com/s-frei/rezepte/service/internal/shopping"
)

func TestNameKeyMergesCaseSpaceAndPlural(t *testing.T) {
	for _, in := range []string{"Zwiebel", "Zwiebeln", " zwiebeln ", "ZWIEBEL"} {
		if got := shopping.NameKey(in); got != shopping.NameKey("zwiebel") {
			t.Errorf("%q → %q", in, got)
		}
	}
	if shopping.NameKey("onions") != shopping.NameKey("onion") {
		t.Error("english plural")
	}
	if shopping.NameKey("Ei") != shopping.NameKey("Eier") {
		t.Error("ei/eier")
	}
	if shopping.NameKey("Reis") == shopping.NameKey("Rei") {
		t.Error("reis must not lose its s")
	}
}

func TestNameKeyPlurals(t *testing.T) {
	same := [][2]string{{"Linsen", "Linse"}, {"Nudeln", "Nudel"}, {"Gurken", "Gurke"}, {"Tomaten", "Tomate"}, {"Kartoffeln", "Kartoffel"},
		{"Erbsen", "Erbse"}, {"Pilze", "Pilz"}, {"Äpfel", "Apfel"}, {"Nüsse", "Nuss"}, {"Eiern", "Ei"}, {"tomatoes", "tomato"}}
	for _, p := range same {
		if shopping.NameKey(p[0]) != shopping.NameKey(p[1]) {
			t.Errorf("%s → %q, %s → %q", p[0], shopping.NameKey(p[0]), p[1], shopping.NameKey(p[1]))
		}
	}
	for _, w := range []string{"glass", "asparagus", "Reis", "Mais", "Kürbis", "hummus", "Ananas"} {
		if got := shopping.NameKey(w); got != strings.ToLower(w) {
			t.Errorf("%s → %q, want unchanged", w, got)
		}
	}
}

func TestNameKeyFoldsAccentsButKeepsUmlauts(t *testing.T) {
	if shopping.NameKey("Crème fraîche") != shopping.NameKey("creme fraiche") {
		t.Error("accents must fold")
	}
	if shopping.NameKey("Mehl") == shopping.NameKey("Möhl") || shopping.NameKey("Öl") != "öl" {
		t.Error("umlauts must stay")
	}
}
