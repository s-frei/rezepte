package recipe

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/s-frei/rezepte/service/internal/db/sqlc"
	"github.com/s-frei/rezepte/service/internal/ingredient"
)

// Caps on ListParams.Ingredients and ListParams.Have, applied after blanks
// and duplicates are dropped.
const (
	maxChips = 20
	maxHave  = 100
)

// PantryItem is an ingredient by the name to show and its ingredient.Key.
type PantryItem struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

// CardPantry is how a recipe fits the ingredient chips. Chips is how many
// chips it uses; Used are the recipe's own names of the ingredients a chip
// covers; Missing are its ingredients no chip covers, in recipe order, once
// per key, water left out. Ticks are not applied to Missing: the client
// applies them, so a tick needs no new request.
type CardPantry struct {
	Chips   int          `json:"chips"`
	Used    []string     `json:"used"`
	Missing []PantryItem `json:"missing"`
}

// PagePantry is the pantry state of a whole result. Sections counts the
// recipes by Chips, over all pages, and is there only in the best-match
// order. Row is every result's Missing in rank order, once per key: the
// tick row. Ticked echoes ListParams.Have as normalized.
type PagePantry struct {
	Sections map[int]int  `json:"sections,omitempty" required:"false"`
	Row      []PantryItem `json:"row"`
	Ticked   []PantryItem `json:"ticked"`
}

// pantryItems trims names, drops blanks, keys in skip and repeated keys, and
// caps the result at limit. It always returns a non-nil slice.
func pantryItems(names []string, limit int, skip []PantryItem) []PantryItem {
	out := []PantryItem{}
	for _, name := range names {
		name = strings.TrimSpace(name)
		key := ingredient.Key(name)
		has := func(i PantryItem) bool { return i.Key == key }
		if key == "" || slices.ContainsFunc(skip, has) || slices.ContainsFunc(out, has) {
			continue
		}
		out = append(out, PantryItem{Name: name, Key: key})
		if len(out) == limit {
			break
		}
	}
	return out
}

func coveredBy(items []PantryItem, key string) bool {
	return slices.ContainsFunc(items, func(i PantryItem) bool { return ingredient.Covers(i.Key, key) })
}

// listPantry is List with ingredient chips: only the recipes f matches that
// use at least one chip, in the best-match order ranked by most chips used,
// then fewest missing after the ticks, SQL's order breaking ties.
//
// ponytail: ranks every filtered recipe in Go on each request; fine at household scale (hundreds), move ranking into SQL if instances grow to thousands.
func (s *Service) listPantry(ctx context.Context, p ListParams, chips []PantryItem, f sqlc.RecipeFilter, sort sqlc.RecipeSort, limit, page int) (Page, error) {
	have := pantryItems(p.Have, maxHave, chips)
	all, err := s.q.ListRecipes(ctx, f, sort, -1, 0) // SQLite: LIMIT -1 is no limit
	if err != nil {
		return Page{}, fmt.Errorf("list recipes: %w", err)
	}
	ids := make([]string, len(all))
	for i, r := range all {
		ids[i] = r.ID
	}
	names, err := s.ingredientNames(ctx, ids)
	if err != nil {
		return Page{}, err
	}

	type ranked struct {
		row     sqlc.Recipe
		pantry  *CardPantry
		missing int // after the ticks
	}
	var rs []ranked
	for _, r := range all {
		cp := &CardPantry{Used: []string{}, Missing: []PantryItem{}}
		seen := map[string]bool{}
		for _, name := range names[r.ID] {
			key := ingredient.Key(name)
			if seen[key] {
				continue
			}
			seen[key] = true
			if coveredBy(chips, key) {
				cp.Used = append(cp.Used, name)
			} else if !ingredient.IsWater(key) {
				cp.Missing = append(cp.Missing, PantryItem{Name: name, Key: key})
			}
		}
		for _, c := range chips {
			if slices.ContainsFunc(cp.Used, func(name string) bool { return ingredient.Covers(c.Key, ingredient.Key(name)) }) {
				cp.Chips++
			}
		}
		if cp.Chips == 0 {
			continue
		}
		n := 0
		for _, m := range cp.Missing {
			if !coveredBy(have, m.Key) {
				n++
			}
		}
		rs = append(rs, ranked{r, cp, n})
	}

	pp := &PagePantry{Row: []PantryItem{}, Ticked: have}
	if sort == sqlc.RecipeSortUpdated {
		slices.SortStableFunc(rs, func(a, b ranked) int {
			if a.pantry.Chips != b.pantry.Chips {
				return b.pantry.Chips - a.pantry.Chips
			}
			return a.missing - b.missing
		})
		pp.Sections = map[int]int{}
		for _, r := range rs {
			pp.Sections[r.pantry.Chips]++
		}
	}
	inRow := map[string]bool{}
	for _, r := range rs {
		for _, m := range r.pantry.Missing {
			if !inRow[m.Key] {
				inRow[m.Key] = true
				pp.Row = append(pp.Row, m)
			}
		}
	}

	start := min((page-1)*limit, len(rs))
	pageRanks := rs[start:min(start+limit, len(rs))]
	rows := make([]sqlc.Recipe, len(pageRanks))
	for i, r := range pageRanks {
		rows[i] = r.row
	}
	items, err := s.toCards(ctx, rows, p.UserID)
	if err != nil {
		return Page{}, err
	}
	if err := s.matchCards(ctx, items, rows, searchWords(p.Query), names); err != nil {
		return Page{}, err
	}
	for i, r := range pageRanks {
		items[i].Pantry = r.pantry
	}
	return Page{Items: items, Page: page, Limit: limit, Total: len(rs), Pantry: pp}, nil
}
