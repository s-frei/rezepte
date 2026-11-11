package recipeimport

import (
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/s-frei/rezepte/service/internal/recipe"
)

var fractions = map[rune]float64{
	'½': 0.5, '⅓': 1.0 / 3, '⅔': 2.0 / 3, '¼': 0.25, '¾': 0.75,
	'⅕': 0.2, '⅛': 0.125, '⅜': 0.375, '⅝': 0.625, '⅞': 0.875,
}

const fracClass = `½⅓⅔¼¾⅕⅛⅜⅝⅞`

// number: "1 1/2", "1 ½", "1/2", "1½", "1,5", "1.5", "2", "½".
const number = `(?:\d+\s+\d+/\d+|\d+\s+[` + fracClass + `]|\d+/\d+|\d+[` + fracClass + `]|\d+(?:[.,]\d+)?|[` + fracClass + `])`

// parseLine splits one ingredient line with splitLine and judges the
// split. sure is false when the member should look at the result: a number
// that is not at the start, a vague amount like "to taste" in the name (in
// a note it is said where the cook sees it: "Salz, nach Geschmack"), or a
// name that cannot be right (see plausibleName). An unsure line without a
// quantity, or without a name, is kept whole.
// parseLine("1 EL, gehäuft Salz (zum Entwässern)", german) =
// {1 EL Salz, note "gehäuft, zum Entwässern"}, true.
func parseLine(line string, lang *language) (recipe.Ingredient, bool) {
	line = strings.Join(strings.Fields(line), " ")
	if line == "" {
		return recipe.Ingredient{}, false
	}
	ing, _ := splitLine(line, lang)
	sure := plausibleName(ing.Name)
	if ing.Quantity == nil && strings.IndexFunc(line, unicode.IsDigit) >= 0 {
		sure = false
	}
	if lang.patterns().vague.MatchString(ing.Name) {
		sure = false
	}
	if !sure && (ing.Quantity == nil || ing.Name == "") {
		// Unsure with nothing split off, or with no name left: keep the line
		// whole so nothing is lost (Build drops ingredients without a name).
		ing, _ = clampIngredient(recipe.Ingredient{Name: line})
	}
	return ing, sure
}

// splitLine splits one line into quantity, unit, name and note, reading
// every word from lang, and passes no verdict. In order: plural markers go,
// then the quantity (a range keeps the range as note), a parenthesis right
// after it ("1 (14-ounce) can"), the unit (one word or two: "fl oz"), a
// parenthesis right after that ("1 can (14 oz) tomatoes"), a unit modifier
// ("gehäuft"), and a trailing note in balanced parentheses or after a comma
// (see splitTrailingNote).
// splitLine("2 l Milch, 1,5 %", german) = {2 l Milch, note "1,5 %"}.
func splitLine(line string, lang *language) (recipe.Ingredient, bool) {
	re := lang.patterns()
	line = strings.Join(strings.Fields(line), " ")
	var ing recipe.Ingredient
	var notes []string
	rest := stripPluralMarkers(line, lang)
	m := re.quantity.FindStringSubmatchIndex(rest)
	var q float64
	if m != nil {
		var ok bool
		if q, ok = readQuantity(rest[m[2]:m[3]], lang); !ok {
			m = nil // "0 g" or 400 digits: no amount, the line stays text
		}
	}
	if m != nil {
		ing.Quantity = &q
		if m[4] >= 0 { // a range: lower bound as quantity, the range as note
			notes = append(notes, rest[m[0]:m[1]])
		}
		rest = strings.TrimSpace(rest[m[1]:])
		var note string
		note, rest = splitLeadingNote(rest)
		notes = append(notes, note)
		if unit, after, ok := leadingUnit(rest, re.units); ok {
			ing.Unit = &unit
			note, rest = splitLeadingNote(strings.TrimSpace(after)) // "1 can (14 oz) tomatoes"
			notes = append(notes, note)
			note, rest = splitUnitModifier(rest, lang)
			notes = append(notes, note)
		}
	}
	name, note := splitTrailingNote(rest)
	notes = append(notes, note)
	for i, n := range notes {
		notes[i] = strings.TrimLeft(n, " ,;") // WP Recipe Maker writes "(, minced)"
	}
	notes = slices.DeleteFunc(notes, func(n string) bool { return n == "" })
	if len(notes) > 0 {
		joined := strings.Join(notes, ", ")
		ing.Note = &joined
	}
	ing.Name = name
	return clampIngredient(ing)
}

// leadingUnit takes a unit from units off the start of s, two words before
// one: leadingUnit("fl oz cream", …) = "fl oz", " cream", true.
func leadingUnit(s string, units map[string]bool) (unit, rest string, ok bool) {
	word, after := leadingWord(s)
	if second, after2 := leadingWord(strings.TrimPrefix(after, " ")); strings.HasPrefix(after, " ") && units[strings.ToLower(word+" "+second)] {
		return word + " " + second, after2, true
	}
	return word, after, units[strings.ToLower(word)]
}

