package recipe

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/s-frei/rezepte/service/internal/db"
	"github.com/s-frei/rezepte/service/internal/db/sqlc"
)

// Bounds applied to ListParams.Limit by (*Service).List.
const (
	defaultListLimit = 24
	maxListLimit     = 100
)

// ListParams filters and paginates (*Service).List. Query runs a full-text
// search across title, description, ingredient names and tags; Tags keeps
// only recipes carrying *every* listed tag; MaxMinutes keeps only recipes
// whose prep and cook time sum to at most that many minutes, excluding
// recipes with neither time set (0 = off). When several are set they
// combine with AND. Sort orders the result: "created" (newest created
// first), "title" (A-Z), "tasty" (most tasty marks first, ties most recently
// updated first), or anything else including "" (most recently updated
// first, the default; with a Query, the best hits first). Limit is clamped to [1, 100] (0 defaults to
// 24); Page is clamped to at least 1.
//
// UserID is the caller's id, used to fill Card.Favorite for the returned
// page and, when FavoritesOnly is set, to narrow the result to that
// caller's own favorites. The handler sets it from auth.UserFrom(ctx) on
// every list request, not only when a favorites filter is active -
// otherwise the star would be wrong on every card. An empty UserID
// disables both the favorite lookup and the filter rather than matching
// rows, so unauthenticated paths such as demo mode go through the same
// code without querying on a caller's behalf.
type ListParams struct {
	Query         string
	Tags          []string
	MaxMinutes    int
	FavoritesOnly bool
	// Author narrows to the recipes this username wrote. Empty switches
	// the filter off; an unknown name matches nothing, so a stale link
	// shows an empty grid rather than silently dropping the filter.
	Author string
	// Ingredients are the ingredient chips, raw names: set, the list holds
	// only recipes that use at least one and ranks them by how many (see
	// listPantry). Have are the ingredients ticked as at hand, raw names;
	// they lower a recipe's missing count, nothing else.
	Ingredients []string
	Have        []string
	Sort        string
	Page        int
	Limit       int
	UserID      string
}

// Page is one page of recipe cards plus the pagination state used to
// produce it.
type Page struct {
	Items []Card `json:"items"`
	Page  int    `json:"page"`
	Limit int    `json:"limit"`
	Total int    `json:"total"`
	// Sections counts a search's results by the section each card's Match
	// names, over all pages; it is there only on a search in the best-match
	// order, the one that is ordered by section.
	Sections map[string]int `json:"sections,omitempty" required:"false"`
	// Pantry is there only when ListParams.Ingredients is set; Sections is
	// then left out.
	Pantry *PagePantry `json:"pantry,omitempty" required:"false"`
}

// filter turns the narrowing half of p into the one RecipeFilter that List
// and TagsFor both query with, so the grid and its tag counts can never
// disagree about what a filter means.
func (p ListParams) filter() sqlc.RecipeFilter {
	// Normalized, de-duplicated and capped here, the one place every caller
	// passes through: the SQL matches on COUNT(DISTINCT t.name) = len(Tags),
	// so a duplicate tag would silently match nothing.
	words := searchWords(p.Query)
	f := sqlc.RecipeFilter{
		Search: searchTerms(words),
		Phrase: db.FoldWords(p.Query),
		Tags:   normalizeTagQuery(p.Tags),
		Author: p.Author,
	}
	// A negative bound is ignored like an invalid Limit. There is no upper
	// clamp: an oversized value just fails to narrow anything further.
	if p.MaxMinutes > 0 {
		f.MaxMinutes = int64(p.MaxMinutes)
	}
	// Without a caller there are no favorites to narrow to, so the filter is
	// off rather than matching nothing (see ListParams.UserID).
	if p.FavoritesOnly {
		f.FavoritesOf = p.UserID
	}
	return f
}

// List returns a page of recipe cards, most recently updated first or, for a
// search, best hits first, optionally narrowed by full-text search query
// and/or tags.
func (s *Service) List(ctx context.Context, p ListParams) (Page, error) {
	limit := p.Limit
	if limit == 0 {
		limit = defaultListLimit
	}
	limit = min(max(limit, 1), maxListLimit)
	page := max(p.Page, 1)
	offset := int64((page - 1) * limit)

	f := p.filter()
	sort := normalizeSort(p.Sort)
	if chips := pantryItems(p.Ingredients, maxChips, nil); len(chips) > 0 {
		return s.listPantry(ctx, p, chips, f, sort, limit, page)
	}
	rows, err := s.q.ListRecipes(ctx, f, sort, int64(limit), offset)
	if err != nil {
		return Page{}, fmt.Errorf("list recipes: %w", err)
	}
	total, err := s.q.CountRecipes(ctx, f)
	if err != nil {
		return Page{}, fmt.Errorf("count recipes: %w", err)
	}
	items, err := s.toCards(ctx, rows, p.UserID)
	if err != nil {
		return Page{}, err
	}
	if err := s.matchCards(ctx, items, rows, searchWords(p.Query), nil); err != nil {
		return Page{}, err
	}
	// Only the best-match order runs section by section; in any other a
	// section's cards are spread over the list.
	var sections map[string]int
	if sort == sqlc.RecipeSortUpdated {
		counts, err := s.q.CountSections(ctx, f)
		if err != nil {
			return Page{}, fmt.Errorf("count sections: %w", err)
		}
		for tier, n := range counts {
			if sections == nil {
				sections = map[string]int{}
			}
			sections[tierSections[tier]] = int(n)
		}
	}
	return Page{Items: items, Page: page, Limit: limit, Total: int(total), Sections: sections}, nil
}

