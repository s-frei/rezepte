package shopping

import (
	"regexp"
	"strconv"
	"strings"
)

// freeUnits are the editor's suggested units (frontend/messages
// editor_unit_options, both languages) plus their plural forms.
const freeUnits = `g|kg|ml|l|TL|EL|Prisen?|Stücke?|Bunde?|Bünde|Zehen?|Dosen?|Packungen?|tsp|tbsp|tablespoons?|teaspoons?|cups?|fl oz|oz|lbs?|pinch(?:es)?|pieces?`

// freeRe matches "2 l Milch" or "500g Mehl": amount, optional unit, name. A
// unit may touch the amount, but a name may not ("3Zitronen" stays a name).
var freeRe = regexp.MustCompile(`(?i)^(\d+(?:[.,]\d+)?)(?:\s*(` + freeUnits + `)\s+|\s+)(.+)$`)

// noteSepRe is the first ", ", " – " or " - " after the name.
var noteSepRe = regexp.MustCompile(`,\s+|\s+[–-]\s+`)

// trailingNoteRe is a closing "(...)" after a name.
var trailingNoteRe = regexp.MustCompile(`^(.+?)\s*\(([^()]+)\)$`)

// ParseFree splits free text into quantity, unit, name and note. Text without
// a leading amount is all name; the first ", " or " – " after the name starts
// the note, and so does a closing "(...)".
func ParseFree(text string) (quantity *float64, unit *string, name string, note *string) {
	text = strings.TrimSpace(text)
	if m := freeRe.FindStringSubmatch(text); m != nil {
		if q, err := strconv.ParseFloat(strings.Replace(m[1], ",", ".", 1), 64); err == nil {
			quantity = &q
			if m[2] != "" {
				unit = &m[2]
			}
			text = strings.TrimSpace(m[3])
		}
	}
	name = text
	if loc := noteSepRe.FindStringIndex(text); loc != nil && loc[0] > 0 && strings.TrimSpace(text[loc[1]:]) != "" {
		name, note = strings.TrimSpace(text[:loc[0]]), ptr(strings.TrimSpace(text[loc[1]:]))
	} else if m := trailingNoteRe.FindStringSubmatch(text); m != nil {
		name, note = strings.TrimSpace(m[1]), ptr(strings.TrimSpace(m[2]))
	}
	return quantity, unit, name, note
}

func ptr(s string) *string { return &s }
