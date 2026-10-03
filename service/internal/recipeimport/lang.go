package recipeimport

import (
	"regexp"
	"slices"
	"strings"
	"sync"
)

// language is the table of words the import reads one language with. The
// parser holds no words of its own: every word it looks for comes from a
// table, so adding a language is one lang_<tag>.go file, registered in
// languages below, and its test rows
// (docs/memory/content/howtos/add-an-import-language.mdx).
//
// Every word list is matched case-insensitively; what the cook wrote is kept
// as written. A field a language has no use for stays empty. A new field
// must also be added to combine, or reading with all tables ignores it.
type language struct {
	// tag is the language's primary subtag in lower case, the code the app
	// uses for it in service/internal/i18n/locales.json; e.g. "de". A page
	// that declares "de-DE" or "de_AT" is read with the "de" table. The
	// union of all tables has "".
	tag string

	// units are the unit words cooks write after a quantity, abbreviations
	// with their dot as a separate entry; e.g. "EL", "TL", "Prise", "Pck.".
	// A word after the quantity is the unit only when it is in this list;
	// list the singular and the plural ("Zehe", "Zehen"). Metric symbols
	// ("g", "ml") belong in every table that uses them.
	units []string

	// thousandsSeparators group thousands in a quantity when exactly three
	// digits follow each; e.g. "." ("1.000 g Mehl" is 1000 g). Leave it
	// empty where the same mark is the decimal point ("1.5 cups").
	thousandsSeparators []string

	// decimalSeparators start the fraction after a thousands group; e.g.
	// "," ("1.000,5 g" is 1000.5 g). Only read together with
	// thousandsSeparators; a plain "1,5" or "1.5" needs no entry.
	decimalSeparators []string

	// rangeWords join two quantities into a range, besides the dashes every
	// language shares; e.g. "bis" ("1 bis 2 Zwiebeln"). The lower bound
	// becomes the quantity and the range the note.
	rangeWords []string

	// pluralMarkers are the endings sites glue to a word to say "one or
	// more", removed wherever they follow a letter and end the word; e.g.
	// "(n)" and "/n" turn "Gurke(n)" into "Gurke" and "Zehe/n" into the unit
	// "Zehe". Write them exactly as they appear, parentheses or slash
	// included.
	pluralMarkers []string

	// unitModifiers are words that qualify the unit rather than the
	// ingredient, written right after the unit with or without a comma, and
	// moved to the note; e.g. "gehäuft" ("1 EL, gehäuft Salz" is 1 EL Salz,
	// note "gehäuft"). A phrase ("leicht gehäuft") wins over a word it
	// starts with.
	unitModifiers []string

	// vague are words and phrases that make an amount uncertain, matched as
	// whole words anywhere in the line; e.g. "nach Belieben", "etwas",
	// "n.B.". A line with one is flagged for the cook to check.
	vague []string

	// ingredientHeadings are the lines that open the ingredient list in
	// pasted text, matched as the whole line with or without a trailing
	// colon; e.g. "Zutaten". An entry ending in " …" also matches any words
	// after it: "Zutaten für …" matches "Zutaten für 4 Personen".
	ingredientHeadings []string

	// stepHeadings are the lines that open the steps in pasted text, matched
	// like ingredientHeadings; e.g. "Zubereitung", "Anleitung".
	stepHeadings []string

	// stepWords are the words that may stand before a step's number in
	// pasted text and are dropped with it; e.g. "Schritt" ("Schritt 2: …").
	// A bare "2." or "2)" is a step number in every language.
	stepWords []string

	// promo are patterns for the advertising sentences sites append to a
	// description; a sentence of the description that any of them matches
	// is dropped. Write them as regular expressions with (?i) when case does
	// not matter; e.g. `(?i)^Über \d+ Bewertungen` for Chefkoch's "Über 112
	// Bewertungen und für köstlich befunden.", and `►` for its "Mit ►
	// Portionsrechner ► Kochbuch".
	promo []*regexp.Regexp

	once sync.Once
	re   patterns
}

