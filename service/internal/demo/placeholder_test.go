package demo

import (
	"bytes"
	"crypto/sha256"
	"image/jpeg"
	"testing"

	"github.com/s-frei/rezepte/service/internal/recipe"
)

func TestPlaceholderIsDeterministic(t *testing.T) {
	a, err := Placeholder(0, "Königsberger Klopse")
	if err != nil {
		t.Fatalf("Placeholder: %v", err)
	}
	b, err := Placeholder(0, "Königsberger Klopse")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Fatal("same input produced different bytes")
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(a))
	if err != nil {
		t.Fatalf("not a JPEG: %v", err)
	}
	if cfg.Width != 1200 || cfg.Height != 900 {
		t.Fatalf("size = %dx%d, want 1200x900", cfg.Width, cfg.Height)
	}
}

func TestPlaceholderVariesByRecipe(t *testing.T) {
	a, _ := Placeholder(0, "Königsberger Klopse")
	b, _ := Placeholder(1, "Rinderrouladen")
	c, _ := Placeholder(3, "Käsespätzle") // same tint rotation and initial as index 0, one shade darker
	if bytes.Equal(a, b) || bytes.Equal(a, c) {
		t.Fatal("different recipes produced identical placeholders")
	}
}

func TestInitialUsesTheFirstLetterUppercased(t *testing.T) {
	for in, want := range map[string]string{
		"Königsberger Klopse": "K",
		"  rote Grütze":       "R",
		"Übrig":               "Ü",
		"":                    "?",
	} {
		if got := initial(in); got != want {
			t.Errorf("initial(%q) = %q, want %q", in, got, want)
		}
	}
}

// Several sample recipes share an initial (Königsberger Klopse, Käsespätzle,
// Kartoffelsalat) and would collide if the tint only rotated, leaving the demo
// catalogue with repeated tiles.
func TestEverySampleGetsItsOwnPlaceholder(t *testing.T) {
	samples, err := recipe.Samples()
	if err != nil {
		t.Fatalf("Samples: %v", err)
	}
	seen := make(map[[sha256.Size]byte]int, len(samples))
	for i, s := range samples {
		img, err := Placeholder(i, s.Title)
		if err != nil {
			t.Fatalf("Placeholder(%d, %q): %v", i, s.Title, err)
		}
		key := sha256.Sum256(img)
		if prev, dup := seen[key]; dup {
			t.Errorf("%q (index %d) has the same placeholder as index %d", s.Title, i, prev)
		}
		seen[key] = i
	}
}