// normalizeSort maps sort to one of the orders sqlc.ListRecipes knows,
// falling back to the default, most recently updated first, for anything
// else. ListRecipes falls back the same way on its own; normalizing here is
// about a predictable API, since sort only ever selects a fixed ORDER BY.
func normalizeSort(sort string) sqlc.RecipeSort {
	switch s := sqlc.RecipeSort(sort); s {
	case sqlc.RecipeSortCreated, sqlc.RecipeSortTitle, sqlc.RecipeSortTasty:
		return s
	default:
		return sqlc.RecipeSortUpdated
	}
}

// maxTagFilters caps the number of tags normalizeTagQuery keeps - the same
// ceiling Input.Tags carries as maxItems:"20".
const maxTagFilters = 20

// normalizeTagQuery trims and lower-cases each tag filter so "?tags=Fleisch"
// or "?tags= fleisch " match the lower-cased tag names NormalizeTags stores.
// Blanks are dropped, duplicates collapsed, and the result capped at
// maxTagFilters - a caller passing 500 tags would otherwise build a 500-way
// subquery.
func normalizeTagQuery(tags []string) []string {
	out := make([]string, 0, len(tags))
	seen := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		name := strings.ToLower(strings.TrimSpace(tag))
		if name == "" {
			continue
		}
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
		if len(out) == maxTagFilters {
			break
		}
	}
	return out
}

// matchCards fills Card.Match for a page of search results, from the rows
// they were built from: the title, the tags already on each card, the
// ingredient names, and the description. names holds the ingredient names
// by recipe id when the caller has them already; nil loads them for the
// page in one batch. Without search words it does nothing and costs no
// query.
func (s *Service) matchCards(ctx context.Context, cards []Card, rows []sqlc.Recipe, words []string, names map[string][]string) error {
	if len(words) == 0 || len(cards) == 0 {
		return nil
	}
	if names == nil {
		ids := make([]string, len(rows))
		for i, r := range rows {
			ids[i] = r.ID
		}
		var err error
		if names, err = s.ingredientNames(ctx, ids); err != nil {
			return err
		}
	}
	mw := matchWords(words)
	for i, r := range rows {
		cards[i].Match = matchCard(mw, r.Title, cards[i].Tags, names[r.ID], r.Description)
	}
	return nil
}

// ingredientNames loads the ingredient names of the recipes ids name, by
// recipe id, in one batch.
func (s *Service) ingredientNames(ctx context.Context, ids []string) (map[string][]string, error) {
	rows, err := s.q.ListIngredientNamesForRecipes(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("list ingredient names for recipes: %w", err)
	}
	names := make(map[string][]string, len(ids))
	for _, r := range rows {
		names[r.RecipeID] = append(names[r.RecipeID], r.Name)
	}
	return names, nil
}

// toCards loads the tags, the authors' display names and colors, the tasty
// counts and, when userID is set, the caller's favorite and tasty state for
// rows in one batch each and
// assembles them into Cards, in the same order as rows. It always returns a
// non-nil slice.
//
// An empty userID disables the favorite batch entirely rather than
// querying with an empty id: unauthenticated paths such as demo mode call
// List the same way authenticated ones do, and must not have every card
// come back favorited by matching favorites rows that happen to carry no
// user id.
func (s *Service) toCards(ctx context.Context, rows []sqlc.Recipe, userID string) ([]Card, error) {
	items := make([]Card, 0, len(rows))
	if len(rows) == 0 {
		return items, nil
	}
	ids := make([]string, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}
	tagRows, err := s.q.ListTagNamesForRecipes(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("list tag names for recipes: %w", err)
	}
	tagsByRecipe := make(map[string][]string, len(rows))
	for _, tr := range tagRows {
		tagsByRecipe[tr.RecipeID] = append(tagsByRecipe[tr.RecipeID], tr.Name)
	}

	// Both author columns of the page in one lookup: most pages are written
	// by a handful of people, so the set of ids is far smaller than the set
	// of rows.
	userIDs := make([]string, 0, 2*len(rows))
	for _, r := range rows {
		userIDs = append(userIDs, r.CreatedBy, r.UpdatedBy)
	}
	authors, err := s.peopleByID(ctx, userIDs)
	if err != nil {
		return nil, err
	}

	favorites := make(map[string]bool)
	tasty := make(map[string]bool)
	newComments := make(map[string]bool)
	if userID != "" {
		if favorites, err = s.favoritesOf(ctx, userID, ids); err != nil {
			return nil, err
		}
		if tasty, err = s.tastyOf(ctx, userID, ids); err != nil {
			return nil, err
		}
		if newComments, err = s.newCommentsFor(ctx, userID, ids); err != nil {
			return nil, err
		}
	}
	// The counts are the same for every caller, so they are loaded with or
	// without one.
	tastyCounts, err := s.tastyCounts(ctx, ids)
	if err != nil {
		return nil, err
	}
	statRows, err := s.q.ImageStatsForRecipes(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("image stats for recipes: %w", err)
	}
	stats := make(map[string]sqlc.ImageStatsForRecipesRow, len(statRows))
	for _, st := range statRows {
		stats[st.RecipeID] = st
	}
	coverRows, err := s.q.CoverSizesForRecipes(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("cover sizes for recipes: %w", err)
	}
	covers := make(map[string]sqlc.CoverSizesForRecipesRow, len(coverRows))
	for _, c := range coverRows {
		covers[c.RecipeID] = c
	}

	for _, r := range rows {
		card, err := toCard(r, tagsByRecipe[r.ID], favorites[r.ID],
			authors[r.CreatedBy], authors[r.UpdatedBy])
		if err != nil {
			return nil, err
		}
		card.TastyCount = tastyCounts[r.ID]
		card.Tasty = tasty[r.ID]
		card.NewComments = newComments[r.ID]
		card.ImageCount = int(stats[r.ID].ImageCount)
		card.ImageBytes = stats[r.ID].ImageBytes
		if c, ok := covers[r.ID]; ok {
			w, h := int(c.Width), int(c.Height)
			card.CoverWidth, card.CoverHeight = &w, &h
		}
		items = append(items, card)
	}
	return items, nil
}

