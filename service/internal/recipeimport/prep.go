package recipeimport

import (
	"slices"
	"strings"

	"github.com/s-frei/rezepte/service/internal/recipe"
)

// PrepLine is one line of text an AI chat or a notes app wrote, with its
// Markdown taken off. Bullet says the line started with "- ", "* " or "• ";
// Line is its number in the pasted text, from 1.
type PrepLine struct {
	Text   string
	Bullet bool
	Line   int
}

// PrepLines readies pasted text for the recipe and shopping-list readers.
// When the text holds a fenced block, only the first block counts (an AI
// chat wraps its answer in talk); its info string ("```text") goes. Each
// line has its whitespace collapsed and loses, in order, a bullet with its
// space, a checkbox ("[ ]", "[x]"), a "#" heading run with its space and
// "**…**" or "__…__" around the whole line. Empty lines stay as "", so
// paragraphs still split; "-5 °C" and "*optional" keep their marks.
// PrepLines("- [x] 2 l Milch") = [{"2 l Milch", true, 1}].
func PrepLines(text string) []PrepLine {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	first := 1
	if open := fenceAt(lines); open >= 0 {
		lines, first = lines[open+1:], open+2
		if end := fenceAt(lines); end >= 0 {
			lines = lines[:end]
		}
	}
	out := make([]PrepLine, 0, len(lines))
	for i, l := range lines {
		p := PrepLine{Line: first + i}
		l = strings.Join(strings.Fields(l), " ")
		for _, b := range []string{"- ", "* ", "• "} {
			if rest, ok := strings.CutPrefix(l, b); ok {
				l, p.Bullet = rest, true
				break
			}
		}
		for _, c := range []string{"[ ] ", "[x] ", "[X] "} {
			l = strings.TrimPrefix(l, c)
		}
		if h := strings.TrimLeft(l, "#"); h != l && strings.HasPrefix(h, " ") {
			l = h[1:]
		}
		for _, e := range []string{"**", "__"} {
			if len(l) > 2*len(e) && strings.HasPrefix(l, e) && strings.HasSuffix(l, e) {
				l = strings.TrimSpace(l[len(e) : len(l)-len(e)])
			}
		}
		p.Text = l
		out = append(out, p)
	}
	return out
}

// fenceAt returns the index of the first line that opens or closes a
// fenced block, -1 when there is none.
func fenceAt(lines []string) int {
	return slices.IndexFunc(lines, func(l string) bool { return strings.HasPrefix(strings.TrimSpace(l), "```") })
}

// SplitItem splits a shopping-list line into quantity, unit, name and note
// with every language's words, like an ingredient but without a verdict: a
// list item is taken as written.
// The bool reports a name, unit or note cut to its recipe.Ingredient
// maximum, so a list import can refuse the line instead.
// SplitItem("Eier (6 Stück)") = {Name "Eier", Note "6 Stück"}, false.
func SplitItem(line string) (recipe.Ingredient, bool) {
	return splitLine(line, anyLanguage)
}
