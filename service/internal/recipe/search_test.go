package recipe_test

import (
	"context"
	"testing"

	"github.com/s-frei/rezepte/service/internal/recipe"
)

func seedAll(t *testing.T) *recipe.Service {
	t.Helper()
	svc, uid := setup(t)
	for _, in := range loadFixtures(t) {
		if _, err := svc.Create(context.Background(), uid, in); err != nil {
			t.Fatal(err)
		}
	}
	return svc
}

func TestListPaginates(t *testing.T) {
	svc := seedAll(t)
	p, err := svc.List(context.Background(), recipe.ListParams{Page: 2, Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if p.Total != 12 || len(p.Items) != 5 || p.Page != 2 {
		t.Fatalf("page = %+v", p)
	}
	if p.Items[0].Tags == nil {
		t.Fatal("tags must be an empty array, not null")
	}
}

func TestSearchPrefixAndDiacritics(t *testing.T) {
	svc := seedAll(t)
	for _, q := range []string{"kaese", "Käsesp", "spätzle röst"} {
		p, err := svc.List(context.Background(), recipe.ListParams{Query: q, Page: 1, Limit: 24})
		if err != nil {
			t.Fatalf("%q: %v", q, err)
		}
		if p.Total < 1 || p.Items[0].Title != "Käsespätzle" {
			t.Fatalf("%q → %+v", q, p.Items)
		}
	}
	p, _ := svc.List(context.Background(), recipe.ListParams{Query: "zzzz", Page: 1, Limit: 24})
	if p.Total != 0 || len(p.Items) != 0 {
		t.Fatalf("no results expected: %+v", p)
	}
}

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

func TestSearchMatchesIngredientsAndTags(t *testing.T) {
	svc := seedAll(t)
	p, _ := svc.List(context.Background(), recipe.ListParams{Query: "kapern", Page: 1, Limit: 24})
	if p.Total != 1 || p.Items[0].Slug != "koenigsberger-klopse" {
		t.Fatalf("ingredient search: %+v", p.Items)
	}
	p, _ = svc.List(context.Background(), recipe.ListParams{Query: "schwäbisch", Page: 1, Limit: 24})
	if p.Total != 2 {
		t.Fatalf("tag search: %+v", p.Items)
	}
}

func TestFilterByTagAndCombined(t *testing.T) {
	svc := seedAll(t)
	p, _ := svc.List(context.Background(), recipe.ListParams{Tag: "fleisch", Page: 1, Limit: 24})
	if p.Total != 5 {
		t.Fatalf("fleisch = %d", p.Total)
	}
	p, _ = svc.List(context.Background(), recipe.ListParams{Tag: "fleisch", Query: "schnitzel", Page: 1, Limit: 24})
	if p.Total != 1 {
		t.Fatalf("combined = %d", p.Total)
	}
}

func TestTagsWithCounts(t *testing.T) {
	svc := seedAll(t)
	tags, err := svc.Tags(context.Background())
	if err != nil || len(tags) == 0 {
		t.Fatalf("tags: %v %v", tags, err)
	}
	if tags[0].Name != "fleisch" || tags[0].Count != 5 {
		t.Fatalf("first tag = %+v", tags[0])
	}
}
