package shopping

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/s-frei/rezepte/service/internal/recipe"

	"github.com/s-frei/rezepte/service/internal/recipeimport"
)

// ErrNoItems is returned for list text without a single item.
var ErrNoItems = errors.New("no items found")

// ErrTooManyItems is returned for list text of more than maxImportItems items.
var ErrTooManyItems = errors.New("too many items")

// LineError names the line (1-based, in the pasted text) whose name or note
// exceeds an item's limits; such a line is refused, never cut.
type LineError struct{ Line int }

func (e *LineError) Error() string { return fmt.Sprintf("line %d is too long", e.Line) }

const maxImportItems = 200

// listNameWords open the line that names a list, one per app language.
var listNameWords = []string{"Liste", "List"}

var listNameRe = regexp.MustCompile(`(?i)^(` + strings.Join(listNameWords, "|") + `)\s*:\s*(.+)$`)

// ListText is a shopping list read from text: the name its first name line
// gives, if any, and its items in order.
type ListText struct {
	Name  string
	Items []recipe.Ingredient
}

// ParseListText reads a pasted shopping list: the text is readied like a
// recipe (recipeimport.PrepLines: code fence, bullets, checkboxes, Markdown
// around a line), then every "Liste:"/"List:" line is skipped and the first
// names the list, a line without a leading amount that ends in ":" is a
// heading and skipped, a merged row "2 Stück + 200 g Zwiebeln" gives one item
// per amount, and every other line is one item split as ParseFree does.
func ParseListText(text string) (ListText, error) {
	var lt ListText
	named := false
	for _, p := range recipeimport.PrepLines(text) {
		line := p.Text
		if line == "" {
			continue
		}
		if m := listNameRe.FindStringSubmatch(line); m != nil {
			if !named {
				lt.Name, named = strings.TrimSpace(m[2]), true
			}
			continue
		}
		items, cut := listLine(line)
		if cut {
			return ListText{}, &LineError{Line: p.Line}
		}
		if lt.Items = append(lt.Items, items...); len(lt.Items) > maxImportItems {
			return ListText{}, ErrTooManyItems
		}
	}
	if len(lt.Items) == 0 {
		return ListText{}, ErrNoItems
	}
	return lt, nil
}

// listLine reads one line: none for a heading, one item per part of a merged
// row, else one item. The bool reports a name or note over its limit.
func listLine(line string) ([]recipe.Ingredient, bool) {
	whole, cut := splitFree(line)
	if whole.Quantity == nil && strings.HasSuffix(line, ":") {
		return nil, false
	}
	parts := strings.Split(line, " + ")
	if whole.Quantity == nil || len(parts) < 2 {
		return []recipe.Ingredient{whole}, cut
	}
	last, lastCut := splitFree(parts[len(parts)-1])
	if last.Quantity == nil {
		return []recipe.Ingredient{whole}, cut
	}
	// Each amount before the last borrows its name: "2 Stück" + " Zwiebeln".
	items := make([]recipe.Ingredient, 0, len(parts))
	for _, part := range parts[:len(parts)-1] {
		p, _ := splitFree(part + " " + last.Name)
		if p.Quantity == nil || p.Name != last.Name || p.Note != nil {
			return []recipe.Ingredient{whole}, cut
		}
		p.Note = last.Note
		items = append(items, p)
	}
	return append(items, last), lastCut
}
