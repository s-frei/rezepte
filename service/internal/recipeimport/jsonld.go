package recipeimport

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	xhtml "golang.org/x/net/html"
)

// ReadPage finds the schema.org Recipe in a page's JSON-LD and reads it,
// with the language table the page declares (see pageLanguage) or, failing
// that, the one its ingredient lines fit best.
func ReadPage(body []byte, base *url.URL) (Page, bool) {
	blocks, meta := scan(body)
	var rec map[string]any
	var block any
	ids := map[string]map[string]any{}
	for _, b := range blocks {
		if v, ok := decodeLenient(b); ok {
			if rec = findRecipe(v); rec != nil {
				block = v
				indexIDs(v, ids)
				break
			}
		}
	}
	if rec == nil {
		return Page{}, false
	}
	ingredients := cleanAll(list(rec["recipeIngredient"]))
	lang := detectLanguage(pageLanguage(rec, block, meta), ingredients)
	p := Page{
		Title:       stripAuthorFromTitle(clean(text(rec["name"])), authorName(rec["author"], ids)),
		Description: removePromo(clean(text(rec["description"])), lang),
		Language:    lang.tag,
		Servings:    servings(rec["recipeYield"]),
		PrepMinutes: ParseDuration(text(rec["prepTime"])),
		CookMinutes: ParseDuration(text(rec["cookTime"])),
		Ingredients: ingredients,
		Steps:       instructions(rec["recipeInstructions"]),
		Keywords:    keywords(rec),
		Authors:     authorNames(rec["author"], ids),
		PhotoURL:    resolve(base, photo(rec["image"], ids)),
		SourceURL:   sourceURL(base, meta),
		SourceName:  firstNonEmpty(meta["og:site_name"], clean(text(mapOf(rec["publisher"])["name"]))),
	}
	if p.CookMinutes == nil && p.PrepMinutes == nil {
		p.CookMinutes = ParseDuration(text(rec["totalTime"]))
	}
	return p, true
}

// pageLanguage returns the language tag a page declares, as written: the
// Recipe's inLanguage, else a WebPage node's in the same JSON-LD, else
// <html lang>; "" when none does.
// pageLanguage({"inLanguage": "de-DE"}, …) = "de-DE".
func pageLanguage(rec map[string]any, block any, meta map[string]string) string {
	return firstNonEmpty(text(rec["inLanguage"]), webPageLanguage(block), meta["lang"])
}

// webPageLanguage finds the inLanguage of the first WebPage node in v.
func webPageLanguage(v any) string {
	switch t := v.(type) {
	case []any:
		for _, e := range t {
			if s := webPageLanguage(e); s != "" {
				return s
			}
		}
	case map[string]any:
		if isType(t["@type"], "WebPage") {
			if s := text(t["inLanguage"]); s != "" {
				return s
			}
		}
		return webPageLanguage(t["@graph"])
	}
	return ""
}

// authorName resolves a Recipe's author to its name, following an @id
// reference to the Person or Organization node.
// authorName({"@id": "#a"}, {"#a": {"name": "KochFan42"}}) = "KochFan42".
func authorName(v any, ids map[string]map[string]any) string {
	m := mapOf(v)
	if m == nil {
		return clean(text(v))
	}
	if id, ok := m["@id"].(string); ok && len(m) == 1 && ids[id] != nil {
		m = ids[id]
	}
	return clean(text(m["name"]))
}

// authorNames resolves every author of a Recipe (one or a list) to a name.
// authorNames([{"name": "A"}, {"name": "B"}], ids) = ["A", "B"].
func authorNames(v any, ids map[string]map[string]any) []string {
	items, ok := v.([]any)
	if !ok {
		items = []any{v}
	}
	var out []string
	for _, a := range items {
		if n := authorName(a, ids); n != "" {
			out = append(out, n)
		}
	}
	return out
}

