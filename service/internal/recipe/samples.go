package recipe

import (
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/s-frei/rezepte/service/internal/user"
)

// The sample sets are embedded so demo mode can seed the same recipes the
// tests use without shipping a separate copy of either file.
//
//go:embed testdata/recipes.de.json
var samplesDE []byte

//go:embed testdata/recipes.en.json
var samplesEN []byte

// Samples returns the twelve sample recipes of one language, shared by the
// recipe tests, the frontend e2e suite and demo mode. The two sets are
// different recipes rather than translations of each other: a demo instance
// shows what someone would actually cook, and nothing in the app translates
// recipe content. Every call parses the embedded JSON afresh, so callers may
// modify the result.
func Samples(locale user.Locale) ([]Input, error) {
	var raw []byte
	switch locale {
	case "de":
		raw = samplesDE
	case "en":
		raw = samplesEN
	default:
		return nil, fmt.Errorf("no sample recipes for locale %q", locale)
	}
	var in []Input
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, fmt.Errorf("parse sample recipes for %q: %w", locale, err)
	}
	return in, nil
}
