package pdf

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

const maxNameBytes = 120

// FileName turns a recipe title into a file name that is safe on every
// common file system: no `/\:*?"<>|`, control or invisible format characters
// (bidi overrides), whitespace runs collapsed, at most 120 bytes, no leading
// or trailing dots and spaces. An empty result becomes fallback, cleaned the
// same way, then "recipe". taken records the names handed out (case folded,
// as macOS and Windows compare them), so a repeat gets " (2)", " (3)".
func FileName(title, fallback string, taken map[string]int) string {
	base := cleanName(title)
	if base == "" {
		base = cleanName(fallback)
	}
	if base == "" {
		base = "recipe"
	}
	name := base
	for n := 2; taken[strings.ToLower(name)] > 0; n++ {
		name = fmt.Sprintf("%s (%d)", base, n)
	}
	taken[strings.ToLower(name)]++
	return name + ".pdf"
}

func cleanName(s string) string {
	s = strings.Join(strings.Fields(strings.Map(func(r rune) rune {
		// The zero-width joiner stays: it holds emoji like 👨‍🍳 together.
		if strings.ContainsRune(`/\:*?"<>|`, r) || unicode.IsControl(r) && !unicode.IsSpace(r) ||
			unicode.Is(unicode.Cf, r) && r != '\u200d' {
			return -1
		}
		return r
	}, s)), " ")
	if len(s) > maxNameBytes {
		cut := maxNameBytes
		for !utf8.RuneStart(s[cut]) {
			cut--
		}
		s = s[:cut]
	}
	return strings.Trim(s, " .")
}