// stripAuthorFromTitle removes the author's name from the end of a title,
// as sites like Chefkoch append it, with the word before it ("von", "by",
// "de") and separators. It changes the title only on an exact match of the
// name, keeps a capitalized word before it (that one belongs to the
// title), and never leaves an empty title.
// stripAuthorFromTitle("Gurkensalat von KochFan42", "KochFan42") = "Gurkensalat".
func stripAuthorFromTitle(title, author string) string {
	if author == "" {
		return title
	}
	rest, ok := strings.CutSuffix(title, " "+author)
	if !ok {
		return title
	}
	rest = strings.TrimSpace(rest)
	i := strings.LastIndexByte(rest, ' ')
	if r, _ := utf8.DecodeRuneInString(rest[i+1:]); !unicode.IsUpper(r) {
		rest = rest[:max(i, 0)]
	}
	if rest = strings.TrimRight(rest, " -–—|:,·"); rest == "" {
		return title
	}
	return rest
}

// scan returns the text of every ld+json script and the meta the import
// uses: canonical, og:url, og:site_name, and <html lang> as "lang".
func scan(body []byte) ([]string, map[string]string) {
	meta := map[string]string{}
	var blocks []string
	z := xhtml.NewTokenizer(bytes.NewReader(body))
	inLD := false
	for {
		switch z.Next() {
		case xhtml.ErrorToken:
			return blocks, meta
		case xhtml.StartTagToken, xhtml.SelfClosingTagToken:
			t := z.Token()
			a := attrs(t)
			switch t.Data {
			case "html":
				meta["lang"] = a["lang"]
			case "script":
				inLD = strings.EqualFold(strings.TrimSpace(a["type"]), "application/ld+json")
			case "link":
				if strings.EqualFold(a["rel"], "canonical") {
					meta["canonical"] = a["href"]
				}
			case "meta":
				if p := a["property"]; p == "og:url" || p == "og:site_name" {
					meta[p] = a["content"]
				}
			}
		case xhtml.TextToken:
			if inLD {
				blocks = append(blocks, string(z.Text()))
			}
		case xhtml.EndTagToken:
			inLD = false
		}
	}
}

func attrs(t xhtml.Token) map[string]string {
	m := make(map[string]string, len(t.Attr))
	for _, a := range t.Attr {
		m[strings.ToLower(a.Key)] = a.Val
	}
	return m
}

var trailingComma = regexp.MustCompile(`,\s*([}\]])`)

// decodeLenient decodes JSON-LD, repairing the two mistakes sites make most:
// trailing commas and raw control characters inside strings.
func decodeLenient(s string) (any, bool) {
	var v any
	if json.Unmarshal([]byte(s), &v) == nil {
		return v, true
	}
	s = escapeControls(s)
	s = trailingComma.ReplaceAllString(s, "$1")
	v = nil // the failed decode may have filled part of it
	return v, json.Unmarshal([]byte(s), &v) == nil
}

