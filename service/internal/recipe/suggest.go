package recipe

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/s-frei/rezepte/service/internal/db"
	"github.com/s-frei/rezepte/service/internal/ingredient"
)

// maxAlso caps IngredientSuggestion.Also.
const maxAlso = 3

// IngredientSuggestion is an ingredient name to offer as a chip. Name is the
// most frequent spelling of Key; Count is the distinct recipes using any
// name Key covers; Also are other names Key covers, most used first.
type IngredientSuggestion struct {
	Name  string   `json:"name"`
	Key   string   `json:"key"`
	Count int      `json:"count"`
	Also  []string `json:"also"`
}

// IngredientSuggestions returns the ingredient names with a word starting
// with q (folded), the names starting with q as a whole first, then the most
// used. limit is clamped to 1..50, 10 when unset.
//
// ponytail: reads every ingredient name per call and pairs keys in O(keys²); fine at household scale, cache or index if instances grow to thousands of recipes.
func (s *Service) IngredientSuggestions(ctx context.Context, q string, limit int) ([]IngredientSuggestion, error) {
	if limit <= 0 {
		limit = 10
	}
	limit = min(limit, 50)
	rows, err := s.q.ListIngredientNames(ctx)
	if err != nil {
		return nil, fmt.Errorf("list ingredient names: %w", err)
	}
	type group struct {
		spellings map[string]int
		recipes   map[string]bool
		name      string
	}
	groups := map[string]*group{}
	for _, r := range rows {
		name := strings.TrimSpace(r.Name)
		key := ingredient.Key(name)
		if key == "" {
			continue
		}
		g := groups[key]
		if g == nil {
			g = &group{spellings: map[string]int{}, recipes: map[string]bool{}}
			groups[key] = g
		}
		g.spellings[name]++
		g.recipes[r.RecipeID] = true
	}
	keys := make([]string, 0, len(groups))
	for key, g := range groups {
		for name, n := range g.spellings {
			if n > g.spellings[g.name] || (n == g.spellings[g.name] && (g.name == "" || name < g.name)) {
				g.name = name
			}
		}
		keys = append(keys, key)
	}
	slices.Sort(keys) // a stable order for ties

	fq := db.FoldWords(q)
	type hit struct {
		IngredientSuggestion
		first bool
	}
	var hits []hit
	for _, key := range keys {
		g := groups[key]
		words := strings.Fields(db.FoldWords(g.name))
		start := -1
		for i := range words {
			if strings.HasPrefix(strings.Join(words[i:], " "), fq) {
				start = i
				break
			}
		}
		if start < 0 {
			continue
		}
		covered := map[string]bool{}
		type other struct {
			name string
			n    int
		}
		var others []other
		for _, k2 := range keys {
			if !ingredient.Covers(key, k2) {
				continue
			}
			for id := range groups[k2].recipes {
				covered[id] = true
			}
			if k2 != key && groups[k2].name != g.name {
				others = append(others, other{groups[k2].name, len(groups[k2].recipes)})
			}
		}
		slices.SortStableFunc(others, func(a, b other) int { return cmp.Compare(b.n, a.n) })
		also := []string{}
		for _, o := range others[:min(maxAlso, len(others))] {
			also = append(also, o.name)
		}
		hits = append(hits, hit{IngredientSuggestion{g.name, key, len(covered), also}, start == 0})
	}
	slices.SortStableFunc(hits, func(a, b hit) int {
		if a.first != b.first {
			if a.first {
				return -1
			}
			return 1
		}
		return cmp.Compare(b.Count, a.Count)
	})
	out := make([]IngredientSuggestion, 0, min(limit, len(hits)))
	for _, h := range hits[:min(limit, len(hits))] {
		out = append(out, h.IngredientSuggestion)
	}
	return out, nil
}
