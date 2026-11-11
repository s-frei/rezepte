package shopping

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/s-frei/rezepte/service/internal/recipe"
	"github.com/s-frei/rezepte/service/internal/recipeimport"
)

// dashNoteRe is a " - " or " – " between name and note.
var dashNoteRe = regexp.MustCompile(`\s+[–-]\s+`)

// ParseFree splits free text into quantity, unit, name and note with the
// recipe import's shared split (recipeimport.SplitItem), so a typed and an
// imported item land in the same row. When that split finds no note, the
// first " - " or " – " after the name starts one.
func ParseFree(text string) recipe.Ingredient {
	it, _ := splitFree(text)
	return it
}

// splitFree is ParseFree, and reports whether SplitItem cut the name
// (recipeimport.MaxName runes) or the note (200 runes) to fit.
func splitFree(text string) (recipe.Ingredient, bool) {
	text = strings.Join(strings.Fields(text), " ")
	it, cut := recipeimport.SplitItem(text)
	if it.Name == "" { // "2" alone stays a name, as typed
		return recipe.Ingredient{Name: text}, utf8.RuneCountInString(text) > recipeimport.MaxName
	}
	if it.Note == nil {
		if loc := dashNoteRe.FindStringIndex(it.Name); loc != nil && loc[0] > 0 && strings.TrimSpace(it.Name[loc[1]:]) != "" {
			it.Name, it.Note = strings.TrimSpace(it.Name[:loc[0]]), ptr(strings.TrimSpace(it.Name[loc[1]:]))
		}
	}
	return it, cut
}

func ptr(s string) *string { return &s }
