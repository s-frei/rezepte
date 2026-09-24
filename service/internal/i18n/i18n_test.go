package i18n

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLocalesJSONParses(t *testing.T) {
	if _, err := parse(localesJSON); err != nil {
		t.Fatal(err)
	}
}

func TestBaseLocaleIsEnglish(t *testing.T) {
	// English is the instance default an operator gets without configuring
	// anything, and the language the user docs are shot in.
	if got := BaseLocale(); got != "en" {
		t.Errorf("BaseLocale() = %q, want en", got)
	}
}

func TestParseRejectsABaseLocaleOutsideTheSet(t *testing.T) {
	if _, err := parse([]byte(`{"baseLocale":"fr","locales":["en","de"]}`)); err == nil {
		t.Fatal("parse accepted a baseLocale that is not in locales")
	}
}

func TestParseRejectsAnEmptySet(t *testing.T) {
	if _, err := parse([]byte(`{"baseLocale":"en","locales":[]}`)); err == nil {
		t.Fatal("parse accepted an empty locale set")
	}
}

func TestLocalesReturnsACopy(t *testing.T) {
	a := Locales()
	a[0] = "changed"
	if Locales()[0] == "changed" {
		t.Fatal("Locales shares its backing array with the caller")
	}
}

// A language in the settings without a catalog would compile: Paraglide
// falls back to the base locale for every missing message, so the interface
// would silently render English under another language's name.
func TestEveryLocaleHasACatalog(t *testing.T) {
	for _, l := range Locales() {
		// The tests run in service/internal/i18n, three levels below the
		// repository root.
		path := filepath.Join("..", "..", "..", "frontend", "messages", l+".json")
		if _, err := os.Stat(path); err != nil {
			t.Errorf("no message catalog for %q: %v", l, err)
		}
	}
}