// toCard builds a Card from a stored recipe row, its tag names, whether the
// caller has favorited it and the display name/color pair behind each of
// its two author columns.
func toCard(row sqlc.Recipe, tags []string, favorite bool, createdBy, updatedBy Person) (Card, error) {
	if tags == nil {
		tags = []string{}
	}
	updated, err := db.ParseTime(row.UpdatedAt)
	if err != nil {
		return Card{}, err
	}
	return Card{
		ID:           row.ID,
		Slug:         row.Slug,
		Title:        row.Title,
		Tags:         tags,
		TotalMinutes: totalMinutes(row.PrepMinutes, row.CookMinutes),
		CoverImageID: row.CoverImageID,
		UpdatedAt:    updated,
		Favorite:     favorite,

		CreatedBy: createdBy,
		UpdatedBy: updatedBy,
	}, nil
}

// totalMinutes adds prep and cook minutes when at least one is set,
// returning nil when both are unset.
func totalMinutes(prep, cook *int64) *int {
	if prep == nil && cook == nil {
		return nil
	}
	var total int64
	if prep != nil {
		total += *prep
	}
	if cook != nil {
		total += *cook
	}
	t := int(total)
	return &t
}

// searchWords splits what the user typed into words at anything that is not
// a letter or a digit, as the unicode61 tokenizer does, and folds each the
// way the search indexes hold their text (db.Fold): "Käse" and "kaese" are
// both "kase". The split also leaves no quote or FTS5 operator in a word.
// A word whose stem an earlier word already has is dropped: "zwiebel
// Zwiebeln" would otherwise score twice and show two identical finds.
func searchWords(q string) []string {
	var words []string
	seen := map[string]bool{}
	for _, w := range strings.Fields(db.FoldWords(q)) {
		if s := stem(w); !seen[s] {
			seen[s] = true
			words = append(words, w)
		}
	}
	return words
}

// stem drops a German plural or inflection ending from a word of five
// letters or more - "en", else "n" or "e" - so "tomaten" finds "Tomate" and
// "äpfel", folded to "apfel", finds "Apfel". What is left still starts the
// longer form, so nothing that the whole word found is lost. ponytail: a
// suffix cut, not a stemmer; a Snowball stemmer conflates too much for
// recipes ("Küche" and "Kuchen").
func stem(word string) string {
	if utf8.RuneCountInString(word) < 5 {
		return word
	}
	if s, ok := strings.CutSuffix(word, "en"); ok {
		return s
	}
	return strings.TrimSuffix(strings.TrimSuffix(word, "n"), "e")
}

// searchTerms renders each word as the MATCH expressions sqlc.SearchTerm
// holds, quoted as FTS5 strings: Exact as typed, Prefix stemmed with "*",
// and Trigram stemmed, for the trigram index, where a quoted string matches
// anywhere in the text - unless the stem is shorter than the three
// characters a trigram needs. No words returns nil: no search, not one that
// matches nothing.
func searchTerms(words []string) []sqlc.SearchTerm {
	var terms []sqlc.SearchTerm
	for _, word := range words {
		s := stem(word)
		term := sqlc.SearchTerm{Exact: quote(word), Prefix: quote(s) + "*"}
		if utf8.RuneCountInString(s) >= 3 {
			term.Trigram = quote(s)
		}
		terms = append(terms, term)
	}
	return terms
}

// quote makes word an FTS5 string. Words come from searchWords, so they hold
// no quote to escape.
func quote(word string) string {
	return `"` + word + `"`
}
