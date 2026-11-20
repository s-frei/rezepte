package db

import (
	"database/sql/driver"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
	"modernc.org/sqlite"
)

// spelledOut is each German letter's spelled-out form. A letter is spelled
// out first and every spelled-out form shortened after, so the letter meets
// its spelled-out form whatever stands next to it ("Schloßsalat" and
// "Schlosssalat").
var spelledOut = map[rune]string{'ä': "ae", 'ö': "oe", 'ü': "ue", 'ß': "ss"}

// digraphs are the spelled-out forms and the letter each shortens to.
var digraphs = map[[2]rune]rune{{'a', 'e'}: 'a', {'o', 'e'}: 'o', {'u', 'e'}: 'u', {'s', 's'}: 's'}

// Folded is text as the search indexes hold it, with where each folded rune
// came from: Spans[i] is the byte range of Source that Runes[i] stands for.
type Folded struct {
	Source string
	Runes  []rune
	Spans  [][2]int
}

// FoldSpans folds s (see Fold) and keeps, for every folded rune, the byte
// range of the NFC form of s it came from, so a match found in the folded
// text can be marked in the text a reader sees.
func FoldSpans(s string) Folded {
	src := norm.NFC.String(s)
	// Lower-case, strip diacritics and spell out, rune by rune.
	var runes []rune
	var spans [][2]int
	for i, r := range src {
		span := [2]int{i, i + len(string(r))}
		lr := unicode.ToLower(r)
		out, ok := spelledOut[lr]
		if !ok {
			// Any other diacritic goes, as the FTS5 tokenizers drop it too.
			var b strings.Builder
			for _, d := range norm.NFD.String(string(lr)) {
				if !unicode.Is(unicode.Mn, d) {
					b.WriteRune(d)
				}
			}
			out = b.String()
		}
		for _, o := range out {
			runes = append(runes, o)
			spans = append(spans, span)
		}
	}
	// Shorten every spelled-out form, left to right and without overlap.
	f := Folded{Source: src}
	for i := 0; i < len(runes); i++ {
		if i+1 < len(runes) {
			if short, ok := digraphs[[2]rune{runes[i], runes[i+1]}]; ok {
				f.Runes = append(f.Runes, short)
				f.Spans = append(f.Spans, [2]int{spans[i][0], spans[i+1][1]})
				i++
				continue
			}
		}
		f.Runes = append(f.Runes, runes[i])
		f.Spans = append(f.Spans, spans[i])
	}
	return f
}

// Fold is what the search indexes hold and what a search looks for: text
// lower-cased, without diacritics and with the German spellings folded
// together, so "Käse" finds "Kaese" and "Süßkartoffel" finds "Süsskartoffel".
// Folding both sides the same way means a word only meets one it differs
// from by those spellings alone ("Mus" and "muss"). NFC first: text pasted
// from a PDF can carry "ä" as "a" and a combining mark.
func Fold(s string) string {
	return string(FoldSpans(s).Runes)
}

// FoldWords is Fold with everything that is not a letter or a digit cut
// down to one space between the words: "Käse-Spätzle!" is "kase spatzle",
// the words a search is split into.
func FoldWords(s string) string {
	return strings.Join(strings.FieldsFunc(Fold(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}), " ")
}

// fold and fold_words are Fold and FoldWords as SQL functions, registered
// with the driver for every connection this package opens, so the
// statements that write the search indexes and the migration that rebuilt
// them fold with the same code, and a title compares with a search word
// for word.
func init() {
	for name, f := range map[string]func(string) string{"fold": Fold, "fold_words": FoldWords} {
		sqlite.MustRegisterDeterministicScalarFunction(name, 1, func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			if s, ok := args[0].(string); ok {
				return f(s), nil
			}
			return args[0], nil
		})
	}
}