// escapeControls escapes raw control characters inside JSON strings as
// \u00XX, keeping line breaks so instruction strings still split into steps.
func escapeControls(s string) string {
	var b strings.Builder
	inStr, esc := false, false
	for _, r := range s {
		switch {
		case esc:
			esc = false
		case r == '\\' && inStr:
			esc = true
		case r == '"':
			inStr = !inStr
		case inStr && r < 0x20:
			fmt.Fprintf(&b, `\u%04x`, r)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func findRecipe(v any) map[string]any {
	switch t := v.(type) {
	case []any:
		for _, e := range t {
			if r := findRecipe(e); r != nil {
				return r
			}
		}
	case map[string]any:
		if isRecipe(t["@type"]) {
			return t
		}
		for _, k := range []string{"@graph", "mainEntity", "mainEntityOfPage", "itemListElement"} {
			if r := findRecipe(t[k]); r != nil {
				return r
			}
		}
	}
	return nil
}

func isRecipe(typ any) bool {
	return slices.ContainsFunc(list(typ), func(s string) bool { return s == "Recipe" || strings.HasSuffix(s, "/Recipe") })
}

// text reads a JSON-LD value as one string: strings as is, numbers
// formatted, objects by their text/name/@value, lists by their first item.
func text(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case []any:
		if len(t) > 0 {
			return text(t[0])
		}
	case map[string]any:
		for _, k := range []string{"text", "name", "@value", "url"} {
			if s := text(t[k]); s != "" {
				return s
			}
		}
	}
	return ""
}

// list reads a value that may be one item or many.
func list(v any) []string {
	switch t := v.(type) {
	case nil:
		return nil
	case []any:
		out := make([]string, 0, len(t))
		for _, e := range t {
			if s := text(e); s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		if s := text(t); s != "" {
			return []string{s}
		}
	}
	return nil
}

func mapOf(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	if l, ok := v.([]any); ok && len(l) > 0 {
		return mapOf(l[0])
	}
	return nil
}

// blockTags separate words; inline tags like <b> do not ("<b>Ba</b>con").
var blockTags = map[string]bool{"br": true, "p": true, "li": true, "div": true}

// clean strips tags, decodes entities and folds whitespace.
func clean(s string) string {
	if strings.ContainsAny(s, "<&") {
		var b strings.Builder
		z := xhtml.NewTokenizer(strings.NewReader(s))
		for tt := z.Next(); tt != xhtml.ErrorToken; tt = z.Next() {
			switch tt {
			case xhtml.TextToken:
				b.Write(z.Text())
			case xhtml.StartTagToken, xhtml.EndTagToken, xhtml.SelfClosingTagToken:
				if name, _ := z.TagName(); blockTags[string(name)] {
					b.WriteByte(' ')
				}
			}
		}
		s = html.UnescapeString(b.String())
	}
	// Whitespace folds into single spaces; any other control character goes.
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && !unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)
	return strings.Join(strings.Fields(s), " ")
}

func cleanAll(ss []string) []string {
	out := ss[:0]
	for _, s := range ss {
		if c := clean(s); c != "" {
			out = append(out, c)
		}
	}
	return out
}

var firstInt = regexp.MustCompile(`\d+`)

func servings(v any) int {
	for _, s := range list(v) {
		if m := firstInt.FindString(s); m != "" {
			if n, err := strconv.Atoi(m); err == nil && n >= 1 && n <= 99 {
				return n
			}
		}
	}
	return 0
}

var durationRE = regexp.MustCompile(`^P(?:(\d+)D)?T?(?:(\d+)D)?(?:(\d+)H)?(?:(\d+)M)?(?:\d+S)?$`)

// ParseDuration reads an ISO 8601 duration in minutes, tolerating the
// "PT0D1H" shape some plugins emit. nil when it is not a duration.
func ParseDuration(s string) *int {
	s = strings.ToUpper(strings.TrimSpace(s))
	m := durationRE.FindStringSubmatch(s)
	if m == nil || !strings.ContainsAny(s, "0123456789") { // "P" and "PT" alone say nothing
		return nil
	}
	n := func(i int) int { v, _ := strconv.Atoi(m[i]); return v }
	minutes := (n(1)+n(2))*1440 + n(3)*60 + n(4)
	return &minutes
}

// instructions flattens a string, a list of strings, HowToSteps and
// HowToSections into step texts. A section's name prefixes its first step
// when there are two or more sections; a lone one ("Zubereitung") says
// nothing the steps do not.
func instructions(v any) []string {
	named := countSections(v) >= 2
	var out []string
	var walk func(v any, prefix string)
	walk = func(v any, prefix string) {
		switch t := v.(type) {
		case string:
			for _, part := range splitSteps(t) {
				out = append(out, prefix+part)
				prefix = ""
			}
		case []any:
			for _, e := range t {
				before := len(out)
				walk(e, prefix)
				if len(out) > before {
					prefix = ""
				}
			}
		case map[string]any:
			if isType(t["@type"], "HowToSection") {
				name := clean(text(t["name"]))
				p := prefix
				if name != "" && named {
					p = name + ": "
				}
				walk(t["itemListElement"], p)
				return
			}
			s := clean(text(t["text"]))
			if s == "" {
				s = clean(text(t["name"]))
			}
			if s != "" {
				out = append(out, prefix+s)
			}
		}
	}
	walk(v, "")
	return out
}

// countSections counts the HowToSection nodes in recipeInstructions.
func countSections(v any) int {
	switch t := v.(type) {
	case []any:
		n := 0
		for _, e := range t {
			n += countSections(e)
		}
		return n
	case map[string]any:
		if isType(t["@type"], "HowToSection") {
			return 1 + countSections(t["itemListElement"])
		}
	}
	return 0
}

func isType(v any, want string) bool {
	return slices.Contains(list(v), want)
}

var (
	lineBreak  = regexp.MustCompile(`(?i)<br\s*/?>|</p\s*>|</li\s*>|\r?\n`)
	stepMarker = regexp.MustCompile(`^\d+[.)]\s+`)
	inlineMark = regexp.MustCompile(`\s+\d+[.)]\s+`)
)

// splitSteps turns a plain-string instruction into one step per line (HTML
// breaks count as lines), dropping "1." / "2)" markers. A single line of the
// form "1. a 2. b 3. c" splits on the inline markers.
func splitSteps(raw string) []string {
	var lines []string
	for _, l := range lineBreak.Split(raw, -1) {
		if l = clean(l); l != "" {
			lines = append(lines, l)
		}
	}
	if len(lines) == 0 {
		return nil
	}
	if len(lines) == 1 && strings.HasPrefix(lines[0], "1") && stepMarker.MatchString(lines[0]) {
		lines = cleanAll(inlineMark.Split(lines[0], -1))
	}
	if strings.HasPrefix(lines[0], "1") && stepMarker.MatchString(lines[0]) {
		for i, l := range lines {
			lines[i] = stepMarker.ReplaceAllString(l, "")
		}
	}
	return lines
}

func keywords(rec map[string]any) []string {
	var raw []string
	for _, k := range []string{"keywords", "recipeCategory", "recipeCuisine"} {
		for _, s := range list(rec[k]) {
			raw = append(raw, strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ';' })...)
		}
	}
	seen := map[string]bool{}
	var out []string
	for _, s := range raw {
		s = clean(s)
		if s == "" || seen[strings.ToLower(s)] {
			continue
		}
		seen[strings.ToLower(s)] = true
		out = append(out, s)
	}
	return out
}

