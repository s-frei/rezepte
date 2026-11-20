package db

import "testing"

func TestFold(t *testing.T) {
	cases := map[string]string{
		"Käse": "kase", "Kaese": "kase", "kase": "kase",
		"Süßkartoffel": "suskartoffel", "Süsskartoffel": "suskartoffel", "suesskartoffel": "suskartoffel",
		"Äpfel": "apfel", "ÖL": "ol", "Brot": "brot",
		// Decomposed, as pasted from a PDF: "a" and a combining diaeresis.
		"Ka\u0308se": "kase",
		// The letter meets its spelled-out form next to the same letter.
		"Schloßsalat": "schlossalat", "Schlosssalat": "schlossalat",
		// Every other diacritic goes, as the tokenizers drop it.
		"Crème fraîche": "creme fraiche",
	}
	for in, want := range cases {
		if got := Fold(in); got != want {
			t.Fatalf("Fold(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestFoldInSQL holds the SQL function the index writers and migration 0030
// call to the Go one a search folds with.
func TestFoldInSQL(t *testing.T) {
	conn, _ := openAt(t, 1)
	var got string
	if err := conn.QueryRowContext(t.Context(), `SELECT fold(?)`, "Süßkartoffel mit Käse").Scan(&got); err != nil {
		t.Fatal(err)
	}
	if want := Fold("Süßkartoffel mit Käse"); got != want {
		t.Fatalf("fold() = %q, want %q", got, want)
	}
}

// TestFoldSpans holds each folded rune to the source bytes it stands for,
// which is what marks a find in the text a reader sees.
func TestFoldSpans(t *testing.T) {
	f := FoldSpans("Süßes Brötchen")
	if got := string(f.Runes); got != "suses brotchen" {
		t.Fatalf("runes = %q", got)
	}
	at := func(i int) string { return f.Source[f.Spans[i][0]:f.Spans[i][1]] }
	// "ü" folds to "u", "ß" (spelled "ss", shortened) to "s", "ö" to "o".
	for i, want := range map[int]string{1: "ü", 2: "ß", 8: "ö"} {
		if got := at(i); got != want {
			t.Fatalf("span %d = %q, want %q", i, got, want)
		}
	}
	// A spelled-out form shortened in the source spans both letters.
	g := FoldSpans("Kaese")
	if got := g.Source[g.Spans[1][0]:g.Spans[1][1]]; got != "ae" {
		t.Fatalf("span = %q, want ae", got)
	}
}
