package recipeimport

import (
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// ReadText reads a recipe someone copied as plain text: first line title,
// detail lines ("Portionen: 4") and description, headed or guessed
// ingredient block in groups, steps by number, bullet or paragraph. Pasted
// text declares no language, so the table that recognizes most of its
// ingredient lines reads them (detectLanguage); Page.Language names it.
func ReadText(text string) (Page, bool) {
	var lines []string
	for _, l := range PrepLines(text) {
		lines = append(lines, l.Text)
	}
	return readText(text, detectLanguage("", lines))
}

// readText is ReadText with lang for the amounts and promo. Headings, step
// words and detail keys are structure and come from every table at once
// (anyLanguage), so German keys above English ingredients read too.
// readText("Suppe\nZutaten\n1 l Wasser\nZubereitung\nKochen.", german) has
// the ingredient "1 l Wasser" and the step "Kochen.".
func readText(text string, lang *language) (Page, bool) {
	re, structure := lang.patterns(), anyLanguage.patterns()
	ingredientHeading, stepHeading, numbered := structure.ingredientHeading, structure.stepHeading, structure.numbered
	lines := PrepLines(text)
	start := 0
	for start < len(lines) && lines[start].Text == "" {
		start++
	}
	if start == len(lines) {
		return Page{}, false
	}
	var p Page
	body := lines[start:]
	if !ingredientHeading.MatchString(body[0].Text) && !stepHeading.MatchString(body[0].Text) {
		p.Title, body = body[0].Text, body[1:] // a text opening with a heading has no title; the editor asks
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
	guess := !slices.ContainsFunc(body, func(l PrepLine) bool { return ingredientHeading.MatchString(l.Text) })
	var desc []string
	var step []string
	flush := func() {
		if len(step) > 0 {
			p.Steps = append(p.Steps, strings.Join(step, " "))
			step = nil
		}
	}
	found := 0 // ingredient lines read
	for _, pl := range body {
		l := pl.Text
		switch {
		case ingredientHeading.MatchString(l):
			flush()
			section, headed = ingredients, true
			continue
		case stepHeading.MatchString(l):
			flush()
			section, headed = steps, true
			continue
		case section == description && p.readDetail(l):
			continue
		case section == ingredients && headed && isGroupLine(l, re):
			p.Groups = append(p.Groups, PageGroup{Name: strings.TrimSpace(strings.TrimSuffix(l, ":"))})
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
				if !headed && found > 0 {
					section = steps // no headings: the blank line after the block starts the steps
				}
				continue
			}
			if headed || !numbered.MatchString(l) {
				if len(p.Groups) == 0 {
					p.Groups = []PageGroup{{}} // lines before the first group line
				}
				g := &p.Groups[len(p.Groups)-1]
				g.Lines = append(g.Lines, l)
				found++
				break
			}
			section = steps // no headings: a numbered line ("1. Mix.", not "1.5 l") starts the steps
			fallthrough
		case steps:
			if l == "" {
				flush()
				continue
			}
			if pl.Bullet {
				flush()
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
	if found == 0 && len(p.Steps) == 0 {
		return Page{}, false
	}
	return p, true
}

// isGroupLine says whether l names an ingredient group: it ends in ":" and
// starts with no amount ("Für den Teig:", not "2 EL Öl:").
func isGroupLine(l string, re *patterns) bool {
	return len(l) > 1 && strings.HasSuffix(l, ":") && !re.quantity.MatchString(l)
}

// readDetail reads l into p when it is a detail line "Key: value" with a
// key of any table, and says whether it was one. An unknown key ("Tipp:
// …") is no detail and stays in the description.
func (p *Page) readDetail(l string) bool {
	key, value, ok := strings.Cut(l, ":")
	if !ok {
		return false
	}
	key, value = strings.TrimSpace(key), strings.TrimSpace(value)
	switch l := anyLanguage; l.detailKeys[strings.ToLower(key)] {
	case "servings":
		if m := firstInt.FindString(value); m != "" {
			n, _ := strconv.Atoi(m)
			p.Servings = min(max(n, 1), 99)
		}
	case "prep":
		if m, ok := readDuration(value, l); ok {
			p.PrepMinutes = &m
		}
	case "cook":
		if m, ok := readDuration(value, l); ok {
			p.CookMinutes = &m
		}
	case "tags":
		for t := range strings.SplitSeq(value, ",") {
			if t = strings.TrimSpace(t); t != "" {
				p.Tags = append(p.Tags, t)
			}
		}
	case "source":
		var name []string
		for _, f := range strings.Fields(value) {
			if p.SourceURL == "" && (strings.HasPrefix(f, "https://") || strings.HasPrefix(f, "http://")) {
				p.SourceURL = f
			} else {
				name = append(name, f)
			}
		}
		p.SourceName = strings.Join(name, " ")
	default:
		return false
	}
	return true
}

var durationPair = regexp.MustCompile(`^\s*(\d+(?:[.,]\d+)?)\s*(\pL+\.?)`)

// readDuration reads the leading run of number-unit pairs of a time value
// in minutes, units from lang's minuteWords and hourWords, from the larger
// unit to the smaller. It stops at anything else, "(" included, and at a
// second pair of the same unit; false when no pair leads. The sum is
// clamped to 0-1440.
// readDuration("1 Std. 30 Min. (davon 20 Min. Backzeit)", german) = 90.
func readDuration(v string, lang *language) (int, bool) {
	total, last, ok := 0.0, math.MaxInt, false
	for {
		m := durationPair.FindStringSubmatch(v)
		if m == nil {
			break
		}
		unit := func(words []string) bool {
			return slices.ContainsFunc(words, func(w string) bool {
				return strings.EqualFold(w, m[2]) || strings.EqualFold(w, strings.TrimSuffix(m[2], "."))
			})
		}
		rank, factor := 0, 0.0
		switch {
		case unit(lang.hourWords):
			rank, factor = 2, 60
		case unit(lang.minuteWords):
			rank, factor = 1, 1
		}
		if rank == 0 || rank >= last {
			break
		}
		n, _ := strconv.ParseFloat(strings.Replace(m[1], ",", ".", 1), 64)
		total, last, ok = total+n*factor, rank, true
		v = v[len(m[0]):]
	}
	return int(math.Round(min(total, 1440))), ok
}
