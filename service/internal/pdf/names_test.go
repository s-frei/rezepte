package pdf

import (
	"strings"
	"testing"
)

func TestFileName(t *testing.T) {
	taken := map[string]int{}
	for _, c := range []struct{ title, want string }{
		{`Käse/Spätzle: "neu"?`, "KäseSpätzle neu.pdf"},
		{"🍝", "🍝.pdf"},
		{"Flammkuchen", "Flammkuchen.pdf"},
		{"flammkuchen", "flammkuchen (2).pdf"}, // case-insensitive file systems
		{"Flammkuchen (3)", "Flammkuchen (3).pdf"},
		{"Flammkuchen", "Flammkuchen (4).pdf"},
		{"", "Rezept.pdf"},
		{" \t*?\x00 ", "Rezept (2).pdf"},
		{"a\n\n  b\tc", "a b c.pdf"},
		{"..hidden.", "hidden.pdf"},
		{"Pasta\u202eflow", "Pastaflow.pdf"},
		{"👨\u200d🍳", "👨\u200d🍳.pdf"},
	} {
		if got := FileName(c.title, "Rezept", taken); got != c.want {
			t.Errorf("FileName(%q) = %q, want %q", c.title, got, c.want)
		}
	}
}

func TestFileNameCleansFallback(t *testing.T) {
	for _, c := range []struct{ fallback, want string }{
		{"../x", "x.pdf"},
		{"", "recipe.pdf"},
		{"..", "recipe.pdf"},
	} {
		got := FileName("", c.fallback, map[string]int{})
		if got != c.want || strings.HasPrefix(got, ".") || strings.ContainsAny(got, `/\`) {
			t.Errorf("FileName(\"\", %q) = %q, want %q", c.fallback, got, c.want)
		}
	}
}

func TestFileNameTrimsOnRuneBoundary(t *testing.T) {
	got := FileName(strings.Repeat("ä", 100), "x", map[string]int{})
	base := strings.TrimSuffix(got, ".pdf")
	if len(base) != 120 || base != strings.Repeat("ä", 60) {
		t.Errorf("base = %d bytes %q, want 60 × ä", len(base), base)
	}
}
