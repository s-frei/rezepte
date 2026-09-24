package recipe_test

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"golang.org/x/text/collate"
	"golang.org/x/text/language"

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

// TestListPutsTheNewestFirstWithinASecond pins the ORDER BY tiebreak.
// Timestamps are stored as RFC3339 with second resolution, so a burst of
// writes - demo seeding, a bulk import - shares one updated_at and is
// separated only by the id, which must run newest first like updated_at.
func TestListPutsTheNewestFirstWithinASecond(t *testing.T) {
	ctx := context.Background()
	svc, uid := setup(t)
	var created []string
	for _, in := range loadFixtures(t) {
		r, err := svc.Create(ctx, uid, in)
		if err != nil {
			t.Fatal(err)
		}
		created = append(created, r.Title)
	}
	p, err := svc.List(ctx, recipe.ListParams{Page: 1, Limit: 24})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Items) != len(created) {
		t.Fatalf("items = %d, want %d", len(p.Items), len(created))
	}
	for i, item := range p.Items {
		if want := created[len(created)-1-i]; item.Title != want {
			t.Fatalf("item %d = %q, want %q (reverse creation order)", i, item.Title, want)
		}
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
// or a capitalized umlaut term, must still find recipes indexed under the
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
	p, _ := svc.List(context.Background(), recipe.ListParams{Tags: []string{"fleisch"}, Page: 1, Limit: 24})
	if p.Total != 5 {
		t.Fatalf("fleisch = %d", p.Total)
	}
	p, _ = svc.List(context.Background(), recipe.ListParams{Tags: []string{"fleisch"}, Query: "schnitzel", Page: 1, Limit: 24})
	if p.Total != 1 {
		t.Fatalf("combined = %d", p.Total)
	}
}

func TestListFiltersByAllTags(t *testing.T) {
	svc := seedAll(t)
	ctx := context.Background()

	both, err := svc.List(ctx, recipe.ListParams{Tags: []string{"fleisch", "klassiker"}})
	if err != nil {
		t.Fatal(err)
	}
	if both.Total == 0 {
		t.Fatal("expected recipes carrying both tags")
	}
	for _, item := range both.Items {
		if !slices.Contains(item.Tags, "fleisch") || !slices.Contains(item.Tags, "klassiker") {
			t.Fatalf("recipe %q lacks one of the filtered tags: %v", item.Title, item.Tags)
		}
	}

	// Narrowing must never widen: two tags can only ever match a subset of one.
	one, err := svc.List(ctx, recipe.ListParams{Tags: []string{"fleisch"}})
	if err != nil {
		t.Fatal(err)
	}
	if both.Total > one.Total {
		t.Fatalf("two tags matched more (%d) than one (%d)", both.Total, one.Total)
	}
}

func TestListWithTagsThatShareNoRecipe(t *testing.T) {
	svc := seedAll(t)
	p, err := svc.List(context.Background(), recipe.ListParams{Tags: []string{"dessert", "fleisch"}})
	if err != nil {
		t.Fatal(err)
	}
	if p.Total != 0 || len(p.Items) != 0 {
		t.Fatalf("expected no matches, got %d", p.Total)
	}
}

