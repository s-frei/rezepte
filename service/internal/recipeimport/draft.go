package recipeimport

import (
	"strings"

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
// author's name (case-insensitively) is neither.
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
	group := recipe.IngredientGroup{Ingredients: []recipe.Ingredient{}}
	lang := detectLanguage(p.Language, p.Ingredients)
	for _, line := range p.Ingredients {
		ing, sure := parseLine(line, lang)
		if ing.Name == "" {
			continue
		}
		if !sure {
			d.Review = append(d.Review, DraftReview{Group: 0, Ingredient: len(group.Ingredients), Line: line})
		}
		group.Ingredients = append(group.Ingredients, ing)
	}
	in.IngredientGroups = []recipe.IngredientGroup{group}
	for _, s := range p.Steps {
		in.Steps = append(in.Steps, recipe.Step{Text: s, References: []recipe.IngredientRef{}, Times: []recipe.StepTime{}})
	}
	spelling := make(map[string]string, len(known))
	for _, k := range known {
		spelling[strings.ToLower(strings.TrimSpace(k))] = k
	}
	authors := make(map[string]bool, len(p.Authors))
	for _, a := range p.Authors {
		authors[strings.ToLower(strings.TrimSpace(a))] = true
	}
	for _, kw := range p.Keywords {
		if authors[strings.ToLower(strings.TrimSpace(kw))] {
			continue
		}
		if k, ok := spelling[strings.ToLower(kw)]; ok {
			in.Tags = append(in.Tags, k)
		} else if len([]rune(kw)) <= 40 {
			d.SuggestedTags = append(d.SuggestedTags, kw)
		}
	}
	d.Recipe, d.Truncated = clampInput(in)
	d.Review = keepReviews(d.Review, d.Recipe)
	// Dropping suggestions (over 40 runes, beyond 20) loses no recipe
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
		if r.Ingredient < len(in.IngredientGroups[0].Ingredients) {
			out = append(out, r)
		}
	}
	return out
}