// plausibleName is the safety net under every table: a name that is empty,
// starts with punctuation, holds a "/" or has unbalanced parentheses comes
// from a wrong split, whatever the language.
// plausibleName("Knoblauch") = true; plausibleName("/n Knoblauch") = false.
func plausibleName(name string) bool {
	if name == "" || strings.Contains(name, "/") {
		return false
	}
	if r, _ := utf8.DecodeRuneInString(name); unicode.IsPunct(r) || unicode.IsSymbol(r) {
		return false
	}
	depth := 0
	for _, r := range name {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
			if depth < 0 {
				return false
			}
		}
	}
	return depth == 0
}

// splitLeadingNote takes a parenthesis that opens s off as a note, when
// something follows it.
// splitLeadingNote("(14-ounce) can coconut milk") = "14-ounce", "can coconut milk".
func splitLeadingNote(s string) (note, rest string) {
	if !strings.HasPrefix(s, "(") {
		return "", s
	}
	depth := 0
	for i, r := range s {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				if rest := strings.TrimSpace(s[i+1:]); rest != "" {
					return strings.TrimSpace(s[1:i]), rest
				}
				return "", s
			}
		}
	}
	return "", s
}

// splitUnitModifier takes one of lang's unit modifiers off the start of s,
// with or without the comma before it.
// splitUnitModifier(", gehäuft Salz", german) = "gehäuft", "Salz".
func splitUnitModifier(s string, lang *language) (modifier, rest string) {
	m := lang.patterns().unitModifier.FindStringSubmatchIndex(s)
	if m == nil {
		return "", s
	}
	return s[m[2]:m[3]], strings.TrimLeft(s[m[3]:], " ,")
}

// maxQuantity is the largest amount an item takes, as the shopping API's.
const maxQuantity = 100000

// readQuantity reads the matched amount with lang's separators ("1.000,5" is
// 1000.5) and refuses one that is not above 0 and at most maxQuantity: a
// zero means none, and hundreds of digits parse to +Inf, which no JSON
// answer can carry.
func readQuantity(num string, lang *language) (float64, bool) {
	if lang.patterns().thousands.MatchString(num) {
		for _, s := range lang.thousandsSeparators {
			num = strings.ReplaceAll(num, s, "")
		}
		for _, s := range lang.decimalSeparators {
			num = strings.ReplaceAll(num, s, ".")
		}
	}
	q := parseNumber(num)
	return q, q > 0 && q <= maxQuantity
}

func parseNumber(s string) float64 {
	s = strings.ReplaceAll(s, ",", ".")
	if whole, frac, ok := strings.Cut(s, " "); ok { // "1 1/2"
		return parseNumber(whole) + parseNumber(frac)
	}
	if num, den, ok := strings.Cut(s, "/"); ok {
		n, _ := strconv.ParseFloat(num, 64)
		d, _ := strconv.ParseFloat(den, 64)
		if d == 0 {
			return n
		}
		return n / d
	}
	r := []rune(s)
	if f, ok := fractions[r[len(r)-1]]; ok { // "1½" or "½"
		whole, _ := strconv.ParseFloat(string(r[:len(r)-1]), 64)
		return whole + f
	}
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

// leadingWord returns the first word of s (letters and dots) and the rest.
// "g Mehl" -> "g", " Mehl"; "Mehl" -> "Mehl", "".
func leadingWord(s string) (string, string) {
	end := strings.IndexFunc(s, func(r rune) bool { return !unicode.IsLetter(r) && r != '.' })
	if end < 0 {
		return s, ""
	}
	return s[:end], s[end:]
}

// splitTrailingNote takes a note off a name: the outermost balanced "(…)"
// that ends it, else everything after the first comma that is not between
// two digits ("Milch 1,5 %" keeps its decimal comma). A parenthesis that is
// the whole of s, or does not balance, stays in the name.
// splitTrailingNote("Chiliflocken (koreanische (Gochugaru))") =
// "Chiliflocken", "koreanische (Gochugaru)"; splitTrailingNote("Mehl, gesiebt") = "Mehl", "gesiebt".
func splitTrailingNote(s string) (name, note string) {
	if strings.HasSuffix(s, ")") {
		depth := 0
		for i := len(s) - 1; i >= 0; i-- {
			switch s[i] {
			case ')':
				depth++
			case '(':
				depth--
			}
			if depth == 0 {
				if i > 0 {
					return strings.TrimSpace(s[:i]), strings.TrimSpace(s[i+1 : len(s)-1])
				}
				break
			}
		}
	}
	for i := 0; i < len(s); i++ {
		if s[i] == ',' && (i == 0 || i == len(s)-1 || !isDigit(s[i-1]) || !isDigit(s[i+1])) {
			return strings.TrimSpace(s[:i]), strings.TrimSpace(s[i+1:])
		}
	}
	return s, ""
}

func isDigit(b byte) bool { return '0' <= b && b <= '9' }