func TestListCombinesTagsWithSearch(t *testing.T) {
	svc := seedAll(t)
	p, err := svc.List(context.Background(), recipe.ListParams{
		Query: "rind",
		Tags:  []string{"fleisch", "klassiker"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range p.Items {
		if !slices.Contains(item.Tags, "fleisch") || !slices.Contains(item.Tags, "klassiker") {
			t.Fatalf("recipe %q lacks one of the filtered tags: %v", item.Title, item.Tags)
		}
	}
}

// TestListTotalSurvivesPagination pins the count query against the mistake
// that hides best: a GROUP BY on the outer level makes COUNT(*) return one
// row per recipe, so Total reads 1 and "Mehr laden" silently disappears.
// Only a filter matching more recipes than one page holds exposes it.
func TestListTotalSurvivesPagination(t *testing.T) {
	svc := seedAll(t)
	ctx := context.Background()
	params := recipe.ListParams{Tags: []string{"klassiker"}, Limit: 1}

	first, err := svc.List(ctx, params)
	if err != nil {
		t.Fatal(err)
	}
	if first.Total < 2 {
		t.Fatalf("fixture must have at least two klassiker recipes, got %d", first.Total)
	}
	if len(first.Items) != 1 {
		t.Fatalf("limit 1 returned %d items", len(first.Items))
	}

	params.Page = 2
	second, err := svc.List(ctx, params)
	if err != nil {
		t.Fatal(err)
	}
	if second.Total != first.Total {
		t.Fatalf("total changed between pages: %d then %d", first.Total, second.Total)
	}
	if len(second.Items) != 1 || second.Items[0].ID == first.Items[0].ID {
		t.Fatal("page 2 must hold a different recipe")
	}
}

// TestListDeduplicatesTags pins the ruling that List, not just the handler,
// must normalize and de-duplicate Tags before deriving tag_count: the SQL
// matches on COUNT(DISTINCT t.name) = tag_count, so a caller passing the
// same tag twice would otherwise set tag_count to 2 against a distinct
// count of 1 and silently return nothing.
func TestListDeduplicatesTags(t *testing.T) {
	svc := seedAll(t)
	ctx := context.Background()

	dup, err := svc.List(ctx, recipe.ListParams{Tags: []string{"fleisch", "fleisch"}})
	if err != nil {
		t.Fatal(err)
	}
	once, err := svc.List(ctx, recipe.ListParams{Tags: []string{"fleisch"}})
	if err != nil {
		t.Fatal(err)
	}
	if dup.Total != once.Total {
		t.Fatalf("duplicate tag changed total: %d vs %d", dup.Total, once.Total)
	}
	if dup.Total == 0 {
		t.Fatal("fixture must have at least one fleisch recipe")
	}
}

func TestNormalizeTagQuery(t *testing.T) {
	got := recipe.NormalizeTagQuery([]string{" Fleisch ", "fleisch", "", "KLASSIKER"})
	want := []string{"fleisch", "klassiker"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// TestNormalizeTagQueryCapsAfterDeduplication pins maxTagFilters, the one
// guard in NormalizeTagQuery with a security shape: without it, a caller
// could make List build an arbitrarily wide subquery. It checks three
// things: more than the cap is truncated down to exactly the cap, the
// tags kept are the first ones supplied rather than an arbitrary subset,
// and the cap counts distinct tags rather than raw input length - 30
// copies of one tag must not be cut down the same way 30 distinct tags
// are.
func TestNormalizeTagQueryCapsAfterDeduplication(t *testing.T) {
	const wantCap = 20 // mirrors the unexported recipe.maxTagFilters

	distinct := make([]string, 30)
	for i := range distinct {
		distinct[i] = fmt.Sprintf("tag%02d", i)
	}
	got := recipe.NormalizeTagQuery(distinct)
	if len(got) != wantCap {
		t.Fatalf("got %d tags, want %d (the cap)", len(got), wantCap)
	}
	if !slices.Equal(got, distinct[:wantCap]) {
		t.Fatalf("cap kept the wrong tags: got %v, want the first %d of %v", got, wantCap, distinct)
	}

	duplicates := make([]string, 30)
	for i := range duplicates {
		duplicates[i] = "fleisch"
	}
	got = recipe.NormalizeTagQuery(duplicates)
	if want := []string{"fleisch"}; !slices.Equal(got, want) {
		t.Fatalf("cap must apply after de-duplication: got %v, want %v", got, want)
	}
}

func TestListFiltersByMaxMinutes(t *testing.T) {
	svc := seedAll(t)
	p, err := svc.List(context.Background(), recipe.ListParams{MaxMinutes: 45})
	if err != nil {
		t.Fatal(err)
	}
	if p.Total == 0 {
		t.Fatal("expected quick recipes in the fixture")
	}
	for _, item := range p.Items {
		if item.TotalMinutes == nil || *item.TotalMinutes > 45 {
			t.Fatalf("recipe %q exceeds the limit: %v", item.Title, item.TotalMinutes)
		}
	}
}

// TestListMaxMinutesExcludesUntimedRecipes pins the BETWEEN 1 clause: a
// recipe without any time set must not match, even against a generous
// limit, since "under N minutes" is a promise the data cannot keep for it.
func TestListMaxMinutesExcludesUntimedRecipes(t *testing.T) {
	ctx := context.Background()
	svc, uid := setup(t)
	in := loadFixtures(t)[0]
	in.PrepMinutes, in.CookMinutes = nil, nil
	if _, err := svc.Create(ctx, uid, in); err != nil {
		t.Fatal(err)
	}
	p, err := svc.List(ctx, recipe.ListParams{MaxMinutes: 600})
	if err != nil {
		t.Fatal(err)
	}
	if p.Total != 0 {
		t.Fatalf("untimed recipe must not match, got %d", p.Total)
	}
}

func TestListFiltersByFavorite(t *testing.T) {
	ctx := context.Background()
	svc, uid := setup(t)
	fixtures := loadFixtures(t)
	starred, err := svc.Create(ctx, uid, fixtures[0])
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(ctx, uid, fixtures[1]); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetFavorite(ctx, uid, starred.ID, true); err != nil {
		t.Fatal(err)
	}

	p, err := svc.List(ctx, recipe.ListParams{UserID: uid, FavoritesOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if p.Total != 1 || p.Items[0].ID != starred.ID {
		t.Fatalf("expected only the starred recipe, got %d", p.Total)
	}
}

// TestListFavoritesOnlyIsPerUser is the isolation check for the filter
// itself, not just the per-card flag: user A's favorites-only list must
// stay empty until user A (not user B) has starred something, even though
// both users can see and list the same underlying recipes.
func TestListFavoritesOnlyIsPerUser(t *testing.T) {
	ctx := context.Background()
	svc, uid := setup(t)
	other := createUser(t, "zweite@example.com")
	fixtures := loadFixtures(t)
	starred, err := svc.Create(ctx, uid, fixtures[0])
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(ctx, uid, fixtures[1]); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetFavorite(ctx, other, starred.ID, true); err != nil {
		t.Fatal(err)
	}

	p, err := svc.List(ctx, recipe.ListParams{UserID: uid, FavoritesOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if p.Total != 0 {
		t.Fatalf("user A must not see user B's favorites through the filter, got %d", p.Total)
	}

	p, err = svc.List(ctx, recipe.ListParams{UserID: other, FavoritesOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if p.Total != 1 || p.Items[0].ID != starred.ID {
		t.Fatalf("user B's own filter should still find it, got %d", p.Total)
	}
}

// TestFavoritesOnlyIsANoOpWithoutAUserID pins ListParams.UserID's "an
// empty user id disables the filter" guarantee for FavoritesOnly
// specifically: without it, the SQL condition's own comparison of user_id
// against an empty string happens to match no favorites row, which would
// silently turn FavoritesOnly: true into "return nothing" instead of
// switching the filter off - the exact failure mode List must not have,
// since nothing forces every future caller to also set UserID whenever it
// sets FavoritesOnly.
func TestFavoritesOnlyIsANoOpWithoutAUserID(t *testing.T) {
	ctx := context.Background()
	svc, uid := setup(t)
	if _, err := svc.Create(ctx, uid, loadFixtures(t)[0]); err != nil {
		t.Fatal(err)
	}
	p, err := svc.List(ctx, recipe.ListParams{FavoritesOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if p.Total != 1 {
		t.Fatalf("FavoritesOnly without a user id must be a no-op, not an empty result: got %d", p.Total)
	}
}

// TestListSortsByCreatedAt pins Sort: "created" against the default order,
// which sorts by updated_at instead. For never-updated recipes, creation
// order and update order are identical, so a test that only ever creates
// recipes cannot tell the two apart: the default order produces the same
// head even if Sort were ignored entirely. Updating the earliest-created recipe after
// all three exist makes its updated_at the newest while its created_at
// stays the oldest, which is what makes the two orderings actually diverge
// - then the whole sequence is checked under "created", and the default
// order is checked to differ, so the test fails if the sort stops being
// honored.
func TestListSortsByCreatedAt(t *testing.T) {
	ctx := context.Background()
	svc, uid := setup(t)
	fixtures := loadFixtures(t)
	var ids []string
	for _, in := range fixtures[:3] {
		r, err := svc.Create(ctx, uid, in)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, r.ID)
	}

	// Timestamps have one-second resolution, so the update below must land
	// in a later second than the creates above for updated_at to move at
	// all - otherwise it would tie with the others and fall back to the
	// same id tiebreak as created_at, hiding the very divergence this test
	// needs.
	time.Sleep(1100 * time.Millisecond)
	if _, err := svc.Update(ctx, ids[0], adminActor(uid), fixtures[0]); err != nil {
		t.Fatal(err)
	}

	wantCreated := []string{ids[2], ids[1], ids[0]}
	byCreated, err := svc.List(ctx, recipe.ListParams{Sort: "created"})
	if err != nil {
		t.Fatal(err)
	}
	if got := recipeIDs(byCreated.Items); !slices.Equal(got, wantCreated) {
		t.Fatalf("Sort: created = %v, want %v", got, wantCreated)
	}

	byDefault, err := svc.List(ctx, recipe.ListParams{})
	if err != nil {
		t.Fatal(err)
	}
	if got := recipeIDs(byDefault.Items); slices.Equal(got, wantCreated) {
		t.Fatalf("default order must differ from creation order once one recipe was updated, got %v", got)
	}
}

func recipeIDs(items []recipe.Card) []string {
	ids := make([]string, len(items))
	for i, item := range items {
		ids[i] = item.ID
	}
	return ids
}

func TestListSortsByTitle(t *testing.T) {
	svc := seedAll(t)
	p, err := svc.List(context.Background(), recipe.ListParams{Sort: "title"})
	if err != nil {
		t.Fatal(err)
	}
	c := collate.New(language.Und)
	for i := 1; i < len(p.Items); i++ {
		if c.CompareString(p.Items[i-1].Title, p.Items[i].Title) > 0 {
			t.Fatalf("%q sorted before %q", p.Items[i-1].Title, p.Items[i].Title)
		}
	}
}

// Titles sort the way a German reader expects: an umlaut sorts with its
// base letter and case does not matter, rather than every non-ASCII initial
// landing after "Z" in byte order.
func TestListSortsTitlesWithUmlautsAlphabetically(t *testing.T) {
	ctx := context.Background()
	svc, uid := setup(t)
	fixtures := loadFixtures(t)
	for i, title := range []string{"Zwiebelkuchen", "überbackene Nudeln", "Bratapfel", "Äpfel im Schlafrock"} {
		in := fixtures[i]
		in.Title = title
		if _, err := svc.Create(ctx, uid, in); err != nil {
			t.Fatal(err)
		}
	}
	p, err := svc.List(ctx, recipe.ListParams{Sort: "title"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Äpfel im Schlafrock", "Bratapfel", "überbackene Nudeln", "Zwiebelkuchen"}
	got := make([]string, len(p.Items))
	for i, item := range p.Items {
		got[i] = item.Title
	}
	if !slices.Equal(got, want) {
		t.Fatalf("Sort: title = %q, want %q", got, want)
	}
}

func TestListRejectsAnUnknownSort(t *testing.T) {
	svc := seedAll(t)
	p, err := svc.List(context.Background(), recipe.ListParams{Sort: "; DROP TABLE recipes"})
	if err != nil {
		t.Fatal(err)
	}
	// A service-level defense, not the API's behavior: the sort query
	// parameter is an enum, so huma answers 422 before List is reached. An
	// in-process caller still gets the default order rather than an error.
	if p.Total == 0 {
		t.Fatal("unknown sort must fall back to the default order")
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

// TestListCombinesFullTextTagsAndTimeFilters pins the one combination none
// of the other tests exercise: a full-text query, a tag filter, a time
// filter and a sort together, so every condition sqlc.RecipeFilter appends
// is checked beside the others, in the list and in the count.
//
// "fleisch" alone matches all five fleisch-tagged recipes through the FTS
// tags column (the same baseline TestFilterByTagAndCombined pins). Adding
// Tags: ["klassiker"] narrows that to the three recipes carrying both tags
// (Königsberger Klopse, Rinderrouladen, Sauerbraten), and MaxMinutes: 140
// then drops Sauerbraten (prep 30 + cook 150 = 180 minutes), leaving two -
// few enough to also pin Sort: "title" against a real ordering, not just a
// single-item result that could not tell a working sort from a broken one.
func TestListCombinesFullTextTagsAndTimeFilters(t *testing.T) {
	svc := seedAll(t)
	ctx := context.Background()

	onlyQuery, err := svc.List(ctx, recipe.ListParams{Query: "fleisch"})
	if err != nil {
		t.Fatal(err)
	}
	if onlyQuery.Total <= 1 {
		t.Fatalf("fixture must have more than one fleisch match for the query alone, got %d", onlyQuery.Total)
	}

	p, err := svc.List(ctx, recipe.ListParams{
		Query:      "fleisch",
		Tags:       []string{"klassiker"},
		MaxMinutes: 140,
		Sort:       "title",
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.Total >= onlyQuery.Total {
		t.Fatalf("filters must narrow the query-only match set: %d vs %d", p.Total, onlyQuery.Total)
	}
	wantSlugs := []string{"koenigsberger-klopse", "rinderrouladen"}
	if p.Total != len(wantSlugs) || len(p.Items) != len(wantSlugs) {
		t.Fatalf("combined filters = %+v (total %d), want %v", p.Items, p.Total, wantSlugs)
	}
	for i, slug := range wantSlugs {
		if p.Items[i].Slug != slug {
			t.Fatalf("item %d = %q, want %q (title order): %+v", i, p.Items[i].Slug, slug, p.Items)
		}
	}
}

// The overview shows who wrote a recipe, so a Card carries the names the
// same way the detail response does.
func TestCardsCarryAuthorNames(t *testing.T) {
	ctx := context.Background()
	svc, uid := setup(t)
	editor := createUser(t, "mara")
	created, err := svc.Create(ctx, uid, loadFixtures(t)[3])
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Update(ctx, created.ID, adminActor(editor), loadFixtures(t)[3]); err != nil {
		t.Fatal(err)
	}

	page, err := svc.List(ctx, recipe.ListParams{})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(page.Items))
	}
	if page.Items[0].CreatedBy.DisplayName != "sam" || page.Items[0].UpdatedBy.DisplayName != "mara" {
		t.Fatalf("names = %q / %q, want \"sam\" / \"mara\"",
			page.Items[0].CreatedBy.DisplayName, page.Items[0].UpdatedBy.DisplayName)
	}
}

// The "Angelegt von" filter offers the people who actually wrote something,
// so somebody who has only ever edited is not on the list.
func TestAuthorsListsWritersWithTheirCounts(t *testing.T) {
	ctx := context.Background()
	svc, uid := setup(t)
	editor := createUser(t, "mara")
	fx := loadFixtures(t)
	first, err := svc.Create(ctx, uid, fx[0])
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(ctx, uid, fx[1]); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Update(ctx, first.ID, adminActor(editor), fx[0]); err != nil {
		t.Fatal(err)
	}

	authors, err := svc.Authors(ctx)
	if err != nil {
		t.Fatalf("Authors: %v", err)
	}
	if len(authors) != 1 {
		t.Fatalf("authors = %+v, want only the one who wrote recipes", authors)
	}
	if authors[0].Username != "sam" || authors[0].Count != 2 {
		t.Fatalf("author = %+v, want sam with 2", authors[0])
	}

	if _, err := svc.Create(ctx, editor, fx[2]); err != nil {
		t.Fatal(err)
	}
	authors, err = svc.Authors(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Most recipes first, so the list reads like the tag list beside it.
	if len(authors) != 2 || authors[0].Username != "sam" || authors[1].Username != "mara" || authors[1].Count != 1 {
		t.Fatalf("authors = %+v, want sam(2) then mara(1)", authors)
	}
}

func TestAuthorFilterNarrowsToWhoWroteIt(t *testing.T) {
	ctx := context.Background()
	svc, uid := setup(t)
	other := createUser(t, "mara")
	fx := loadFixtures(t)
	sams, err := svc.Create(ctx, uid, fx[0])
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(ctx, other, fx[1]); err != nil {
		t.Fatal(err)
	}
	// sam wrote this one; mara only edited it, which must not move it into
	// her half of the split below.
	if _, err := svc.Update(ctx, sams.ID, adminActor(other), fx[0]); err != nil {
		t.Fatal(err)
	}

	all, err := svc.List(ctx, recipe.ListParams{})
	if err != nil {
		t.Fatal(err)
	}
	if all.Total != 2 {
		t.Fatalf("unfiltered total = %d, want 2", all.Total)
	}

	mine, err := svc.List(ctx, recipe.ListParams{Author: "sam"})
	if err != nil {
		t.Fatalf("List by author: %v", err)
	}
	if mine.Total != 1 || len(mine.Items) != 1 || mine.Items[0].Slug != sams.Slug {
		t.Fatalf("author=sam = %+v (total %d), want only %q", mine.Items, mine.Total, sams.Slug)
	}

	// An unknown name matches nobody rather than everybody - a stale link
	// must not silently drop the filter.
	nobody, err := svc.List(ctx, recipe.ListParams{Author: "niemand"})
	if err != nil {
		t.Fatal(err)
	}
	if nobody.Total != 0 {
		t.Fatalf("unknown author = %d, want 0", nobody.Total)
	}
}
