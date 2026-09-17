package recipe

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"github.com/s-frei/rezepte/service/internal/db/sqlc"
)

// maxSlugLength caps the length of a generated slug.
const maxSlugLength = 80

// umlauts transliterates German umlauts and ß the way German speakers
// spell them without diacritics, before other diacritics are stripped.
var umlauts = strings.NewReplacer(
	"ä", "ae", "ö", "oe", "ü", "ue", "ß", "ss",
	"Ä", "Ae", "Ö", "Oe", "Ü", "Ue", "ẞ", "Ss",
)

// Slugify turns title into a URL-safe slug: German umlauts and ß are
// transliterated first (ä→ae, ö→oe, ü→ue, ß→ss), remaining diacritics are
// stripped, the result is lower-cased, runs of characters outside [a-z0-9]
// become a single hyphen, leading and trailing hyphens are trimmed, and the
// result is capped at 80 characters.
func Slugify(title string) string {
	s := umlauts.Replace(title)
	s = stripDiacritics(s)
	s = strings.ToLower(s)

	var b strings.Builder
	prevDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		case !prevDash:
			b.WriteByte('-')
			prevDash = true
		}
	}
	s = strings.Trim(b.String(), "-")
	if len(s) > maxSlugLength {
		s = strings.Trim(s[:maxSlugLength], "-")
	}
	return s
}

// stripDiacritics removes combining marks left after Unicode NFD
// normalization, turning e.g. "é" into "e".
func stripDiacritics(s string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(s) {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// NormalizeTags trims and lower-cases each tag, drops empty results and
// removes duplicates while keeping the order of first occurrence.
func NormalizeTags(tags []string) []string {
	out := make([]string, 0, len(tags))
	seen := make(map[string]struct{}, len(tags))
	for _, t := range tags {
		t = strings.ToLower(strings.TrimSpace(t))
		if t == "" {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	return out
}

// uniqueSlug returns the first of base, base-2, base-3, ... not already
// used by another recipe.
func uniqueSlug(ctx context.Context, q *sqlc.Queries, base string) (string, error) {
	slug := base
	for i := 2; ; i++ {
		exists, err := q.SlugExists(ctx, slug)
		if err != nil {
			return "", fmt.Errorf("check slug %q: %w", slug, err)
		}
		if !exists {
			return slug, nil
		}
		slug = fmt.Sprintf("%s-%d", base, i)
	}
}