// indexIDs collects every node with an @id and more than that, so a
// reference like {"@id": "...#primaryimage"} can be followed.
func indexIDs(v any, ids map[string]map[string]any) {
	switch t := v.(type) {
	case []any:
		for _, e := range t {
			indexIDs(e, ids)
		}
	case map[string]any:
		if id, ok := t["@id"].(string); ok && len(t) > 1 {
			ids[id] = t
		}
		for _, e := range t {
			indexIDs(e, ids)
		}
	}
}

// photo picks the widest declared image, else the first.
func photo(v any, ids map[string]map[string]any) string {
	best, bestW := "", -1
	var walk func(v any)
	walk = func(v any) {
		switch t := v.(type) {
		case string:
			if best == "" {
				best = t
			}
		case []any:
			for _, e := range t {
				walk(e)
			}
		case map[string]any:
			if id, ok := t["@id"].(string); ok && len(t) == 1 {
				if n := ids[id]; n != nil {
					walk(n)
				}
				return
			}
			u := text(t["url"])
			if u == "" {
				u = text(t["contentUrl"])
			}
			w, _ := strconv.Atoi(text(t["width"]))
			if u != "" && (w > bestW || best == "") {
				best, bestW = u, w
			}
		}
	}
	walk(v)
	return best
}

func resolve(base *url.URL, ref string) string {
	if ref == "" {
		return ""
	}
	u, err := base.Parse(strings.TrimSpace(ref))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return ""
	}
	return u.String()
}

// sourceURL prefers the page's canonical address when it is on the same
// site as the address the member entered.
func sourceURL(base *url.URL, meta map[string]string) string {
	for _, k := range []string{"canonical", "og:url"} {
		if c := resolve(base, meta[k]); c != "" {
			if u, _ := url.Parse(c); u != nil && sameSite(u.Hostname(), base.Hostname()) {
				return c
			}
		}
	}
	return base.String()
}

func sameSite(a, b string) bool {
	return strings.TrimPrefix(a, "www.") == strings.TrimPrefix(b, "www.")
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s = strings.TrimSpace(s); s != "" {
			return s
		}
	}
	return ""
}
