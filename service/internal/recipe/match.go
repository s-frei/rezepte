package recipe

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/s-frei/rezepte/service/internal/db"
)

// The fields a search finds a word in, strongest first. A card's Section is
// the strongest field any of its words was found in; sqlc's RecipeFilter.tier
// orders a search by the same fields, so the overview's sections never
// reappear further down the list.
const (
	FieldTitle       = "title"
	FieldTags        = "tags"
	FieldIngredients = "ingredients"
	FieldDescription = "description"
)

// tierSections names sqlc.RecipeFilter.tier's numbers.
var tierSections = map[int]string{4: FieldTitle, 3: FieldTags, 2: FieldIngredients, 1: FieldDescription}

// Match says where a search found a recipe, for a card in a search's
// results: the section the overview groups it under, and one Find for each
// field the search words the title does not already show were found in.
type Match struct {
	Section string `json:"section" enum:"title,tags,ingredients,description"`
	// Title is the title in parts, the words it holds marked, when any
	// search word was found in it; a list too narrow for a find per field
	// (the command palette) marks the hit in the title itself.
	Title []FindPart `json:"title,omitempty" required:"false"`
	Finds []Find     `json:"finds" nullable:"false"`
}

// Find is where search words were found outside the title: the field, the
// matching items in it (at most maxFindItems tags or ingredients, or one
// excerpt of the description), and how many more matched beyond those.
type Find struct {
	Field string     `json:"field" enum:"tags,ingredients,description"`
	Items []FindItem `json:"items" nullable:"false"`
	More  int        `json:"more"`
}

// FindItem is one tag, ingredient or description excerpt, in parts, with the
// found text marked: the client shows it without matching anything itself.
type FindItem struct {
	Parts []FindPart `json:"parts" nullable:"false"`
}

// FindPart is a run of a FindItem's text; Hit marks what the search word matched.
type FindPart struct {
	Text string `json:"text"`
	Hit  bool   `json:"hit"`
}

// maxFindItems is how many matching tags or ingredients a Find names; the
// rest are counted in More.
const maxFindItems = 2

// excerptAfter is roughly how many runes of the description an excerpt keeps
// after the find; the card cuts it to one line anyway.
const excerptAfter = 60

// searchWord is one search word as the matcher needs it: its stem, found
// anywhere in a word when it is long enough for a trigram, at the start of a
// word when it is not - as the WHERE that found the recipe matches it.
type searchWord struct {
	stem []rune
	long bool
}

func matchWords(words []string) []searchWord {
	out := make([]searchWord, len(words))
	for i, w := range words {
		s := stem(w)
		out[i] = searchWord{stem: []rune(s), long: utf8.RuneCountInString(s) >= 3}
	}
	return out
}

// hits returns the folded-rune ranges in f where w matches.
func (w searchWord) hits(f db.Folded) [][2]int {
	var out [][2]int
	n := len(w.stem)
	for i := 0; i+n <= len(f.Runes); i++ {
		if !w.long && i > 0 && (unicode.IsLetter(f.Runes[i-1]) || unicode.IsDigit(f.Runes[i-1])) {
			continue
		}
		if slices.Equal(f.Runes[i:i+n], w.stem) {
			out = append(out, [2]int{i, i + n})
			i += n - 1
		}
	}
	return out
}

// parts splits f's source into runs, the hits marked, from byte from to to.
func parts(f db.Folded, hits [][2]int, from, to int) []FindPart {
	var out []FindPart
	at := from
	for _, h := range hits {
		start, end := f.Spans[h[0]][0], f.Spans[h[1]-1][1]
		if end <= from || start >= to {
			continue
		}
		start, end = max(start, from), min(end, to)
		if start > at {
			out = append(out, FindPart{Text: f.Source[at:start]})
		}
		out = append(out, FindPart{Text: f.Source[start:end], Hit: true})
		at = end
	}
	if at < to {
		out = append(out, FindPart{Text: f.Source[at:to]})
	}
	return out
}

