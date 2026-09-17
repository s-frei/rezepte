package recipe

import (
	"slices"
	"strings"
	"testing"
)

// TestVariants covers the ASCII/umlaut spelling pairs ftsQuery relies on:
// the FTS5 tokenizer's remove_diacritics option folds "ä" to "a", not to
// "ae", so an ASCII query term and its umlaut form would otherwise never
// match the same indexed token.
func TestVariants(t *testing.T) {
	cases := map[string][]string{
		"kaese":   {"kaese", "käse"},
		"röst":    {"röst", "roest"},
		"spätzle": {"spätzle", "spaetzle"},
		"abc":     {"abc"},
	}
	for term, want := range cases {
		got := variants(term)
		if len(got) != len(want) {
			t.Fatalf("variants(%q) = %v, want %v", term, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("variants(%q) = %v, want %v", term, got, want)
			}
		}
	}

	// variants requires lower-cased input (see its doc comment); ftsQuery
	// enforces that with strings.ToLower before calling it. These cases
	// exercise that pairing end-to-end for capitalised/upper-case terms
	// that, before that fix, never produced their umlaut variant because
	// the replacers only match lower-case patterns.
	caseInsensitive := map[string][]string{
		"Aepfel": {"äpfel", "aepfel"},
		"KAESE":  {"käse"},
		"Röst":   {"roest"},
	}
	for term, want := range caseInsensitive {
		got := variants(strings.ToLower(term))
		for _, w := range want {
			if !slices.Contains(got, w) {
				t.Fatalf("variants(strings.ToLower(%q)) = %v, want to contain %q", term, got, w)
			}
		}
	}
}

func TestFtsQueryEmpty(t *testing.T) {
	for _, q := range []string{"", "   ", "\t\n"} {
		if got := ftsQuery(q); got != "" {
			t.Fatalf("ftsQuery(%q) = %q, want empty", q, got)
		}
	}
}

func TestFtsQuerySimpleTerm(t *testing.T) {
	if got, want := ftsQuery("kapern"), `"kapern"*`; got != want {
		t.Fatalf("ftsQuery(kapern) = %q, want %q", got, want)
	}
}

func TestFtsQueryVariantGroup(t *testing.T) {
	if got, want := ftsQuery("kaese"), `("kaese"* OR "käse"*)`; got != want {
		t.Fatalf("ftsQuery(kaese) = %q, want %q", got, want)
	}
}

func TestFtsQueryMultipleTerms(t *testing.T) {
	got := ftsQuery("spätzle röst")
	want := `("spätzle"* OR "spaetzle"*) AND ("röst"* OR "roest"*)`
	if got != want {
		t.Fatalf("ftsQuery(spätzle röst) = %q, want %q", got, want)
	}
}

func TestFtsQueryEscapesQuotes(t *testing.T) {
	if got, want := ftsQuery(`a"b`), `"a""b"*`; got != want {
		t.Fatalf("ftsQuery = %q, want %q", got, want)
	}
}
