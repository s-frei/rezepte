package recipe

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"

	"github.com/s-frei/rezepte/service/internal/user"
)

// The sample sets are embedded so demo mode can seed the same recipes the
// tests use without shipping a separate copy of any file.
//
// Matched by pattern rather than named one by one: a language that wants its
// own set drops `recipes.<locale>.json` in beside the others and Samples finds
// it, with no Go change. A language without one is not an error: it gets the
// base locale's set, so translating the interface never requires writing
// twelve recipes first.
//
//go:embed testdata/recipes.*.json
var samples embed.FS

// Samples returns the twelve sample recipes of one language, shared by the
// recipe tests, the frontend e2e suite and demo mode, falling back to the
// base locale (user.BaseLocale) when locale has no set of its own. The sets
// are different recipes rather than translations of each other: a demo
// instance shows what someone would actually cook, and nothing in the app
// translates recipe content. Every call parses the embedded JSON afresh, so
// callers may modify the result.
func Samples(locale user.Locale) ([]Input, error) {
	raw, err := samples.ReadFile(sampleFile(locale))
	if errors.Is(err, fs.ErrNotExist) {
		locale = user.BaseLocale
		raw, err = samples.ReadFile(sampleFile(locale))
	}
	if err != nil {
		return nil, fmt.Errorf("read sample recipes for %q: %w", locale, err)
	}
	var in []Input
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, fmt.Errorf("parse sample recipes for %q: %w", locale, err)
	}
	return in, nil
}

func sampleFile(locale user.Locale) string {
	return fmt.Sprintf("testdata/recipes.%s.json", locale)
}
