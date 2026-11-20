package ingredient

import (
	"strings"
	"unicode/utf8"
)

// IsWater reports whether a key is plain water, which never counts as missing.
func IsWater(key string) bool { return key == "wasser" || key == "water" }

// Covers reports whether a pantry ingredient (have) serves a recipe
// ingredient (need). Both arguments are keys as returned by Key. Names match
// when they are equal, when one is the last word of the other, or when one
// single word of at least four runes ends the other's last word
// (Gurke/Salatgurke, Essig/Rotweinessig).
//
// ponytail: suffix matching on compound words, not a decompounder; false
// positives like Reis/Preis are possible. Add an exclusion list if that bites.
func Covers(have, need string) bool {
	if have == need {
		return true
	}
	a, b := lastWord(have), lastWord(need)
	return b == have || a == need ||
		(strings.HasSuffix(b, have) && oneLongWord(have)) ||
		(strings.HasSuffix(a, need) && oneLongWord(need))
}

func lastWord(key string) string {
	w := strings.FieldsFunc(key, func(r rune) bool { return r == ' ' || r == '-' })
	if len(w) == 0 {
		return ""
	}
	return w[len(w)-1]
}

func oneLongWord(key string) bool {
	return utf8.RuneCountInString(key) >= 4 && !strings.ContainsAny(key, " -")
}
