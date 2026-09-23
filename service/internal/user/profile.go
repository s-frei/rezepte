package user

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/danielgtaylor/huma/v2"
)

// Color is a palette token identifying a person wherever the UI names them.
// It is a token name, never a colour value: the values live in
// frontend/src/app.css, which is the only place that decides what "sage"
// looks like in either theme.
type Color string

// Colors is the palette, in the order the picker shows it and the order
// ColorUsage returns. This slice owns that order; app.css and
// frontend/src/lib/user/color.ts restate the names, never the order's
// meaning.
var Colors = []Color{"amber", "clay", "rose", "plum", "sage", "olive", "teal", "slate"}

// ColorCount is one palette entry and how many accounts hold it.
type ColorCount struct {
	Color Color `json:"color" doc:"Palette token"`
	Count int   `json:"count" doc:"How many accounts hold it"`
}

// maxDisplayName is counted in runes, not bytes: a name of umlauts is as
// long as a name of ASCII to the person typing it.
const maxDisplayName = 64

// ParseColor accepts exactly the palette, exactly as written. There is no
// trimming and no case folding - the value comes from a closed set the API
// declares as an enum, so anything else is a client error rather than a
// typo to be repaired.
func ParseColor(s string) (Color, error) {
	for _, c := range Colors {
		if Color(s) == c {
			return c, nil
		}
	}
	return "", ErrInvalidColor
}

// Locale is the interface language of one account. It is a display choice
// and nothing else: recipe content is stored in whatever language it was
// written in and is never translated.
type Locale string

// Locales is the set of interface languages, in the order the picker shows
// them. This slice owns that set: config validates against it, and the CHECK
// constraint on users.locale in 0001_users_and_sessions.sql repeats it in the
// only other place it has to exist. The first entry is the base locale.
var Locales = []Locale{"en", "de"}

// Schema makes Locales the enum every API field of this type carries, so a
// language is added to one slice instead of to that slice and to an
// `enum:"..."` tag on each request and response field. A tag left behind
// would compile, pass its tests, and reject the new language at runtime with
// a 422 that neither the database nor ParseLocale agrees with.
//
// Returned by value so both Locale and *Locale fields pick it up: huma looks
// the interface up on the dereferenced type.
func (Locale) Schema(huma.Registry) *huma.Schema {
	values := make([]any, len(Locales))
	for i, l := range Locales {
		values[i] = string(l)
	}
	return &huma.Schema{Type: huma.TypeString, Enum: values}
}

// ParseLocale accepts exactly the set above, exactly as written. There is no
// case folding and no BCP 47 parsing: the value reaches a CHECK constraint
// and a Paraglide locale id, and both want one of two literals.
func ParseLocale(s string) (Locale, error) {
	for _, l := range Locales {
		if Locale(s) == l {
			return l, nil
		}
	}
	return "", ErrInvalidLocale
}

// normalizeDisplayName produces the value the column stores. An empty name
// falls back to the username, which is what lets the column be NOT NULL and
// why no client ever has to fall back itself: the API always sends something
// printable.
func normalizeDisplayName(displayName, username string) (string, error) {
	name := strings.TrimSpace(displayName)
	if name == "" {
		name = strings.TrimSpace(username)
	}
	if utf8.RuneCountInString(name) > maxDisplayName {
		return "", ErrDisplayNameTooLong
	}
	for _, r := range name {
		// A newline in a name breaks every layout that shows it on one line,
		// and a control character is never something anyone meant to type.
		if unicode.IsControl(r) {
			return "", ErrInvalidDisplayName
		}
	}
	return name, nil
}

// fillPalette turns the grouped counts of the users table into one entry per
// palette colour, in palette order, zeros included. The query cannot invent
// rows for colours nobody holds, so they are filled in here.
func fillPalette(rows map[Color]int) []ColorCount {
	out := make([]ColorCount, len(Colors))
	for i, c := range Colors {
		out[i] = ColorCount{Color: c, Count: rows[c]}
	}
	return out
}

// leastUsed returns the colour the fewest accounts hold, ties broken by
// palette order. Least-used rather than first-free because duplicates are
// allowed, so the two differ once every colour is taken, and spreading the
// household out is the property worth keeping. counts must come from
// fillPalette, so it is complete and in palette order.
func leastUsed(counts []ColorCount) Color {
	best := counts[0]
	for _, c := range counts[1:] {
		if c.Count < best.Count {
			best = c
		}
	}
	return best.Color
}
