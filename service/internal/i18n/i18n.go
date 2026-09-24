// Package i18n owns the set of interface languages.
//
// The set is written down exactly once, in locales.json beside this file,
// and both sides read it: this package embeds it, and frontend/vite.config.ts
// generates Paraglide's inlang settings (frontend/project.inlang/settings.json)
// from it before Paraglide compiles the frontend. It lives in the Go module
// because go:embed cannot leave it, and it holds nothing but the languages -
// the rest of the inlang settings is frontend build configuration and stays
// in vite.config.ts.
//
// Adding a language is therefore that file plus its catalog in
// frontend/messages/; nothing in Go changes.
package i18n

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"slices"
)

//go:embed locales.json
var localesJSON []byte

// settings mirrors locales.json, whose keys are named after the inlang
// settings fields they become.
type settings struct {
	BaseLocale string   `json:"baseLocale"`
	Locales    []string `json:"locales"`
}

var parsed = mustParse(localesJSON)

// Locales returns the interface languages in the order locales.json lists
// them, which is the order the language picker shows. The caller gets its own
// copy.
func Locales() []string {
	return slices.Clone(parsed.Locales)
}

// BaseLocale returns the language everything falls back to: a string missing
// from a catalog, a browser asking for a language the app does not have,
// and an instance whose operator configured none.
func BaseLocale() string {
	return parsed.BaseLocale
}

// mustParse panics on a broken file. It is embedded, so a broken one can only
// come from a commit, and TestLocalesJSONParses fails on that commit first.
func mustParse(raw []byte) settings {
	s, err := parse(raw)
	if err != nil {
		panic("i18n: " + err.Error())
	}
	return s
}

func parse(raw []byte) (settings, error) {
	var s settings
	if err := json.Unmarshal(raw, &s); err != nil {
		return settings{}, fmt.Errorf("parse locales.json: %w", err)
	}
	if len(s.Locales) == 0 {
		return settings{}, fmt.Errorf("locales.json lists no locales")
	}
	if !slices.Contains(s.Locales, s.BaseLocale) {
		return settings{}, fmt.Errorf("locales.json: baseLocale %q is not in locales %v", s.BaseLocale, s.Locales)
	}
	return s, nil
}
