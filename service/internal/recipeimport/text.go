package recipeimport

import (
	"slices"
	"strings"
)

// ReadText reads a recipe someone copied as plain text: first line title,
// headed or guessed ingredient block, steps by number or paragraph. Pasted
// text declares no language, so the table that recognizes most of its lines
// reads it (detectLanguage); Page.Language names it.
func ReadText(text string) (Page, bool) {
	return readText(text, detectLanguage("", strings.Split(text, "\n")))
}

// readText is ReadText with the headings, step words, amounts and promo of
// lang.
// readText("Suppe\nZutaten\n1 l Wasser\nZubereitung\nKochen.", german) has
// the ingredient "1 l Wasser" and the step "Kochen.".
func readText(text string, lang *language) (Page, bool) {
	re := lang.patterns()
	ingredientHeading, stepHeading, numbered := re.ingredientHeading, re.stepHeading, re.numbered
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	for i := range lines {
		lines[i] = strings.Join(strings.Fields(lines[i]), " ")
	}
	start := 0
	for start < len(lines) && lines[start] == "" {
		start++
	}
	if start == len(lines) {
		return Page{}, false
	}
	var p Page
	body := lines[start:]
	if !ingredientHeading.MatchString(body[0]) && !stepHeading.MatchString(body[0]) {
		p.Title, body = body[0], body[1:] // a text opening with a heading has no title; the editor asks
	}

	const (
		description = iota
		ingredients
		steps
	)
	section := description
	headed := false
	// Only a text without an ingredient heading has its block guessed, so a
	// "4 Portionen" line above "Zutaten" stays in the description.
	guess := !slices.ContainsFunc(body, ingredientHeading.MatchString)
	var desc []string
	var step []string
	flush := func() {
		if len(step) > 0 {
			p.Steps = append(p.Steps, strings.Join(step, " "))
			step = nil
		}
	}
	for _, l := range body {
		switch {
		case ingredientHeading.MatchString(l):
			flush()
			section, headed = ingredients, true
			continue
		case stepHeading.MatchString(l):
			flush()
			section, headed = steps, true
			continue
		}
		if guess && section == description && l != "" && re.quantity.MatchString(l) {
			section = ingredients // no headings: a line starting with an amount opens the ingredients
		}
		switch section {
		case description:
			if l != "" {
				desc = append(desc, l)
			}
		case ingredients:
			if l == "" {
				if !headed && len(p.Ingredients) > 0 {
					section = steps // no headings: the blank line after the block starts the steps
				}
				continue
			}
			if headed || !numbered.MatchString(l) {
				p.Ingredients = append(p.Ingredients, l)
				break
			}
			section = steps // no headings: a numbered line ("1. Mix.", not "1.5 l") starts the steps
			fallthrough
		case steps:
			if l == "" {
				flush()
				continue
			}
			if loc := numbered.FindStringIndex(l); loc != nil {
				flush()
				l = l[loc[1]:]
			}
			step = append(step, l)
		}
	}
	flush()
	p.Description = removePromo(strings.Join(desc, " "), lang)
	p.Language = lang.tag
	if len(p.Ingredients) == 0 && len(p.Steps) == 0 {
		return Page{}, false
	}
	return p, true
}