// patterns are the regular expressions built once from a language's words.
type patterns struct {
	units             map[string]bool
	thousands         *regexp.Regexp
	quantity          *regexp.Regexp
	plural            *regexp.Regexp
	unitModifier      *regexp.Regexp
	vague             *regexp.Regexp
	ingredientHeading *regexp.Regexp
	stepHeading       *regexp.Regexp
	numbered          *regexp.Regexp
}

// languages are the registered tables, one per lang_<tag>.go file.
var languages = []*language{german, english}

// anyLanguage reads with every table at once, for text whose language is
// not known.
var anyLanguage = combine(languages...)

// combine returns one table holding the words of all of ls, with tag "".
// combine(german, english) reads "1 EL Salz" and "1 tbsp salt" alike.
func combine(ls ...*language) *language {
	u := &language{}
	for _, l := range ls {
		u.units = append(u.units, l.units...)
		u.thousandsSeparators = append(u.thousandsSeparators, l.thousandsSeparators...)
		u.decimalSeparators = append(u.decimalSeparators, l.decimalSeparators...)
		u.rangeWords = append(u.rangeWords, l.rangeWords...)
		u.pluralMarkers = append(u.pluralMarkers, l.pluralMarkers...)
		u.unitModifiers = append(u.unitModifiers, l.unitModifiers...)
		u.vague = append(u.vague, l.vague...)
		u.ingredientHeadings = append(u.ingredientHeadings, l.ingredientHeadings...)
		u.stepHeadings = append(u.stepHeadings, l.stepHeadings...)
		u.stepWords = append(u.stepWords, l.stepWords...)
		u.promo = append(u.promo, l.promo...)
	}
	return u
}

// detectLanguage picks the table to read a recipe with. tag is what the
// page declares ("de-DE", "en_GB", "" for none); its primary subtag picks
// the table when there is one. Otherwise every table scores the lines (see
// score) and the best one wins; a tie, nothing recognized included, reads
// with every table at once.
// detectLanguage("de-DE", nil) = german; detectLanguage("", ["2 cups flour"]) = english.
func detectLanguage(tag string, lines []string) *language {
	primary, _, _ := strings.Cut(strings.ReplaceAll(strings.ToLower(strings.TrimSpace(tag)), "_", "-"), "-")
	if l := byTag(primary); l != nil {
		return l
	}
	var best *language
	bestScore, tie := 0, false
	for _, l := range languages {
		switch s := score(l, lines); {
		case s > bestScore:
			best, bestScore, tie = l, s, false
		case s == bestScore && s > 0:
			tie = true
		}
	}
	if best == nil || tie {
		return anyLanguage
	}
	return best
}

// byTag returns the registered table with tag, nil when there is none.
func byTag(tag string) *language {
	i := slices.IndexFunc(languages, func(l *language) bool { return l.tag == tag })
	if i < 0 {
		return nil
	}
	return languages[i]
}

// score counts the lines lang recognizes: a heading, a unit after the
// quantity, a vague word or a plural marker. Words several tables share,
// like "g", count for each of them and so decide nothing.
// score(german, ["Zutaten", "1 EL Salz", "Salz"]) = 2.
func score(lang *language, lines []string) int {
	re := lang.patterns()
	n := 0
	for _, l := range lines {
		ing, _ := parseLine(l, lang)
		if re.ingredientHeading.MatchString(l) || re.stepHeading.MatchString(l) ||
			ing.Unit != nil || re.vague.MatchString(l) || re.plural.MatchString(l) {
			n++
		}
	}
	return n
}

