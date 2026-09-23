package recipe

import (
	"embed"
	"encoding/json"
	"fmt"

	"github.com/s-frei/rezepte/service/internal/user"
)

// The sample sets are embedded so demo mode can seed the same recipes the
// tests use without shipping a separate copy of any file.
//
// Matched by pattern rather than named one by one: a new language drops
// `recipes.<locale>.json` in beside the others and Samples finds it, with no
// Go change and nothing to forget. TestEveryLocaleHasSampleRecipes in
// internal/user holds the directory to user.Locales, so a missing file fails
// the suite rather than demo mode.
//
//go:embed testdata/recipes.*.json
var samples embed.FS

// Samples returns the twelve sample recipes of one language, shared by the
// recipe tests, the frontend e2e suite and demo mode. The sets are different
// recipes rather than translations of each other: a demo instance shows what
// someone would actually cook, and nothing in the app translates recipe
// content. Every call parses the embedded JSON afresh, so callers may modify
// the result.
func Samples(locale user.Locale) ([]Input, error) {
	raw, err := samples.ReadFile(fmt.Sprintf("testdata/recipes.%s.json", locale))
	if err != nil {
		return nil, fmt.Errorf("no sample recipes for locale %q: %w", locale, err)
	}
	var in []Input
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, fmt.Errorf("parse sample recipes for %q: %w", locale, err)
	}
	return in, nil
}