// matchCard works out where words were found in one recipe: each word the
// title does not show goes to its strongest field, and every field that
// gets a word is one Find, marking all of its words. It returns nil when no
// word is found in any field, which the WHERE that found the recipe rules
// out unless the two disagree.
func matchCard(words []searchWord, title string, tags, ingredients []string, description string) *Match {
	ft := db.FoldSpans(title)
	fd := db.FoldSpans(description)
	ftags := foldAll(tags)
	fings := foldAll(ingredients)

	// best is the strongest field found, as tierSections numbers it.
	best := 0
	var inTitle, inTags, inIngredients, inDescription []searchWord
	for _, w := range words {
		switch {
		case len(w.hits(ft)) > 0:
			best = 4
			inTitle = append(inTitle, w)
		case anyHit(w, ftags):
			best = max(best, 3)
			inTags = append(inTags, w)
		case anyHit(w, fings):
			best = max(best, 2)
			inIngredients = append(inIngredients, w)
		case len(w.hits(fd)) > 0:
			best = max(best, 1)
			inDescription = append(inDescription, w)
		}
	}
	if best == 0 {
		return nil
	}
	m := &Match{Section: tierSections[best], Finds: []Find{}}
	if inTitle != nil {
		m.Title = parts(ft, allHits(inTitle, ft), 0, len(ft.Source))
	}
	if inTags != nil {
		m.Finds = append(m.Finds, listFind(inTags, words, FieldTags, ftags))
	}
	if inIngredients != nil {
		m.Finds = append(m.Finds, listFind(inIngredients, words, FieldIngredients, fings))
	}
	if inDescription != nil {
		m.Finds = append(m.Finds, excerpt(fd, allHits(inDescription, fd), allHits(words, fd)))
	}
	return m
}

func foldAll(texts []string) []db.Folded {
	out := make([]db.Folded, len(texts))
	for i, t := range texts {
		out[i] = db.FoldSpans(t)
	}
	return out
}

func anyHit(w searchWord, items []db.Folded) bool {
	return slices.ContainsFunc(items, func(f db.Folded) bool { return len(w.hits(f)) > 0 })
}

// allHits is where any of words matches in f, in order, overlapping
// matches joined into one ("Butterbrot" for "butter" and "erbrot").
func allHits(words []searchWord, f db.Folded) [][2]int {
	var hits [][2]int
	for _, w := range words {
		hits = append(hits, w.hits(f)...)
	}
	slices.SortFunc(hits, func(a, b [2]int) int { return a[0] - b[0] })
	var out [][2]int
	for _, h := range hits {
		if n := len(out); n > 0 && h[0] <= out[n-1][1] {
			out[n-1][1] = max(out[n-1][1], h[1])
			continue
		}
		out = append(out, h)
	}
	return out
}

// listFind is the Find of words among a recipe's tags or ingredients: the
// first maxFindItems that match, in the recipe's order, the rest counted.
// Every search word in them is marked, the ones another field holds too, so
// a word never shows up plain on one line and marked on the next. An
// ingredient listed twice under the same name counts once.
func listFind(words, all []searchWord, field string, items []db.Folded) Find {
	find := Find{Field: field, Items: []FindItem{}}
	seen := map[string]bool{}
	for _, f := range items {
		if len(allHits(words, f)) == 0 || seen[f.Source] {
			continue
		}
		seen[f.Source] = true
		if len(find.Items) < maxFindItems {
			find.Items = append(find.Items, FindItem{Parts: parts(f, allHits(all, f), 0, len(f.Source))})
		} else {
			find.More++
		}
	}
	return find
}

// excerpt is the description from the word the first of hits is in to about
// excerptAfter runes past it, cut at word boundaries and marked with "…"
// where it was cut, every search word in it marked (see listFind); More
// counts the hits past its end.
func excerpt(f db.Folded, hits, marks [][2]int) Find {
	src := f.Source
	first := f.Spans[hits[0][0]][0]
	end := f.Spans[hits[0][1]-1][1]

	// From the start of the word the find is in, so even a narrow card,
	// which cuts the excerpt to one line, shows the find.
	// The space may be wider than a byte (a no-break space).
	from := 0
	if sp := strings.LastIndexFunc(src[:first], unicode.IsSpace); sp >= 0 {
		_, size := utf8.DecodeRuneInString(src[sp:])
		from = sp + size
	}
	to := len(src)
	if rest := src[end:]; utf8.RuneCountInString(rest) > excerptAfter {
		cut := end + len(string([]rune(rest)[:excerptAfter]))
		if sp := strings.LastIndexFunc(src[end:cut], unicode.IsSpace); sp > 0 {
			cut = end + sp
		}
		to = cut
	}
	item := FindItem{Parts: parts(f, marks, from, to)}
	if from > 0 {
		item.Parts = append([]FindPart{{Text: "… "}}, item.Parts...)
	}
	if to < len(src) {
		item.Parts = append(item.Parts, FindPart{Text: " …"})
	}
	// Only the finds the excerpt leaves out are counted.
	more := 0
	for _, h := range hits {
		if f.Spans[h[0]][0] >= to {
			more++
		}
	}
	return Find{Field: FieldDescription, Items: []FindItem{item}, More: more}
}
