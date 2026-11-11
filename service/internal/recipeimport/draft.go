package recipeimport

import (
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/s-frei/rezepte/service/internal/recipe"
)

// DraftReview points at an ingredient the parser was not sure about.
type DraftReview struct {
	Group      int    `json:"group"`
	Ingredient int    `json:"ingredient"`
	Line       string `json:"line"`
}

// Draft is a recipe.Input ready for the editor plus what the editor shows
// beside it. Nothing here is stored.
type Draft struct {
	Recipe        recipe.Input
	Review        []DraftReview
	SuggestedTags []string
	PhotoURL      string
	Truncated     bool
}

// Build turns what a reader found into a Draft, reading the ingredient lines
// with the table for p.Language (detected from the lines when ""). Keywords matching one of
// known (case-insensitively) become tags in the collection's spelling; the
// rest are suggestions the member accepts one by one. A keyword equal to an
// author's name (case-insensitively) is neither. Explicit tags (p.Tags,
// a "Tags:" line) all become tags. Each PageGroup becomes one group, and a
// review names the group of its line.
func Build(p Page, known []string) Draft {
	var d Draft
	in := recipe.Input{
		Title:       p.Title,
		Description: p.Description,
		Servings:    p.Servings,
		PrepMinutes: p.PrepMinutes,
		CookMinutes: p.CookMinutes,
		Tags:        []string{},
		Steps:       []recipe.Step{},
	}
	if in.Servings == 0 {
		in.Servings = 4
	}
	if p.SourceURL != "" {
		in.SourceURL = &p.SourceURL
	}
	if p.SourceName != "" {
		in.SourceName = &p.SourceName
	}
	var lines []string
	for _, g := range p.Groups {
		lines = append(lines, g.Lines...)
	}
	lang := detectLanguage(p.Language, lines)
	for gi, g := range p.Groups {
		group := recipe.IngredientGroup{Ingredients: []recipe.Ingredient{}}
		if g.Name != "" {
			group.Name = &g.Name
		}
		for _, line := range g.Lines {
			ing, sure := parseLine(line, lang)
			if ing.Name == "" {
				continue
			}
			if !sure {
				d.Review = append(d.Review, DraftReview{Group: gi, Ingredient: len(group.Ingredients), Line: line})
			}
			group.Ingredients = append(group.Ingredients, ing)
		}
		in.IngredientGroups = append(in.IngredientGroups, group)
	}
	if len(in.IngredientGroups) == 0 {
		in.IngredientGroups = []recipe.IngredientGroup{{Ingredients: []recipe.Ingredient{}}} // the schema wants one
	}
	// clampInput merges the groups beyond the 20th into it; their reviews
	// move with them.
	for i := range d.Review {
		for r := &d.Review[i]; r.Group > maxGroups-1; {
			r.Group--
			r.Ingredient += len(in.IngredientGroups[r.Group].Ingredients)
		}
	}
	for _, s := range p.Steps {
		in.Steps = append(in.Steps, recipe.Step{Text: s, References: []recipe.IngredientRef{}, Times: []recipe.StepTime{}, Photos: []recipe.StepPhoto{}})
	}
	spelling := make(map[string]string, len(known))
	for _, k := range known {
		spelling[strings.ToLower(strings.TrimSpace(k))] = k
	}
	authors := make(map[string]bool, len(p.Authors))
	for _, a := range p.Authors {
		authors[strings.ToLower(strings.TrimSpace(a))] = true
	}
	// A "Tags:" line is the cook's own choice, so every tag counts, in
	// the collection's spelling when it has one.
	for _, t := range p.Tags {
		t = strings.TrimSpace(strings.TrimPrefix(t, "#"))
		switch n := utf8.RuneCountInString(t); {
		case n == 0:
		case n > maxTag:
			d.Truncated = true
		default:
			if k, ok := spelling[strings.ToLower(t)]; ok {
				t = k
			}
			in.Tags = append(in.Tags, t)
		}
	}
	for _, kw := range p.Keywords {
		if authors[strings.ToLower(strings.TrimSpace(kw))] {
			continue
		}
		if k, ok := spelling[strings.ToLower(kw)]; ok {
			in.Tags = append(in.Tags, k)
		} else if len([]rune(kw)) <= maxTag {
			d.SuggestedTags = append(d.SuggestedTags, kw)
		}
	}
	// The editor keys tags by their text: a tag twice ("vegan, Vegan") broke it.
	in.Tags, d.SuggestedTags = uniqueFold(in.Tags), uniqueFold(d.SuggestedTags)
	var cut bool
	d.Recipe, cut = clampInput(in)
	d.Truncated = d.Truncated || cut
	d.Review = keepReviews(d.Review, d.Recipe)
	// Dropping suggestions (over maxTag runes, beyond 20) loses no recipe
	// content, so it does not count as truncated.
	if len(d.SuggestedTags) > 20 {
		d.SuggestedTags = d.SuggestedTags[:20]
	}
	d.PhotoURL = p.PhotoURL
	return d
}

// keepReviews drops hints whose ingredient was cut by clamping.
func keepReviews(rs []DraftReview, in recipe.Input) []DraftReview {
	out := rs[:0]
	for _, r := range rs {
		if r.Group < len(in.IngredientGroups) && r.Ingredient < len(in.IngredientGroups[r.Group].Ingredients) {
			out = append(out, r)
		}
	}
	return out
}

// uniqueFold keeps the first of tags that differ only in case.
func uniqueFold(tags []string) []string {
	seen := map[string]bool{}
	return slices.DeleteFunc(tags, func(t string) bool {
		k := strings.ToLower(t)
		dup := seen[k]
		seen[k] = true
		return dup
	})
}
