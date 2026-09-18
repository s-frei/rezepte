package recipe

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

// samplesJSON is embedded so demo mode can seed the same recipes the tests
// use without shipping a separate copy of the file.
//
//go:embed testdata/recipes.json
var samplesJSON []byte

// Samples returns the twelve German sample recipes shared by the recipe
// tests, the frontend e2e suite and demo mode. Every call parses the
// embedded JSON afresh, so callers may modify the result.
func Samples() ([]Input, error) {
	var in []Input
	if err := json.Unmarshal(samplesJSON, &in); err != nil {
		return nil, fmt.Errorf("parse sample recipes: %w", err)
	}
	return in, nil
}
