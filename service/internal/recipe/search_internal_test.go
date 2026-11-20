package recipe

import (
	"slices"
	"testing"

	"github.com/s-frei/rezepte/service/internal/db/sqlc"
)

func TestSearchTermsEmpty(t *testing.T) {
	for _, q := range []string{"", "   ", "\t\n", `"-,`} {
		if got := searchTerms(searchWords(q)); got != nil {
			t.Fatalf("searchTerms(%q) = %+v, want nil", q, got)
		}
	}
}

func TestSearchTerms(t *testing.T) {
	cases := map[string][]sqlc.SearchTerm{
		"kapern": {{Exact: `"kapern"`, Prefix: `"kaper"*`, Trigram: `"kaper"`}},
		// Folded the way the indexes hold their text.
		"Käse":   {{Exact: `"kase"`, Prefix: `"kase"*`, Trigram: `"kase"`}},
		"KAESE":  {{Exact: `"kase"`, Prefix: `"kase"*`, Trigram: `"kase"`}},
		"Äpfeln": {{Exact: `"apfeln"`, Prefix: `"apfel"*`, Trigram: `"apfel"`}},
		// Too short for a trigram: found only at the start of a word.
		"Ei":  {{Exact: `"ei"`, Prefix: `"ei"*`}},
		"oel": {{Exact: `"ol"`, Prefix: `"ol"*`}},
		// Punctuation splits words and never reaches a quoted string.
		`brot, "a"b`: {
			{Exact: `"brot"`, Prefix: `"brot"*`, Trigram: `"brot"`},
			{Exact: `"a"`, Prefix: `"a"*`},
			{Exact: `"b"`, Prefix: `"b"*`},
		},
	}
	for q, want := range cases {
		if got := searchTerms(searchWords(q)); !slices.Equal(got, want) {
			t.Fatalf("searchTerms(%q) = %+v, want %+v", q, got, want)
		}
	}
}

func TestStem(t *testing.T) {
	cases := map[string]string{
		"tomaten": "tomat", "kartoffeln": "kartoffel", "zwiebeln": "zwiebel",
		"sahne": "sahn", "brot": "brot", "eier": "eier", "apfel": "apfel",
	}
	for word, want := range cases {
		if got := stem(word); got != want {
			t.Fatalf("stem(%q) = %q, want %q", word, got, want)
		}
	}
}

// A word typed twice, or in two forms of one stem, is searched once: it
// would score twice and show two identical finds.
func TestSearchWordsDropsRepeats(t *testing.T) {
	if got, want := searchWords("zwiebel Zwiebeln brot zwiebel"), []string{"zwiebel", "brot"}; !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
