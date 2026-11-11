package recipeimport

import (
	"strings"
	"unicode/utf8"

	"github.com/s-frei/rezepte/service/internal/recipe"
)

// Cut shortens s to at most n runes, at a word boundary when one is near.
func Cut(s string, n int) (string, bool) {
	r := []rune(s)
	if len(r) <= n {
		return s, false
	}
	c := string(r[:n])
	if i := strings.LastIndexByte(c, ' '); utf8.RuneCountInString(c[:max(i, 0)]) > n/2 {
		c = c[:i]
	}
	return strings.TrimSpace(c), true
}

func cutPtr(p *string, n int) (*string, bool) {
	if p == nil {
		return nil, false
	}
	s, did := Cut(*p, n)
	return &s, did
}

// clampIngredient cuts name, unit and note to the recipe.Ingredient maxima
// and reports whether it cut any of them.
func clampIngredient(i recipe.Ingredient) (recipe.Ingredient, bool) {
	var name, unit, note bool
	i.Name, name = Cut(i.Name, 120)
	i.Unit, unit = cutPtr(i.Unit, 20)
	i.Note, note = cutPtr(i.Note, 200)
	return i, name || unit || note
}

// maxGroups is recipe.Input's most ingredient groups.
const maxGroups = 20

// clampInput cuts every field and list of in to the recipe.Input maxima and
// reports whether anything was cut. Groups beyond the 20th merge into it.
func clampInput(in recipe.Input) (recipe.Input, bool) {
	cutAny := false
	c := func(s string, n int) string {
		out, did := Cut(s, n)
		cutAny = cutAny || did
		return out
	}
	in.Title = c(in.Title, 200)
	in.Description = c(in.Description, 2000)
	in.Servings = min(max(in.Servings, 1), 99)
	for _, p := range []**int{&in.PrepMinutes, &in.CookMinutes} {
		if *p != nil && **p > 1440 {
			v := 1440
			*p, cutAny = &v, true
		}
	}
	if in.SourceURL != nil && len(*in.SourceURL) > 500 {
		in.SourceURL, cutAny = nil, true
	}
	if in.SourceName != nil {
		s := c(*in.SourceName, 200)
		in.SourceName = &s
	}
	if len(in.Tags) > 20 {
		in.Tags, cutAny = in.Tags[:20], true
	}
	if gs := in.IngredientGroups; len(gs) > maxGroups {
		last := &gs[maxGroups-1]
		for _, g := range gs[maxGroups:] {
			last.Ingredients = append(last.Ingredients, g.Ingredients...)
		}
		in.IngredientGroups, cutAny = gs[:maxGroups], true
	}
	for i := range in.IngredientGroups {
		g := &in.IngredientGroups[i]
		g.Name, _ = cutPtr(g.Name, 60)
		if len(g.Ingredients) > 100 {
			g.Ingredients, cutAny = g.Ingredients[:100], true
		}
	}
	if len(in.Steps) > 50 {
		in.Steps, cutAny = in.Steps[:50], true
	}
	for i := range in.Steps {
		in.Steps[i].Text = c(in.Steps[i].Text, 2000)
	}
	return in, cutAny
}
