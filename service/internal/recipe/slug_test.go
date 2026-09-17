package recipe

import "testing"

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Käsespätzle":                           "kaesespaetzle",
		"Rote Grütze mit Vanillesoße":           "rote-gruetze-mit-vanillesosse",
		"  Schweineschnitzel & Bratkartoffeln ": "schweineschnitzel-bratkartoffeln",
		"Crème brûlée":                          "creme-brulee",
		"!!!":                                   "",
	}
	for in, want := range cases {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeTags(t *testing.T) {
	got := NormalizeTags([]string{" Fleisch", "fleisch", "", "Süß ", "klassiker"})
	want := []string{"fleisch", "süß", "klassiker"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}