// patterns compiles the table's words on first use.
func (l *language) patterns() *patterns {
	l.once.Do(func() {
		l.re.units = map[string]bool{}
		for _, u := range l.units {
			l.re.units[strings.ToLower(u)] = true
		}
		grouped := `[1-9]\d{0,2}(?:(?:` + alternatives(l.thousandsSeparators, "") + `)\d{3})+` + // not "0.125"
			`(?:(?:` + alternatives(l.decimalSeparators, "") + `)\d+)?`
		l.re.thousands = regexp.MustCompile(`^` + grouped + `$`)
		num := `(?:` + grouped + `|` + number + `)`
		l.re.quantity = regexp.MustCompile(`(?i)^(` + num + `)(?:\s*(?:-|–|—` + alternatives(l.rangeWords, "|") + `)\s*(` + num + `))?`)
		l.re.plural = regexp.MustCompile(`(?i)(\pL)(?:` + alternatives(l.pluralMarkers, "") + `)([^\pL]|$)`)
		l.re.unitModifier = regexp.MustCompile(`(?i)^,?\s*(` + alternatives(l.unitModifiers, "") + `)(?:[^\pL]|$)`)
		l.re.vague = regexp.MustCompile(`(?i)(?:^|[^\pL])(?:` + alternatives(l.vague, "") + `)(?:[^\pL]|$)`)
		l.re.ingredientHeading = headingPattern(l.ingredientHeadings)
		l.re.stepHeading = headingPattern(l.stepHeadings)
		l.re.numbered = regexp.MustCompile(`(?i)^\d+\s*[.)](?:\s+|$)|^(?:` + alternatives(l.stepWords, "") + `)\s+\d+\s*[.:)]?\s*`)
	})
	return &l.re
}

// alternatives quotes words for a regexp alternation, longest first so a
// longer phrase wins over its prefix, each preceded by lead.
// alternatives(["to", "bis"], "|") = "|bis|to".
func alternatives(words []string, lead string) string {
	ws := slices.Clone(words)
	slices.SortStableFunc(ws, func(a, b string) int { return len(b) - len(a) })
	var parts []string
	for _, w := range ws {
		parts = append(parts, regexp.QuoteMeta(strings.ToLower(w)))
	}
	if len(parts) == 0 {
		if lead == "" {
			return `[^\s\S]` // matches nothing
		}
		return ""
	}
	return lead + strings.Join(parts, "|")
}

// headingPattern matches a whole line that is one of the headings, with or
// without a trailing colon; a heading ending in " …" takes any words after.
func headingPattern(headings []string) *regexp.Regexp {
	var parts []string
	for _, h := range headings {
		if prefix, ok := strings.CutSuffix(h, " …"); ok {
			parts = append(parts, regexp.QuoteMeta(strings.ToLower(prefix))+`\b.*`)
		} else {
			parts = append(parts, regexp.QuoteMeta(strings.ToLower(h)))
		}
	}
	if len(parts) == 0 {
		parts = []string{`[^\s\S]`}
	}
	return regexp.MustCompile(`(?i)^(?:` + strings.Join(parts, "|") + `)\s*:?$`)
}

// stripPluralMarkers removes lang's plural markers where they end a word.
// stripPluralMarkers("2 Zehe/n Knoblauch", german) = "2 Zehe Knoblauch".
func stripPluralMarkers(s string, lang *language) string {
	return lang.patterns().plural.ReplaceAllString(s, "$1$2")
}

// removePromo drops the sentences of a description that one of lang's promo
// patterns matches and keeps the rest as they were.
// removePromo("Frisch und scharf. Über 112 Bewertungen.", german) = "Frisch und scharf.".
func removePromo(s string, lang *language) string {
	var keep []string
	for _, sentence := range sentences(s) {
		if !slices.ContainsFunc(lang.promo, func(p *regexp.Regexp) bool { return p.MatchString(sentence) }) {
			keep = append(keep, sentence)
		}
	}
	return strings.Join(keep, " ")
}

var sentenceEnd = regexp.MustCompile(`[.!?]+\s+`)

// sentences splits text after ".", "!" or "?" followed by a space.
// sentences("Mix. Bake! Serve") = ["Mix.", "Bake!", "Serve"].
func sentences(s string) []string {
	var out []string
	start := 0
	for _, m := range sentenceEnd.FindAllStringIndex(s, -1) {
		out = append(out, strings.TrimSpace(s[start:m[1]]))
		start = m[1]
	}
	if rest := strings.TrimSpace(s[start:]); rest != "" {
		out = append(out, rest)
	}
	return out
}
