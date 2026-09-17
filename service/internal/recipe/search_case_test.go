package recipe_test

import (
	"context"
	"testing"

	"github.com/s-frei/rezepte/service/internal/recipe"
)

// TestSearchIsCaseInsensitive covers the review fix for variants (see
// search_internal_test.go): an upper-case ASCII-transliterated query term,
// or a capitalised umlaut term, must still find recipes indexed under the
// umlaut spelling.
func TestSearchIsCaseInsensitive(t *testing.T) {
	svc := seedAll(t)
	for _, q := range []string{"KAESE", "Kaesesp"} {
		p, err := svc.List(context.Background(), recipe.ListParams{Query: q, Page: 1, Limit: 24})
		if err != nil {
			t.Fatalf("%q: %v", q, err)
		}
		if p.Total < 1 || p.Items[0].Title != "Käsespätzle" {
			t.Fatalf("%q → %+v", q, p.Items)
		}
	}
}
