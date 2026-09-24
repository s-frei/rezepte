package recipe

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

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
// first), "title" (A-Z), or anything else including "" (most recently
// updated first, the default). Limit is clamped to [1, 100] (0 defaults to
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
	Sort   string
	Page   int
	Limit  int
	UserID string
}

// Page is one page of recipe cards plus the pagination state used to
// produce it.
type Page struct {
	Items []Card `json:"items"`
	Page  int    `json:"page"`
	Limit int    `json:"limit"`
	Total int    `json:"total"`
}

// List returns a page of recipe cards, most recently updated first,
// optionally narrowed by full-text search query and/or tags.
func (s *Service) List(ctx context.Context, p ListParams) (Page, error) {
	limit := clampLimit(p.Limit)
	page := p.Page
	if page < 1 {
		page = 1
	}
	offset := int64((page - 1) * limit)

	// Normalized and de-duplicated here, not trusted from the caller: the
	// SQL matches on COUNT(DISTINCT t.name) = tag_count, so a duplicate tag
	// would inflate tag_count past the distinct count and silently match
	// nothing. The handler also calls NormalizeTagQuery, for the API-level
	// cap on the raw query parameter, but List must not depend on that.
	tags := NormalizeTagQuery(p.Tags)

	// Negative input is nonsense (BETWEEN 1 AND a negative number matches
	// nothing, which reads as "everything excluded" rather than "off") and
	// is ignored the same way clampLimit ignores an invalid Limit. There is
	// no upper clamp: an oversized value just fails to narrow the query
	// further, which is harmless.
	maxMinutes := p.MaxMinutes
	if maxMinutes < 0 {
		maxMinutes = 0
	}

	// An empty UserID disables the filter outright, the same way it disables
	// the per-card favorite lookup in toCards - it must not fall through to
	// the SQL condition's own comparison of user_id against an empty string,
	// which happens to match no row today but is a fail-closed accident, not
	// a guarantee. This is what makes the doc comment on ListParams.UserID
	// true for every path, not just the two that were built to already
	// respect it.
	favoritesOnly := p.FavoritesOnly && p.UserID != ""

	rows, total, err := s.listRows(ctx, tags, ftsQuery(p.Query), int64(maxMinutes), favoritesOnly, p.UserID, p.Author, normalizeSort(p.Sort), int64(limit), offset)
	if err != nil {
		return Page{}, err
	}
	items, err := s.toCards(ctx, rows, p.UserID)
	if err != nil {
		return Page{}, err
	}
	return Page{Items: items, Page: page, Limit: limit, Total: int(total)}, nil
}

// The three sort orders ListParams.Sort and listInput.Sort accept. These
// match the SQL string literals compared inside the ORDER BY CASEs of
// ListRecipesFiltered and SearchRecipesFiltered exactly - sortDefault
// itself never appears in the SQL, since any value other than "created"
// and "title" already falls into the default branch there.
const (
	sortUpdated = "updated"
	sortCreated = "created"
	sortTitle   = "title"
)

// normalizeSort maps sort to one of the three known values, falling back
// to sortUpdated - the default - for anything else. That includes an
// empty string and an injection-shaped value alike: sort never reaches SQL
// as an identifier, only as a value compared inside a CASE (see the note
// on ListRecipesFiltered), so normalizing here is about a predictable API,
// not about safety the SQL doesn't already have on its own.
func normalizeSort(sort string) string {
	switch sort {
	case sortCreated, sortTitle:
		return sort
	default:
		return sortUpdated
	}
}

// clampLimit applies List's limit rules: 0 defaults to 24, anything else is
// clamped to [1, 100].
func clampLimit(limit int) int {
	switch {
	case limit == 0:
		return defaultListLimit
	case limit < 1:
		return 1
	case limit > maxListLimit:
		return maxListLimit
	default:
		return limit
	}
}

// listRows runs the list/search query matching whether a full-text query is
// set, returning the matching rows for one page plus the total number of
// matches. query is already an FTS5 match expression (see ftsQuery) or "".
// tags is assumed already normalized and de-duplicated, maxMinutes already
// clamped to a non-negative value, and sort already normalized to one of
// the three known values (see List and normalizeSort). userID is the
// caller's id - see ListParams.UserID for what an empty one means for
// favoritesOnly.
//
// Only the full-text half needs two variants: the match cannot be switched
// off from inside the query (see the note on SearchRecipesFiltered). Every
// other filter is a condition the query itself disables, so a new one is a
// line in the SQL rather than another branch here. sort is the exception:
// it never touches the count queries, which have no ORDER BY.
func (s *Service) listRows(ctx context.Context, tags []string, query string, maxMinutes int64, favoritesOnly bool, userID, author, sort string, limit, offset int64) ([]sqlc.Recipe, int64, error) {
	// The tag names travel as a JSON array rather than a sqlc.slice: see
	// the note on ListRecipesFiltered. A zero count switches the condition
	// off, but the parameter still has to hold valid JSON, so a nil slice
	// has to marshal as "[]" and not as "null".
	tagCount := int64(len(tags))
	if tags == nil {
		tags = []string{}
	}
	names, err := json.Marshal(tags)
	if err != nil {
		return nil, 0, fmt.Errorf("marshal tag names: %w", err)
	}

	var favOnly int64
	if favoritesOnly {
		favOnly = 1
	}

	if query != "" {
		rows, err := s.q.SearchRecipesFiltered(ctx, sqlc.SearchRecipesFilteredParams{
			Query: query, TagNames: string(names), TagCount: tagCount, MaxMinutes: maxMinutes,
			FavoritesOnly: favOnly, UserID: userID, Author: author, Sort: sort, Limit: limit, Offset: offset,
		})
		if err != nil {
			return nil, 0, fmt.Errorf("search recipes filtered: %w", err)
		}
		total, err := s.q.CountSearchRecipesFiltered(ctx, sqlc.CountSearchRecipesFilteredParams{
			Query: query, TagNames: string(names), TagCount: tagCount, MaxMinutes: maxMinutes,
			FavoritesOnly: favOnly, UserID: userID, Author: author,
		})
		if err != nil {
			return nil, 0, fmt.Errorf("count search recipes filtered: %w", err)
		}
		return toRecipesFromSearch(rows), total, nil
	}

	rows, err := s.q.ListRecipesFiltered(ctx, sqlc.ListRecipesFilteredParams{
		TagNames: string(names), TagCount: tagCount, MaxMinutes: maxMinutes,
		FavoritesOnly: favOnly, UserID: userID, Author: author, Sort: sort, Limit: limit, Offset: offset,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list recipes filtered: %w", err)
	}
	total, err := s.q.CountRecipesFiltered(ctx, sqlc.CountRecipesFilteredParams{
		TagNames: string(names), TagCount: tagCount, MaxMinutes: maxMinutes,
		FavoritesOnly: favOnly, UserID: userID, Author: author,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("count recipes filtered: %w", err)
	}
	return toRecipesFromList(rows), total, nil
}

// toRecipesFromList and toRecipesFromSearch convert the row types
// ListRecipesFiltered and SearchRecipesFiltered actually return into
// sqlc.Recipe. Both queries select from a CTE (see the note on
// ListRecipesFiltered for why sort forces that) rather than straight from
// the recipes table, so sqlc synthesizes a query-specific row type instead
// of reusing sqlc.Recipe - even though its fields are identical, name,
// type and order, to sqlc.Recipe's. That identity is exactly what makes
// the per-element conversion below valid Go: converting the whole slice in
// one step isn't, since []ListRecipesFilteredRow and []sqlc.Recipe are
// themselves different, unconvertible types despite their identical
// element layout.
func toRecipesFromList(rows []sqlc.ListRecipesFilteredRow) []sqlc.Recipe {
	out := make([]sqlc.Recipe, len(rows))
	for i, r := range rows {
		out[i] = sqlc.Recipe(r)
	}
	return out
}

func toRecipesFromSearch(rows []sqlc.SearchRecipesFilteredRow) []sqlc.Recipe {
	out := make([]sqlc.Recipe, len(rows))
	for i, r := range rows {
		out[i] = sqlc.Recipe(r)
	}
	return out
}

// toCards loads the tags, the authors' display names and colors and, when
// userID is set, the favorite state for rows in one batch each and
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
	seen := make(map[string]bool, 2*len(rows))
	for _, r := range rows {
		for _, id := range [2]string{r.CreatedBy, r.UpdatedBy} {
			if !seen[id] {
				seen[id] = true
				userIDs = append(userIDs, id)
			}
		}
	}
	authorRows, err := s.q.ListAuthorsForIDs(ctx, userIDs)
	if err != nil {
		return nil, fmt.Errorf("list authors for recipes: %w", err)
	}
	authors := make(map[string]Person, len(authorRows))
	for _, a := range authorRows {
		authors[a.ID] = Person{ID: a.ID, Username: a.Username, DisplayName: a.DisplayName, Color: a.Color}
	}

	favorites := make(map[string]bool)
	if userID != "" {
		idsJSON, err := json.Marshal(ids)
		if err != nil {
			return nil, fmt.Errorf("marshal recipe ids: %w", err)
		}
		favIDs, err := s.q.ListFavoriteRecipeIDs(ctx, sqlc.ListFavoriteRecipeIDsParams{
			UserID: userID, RecipeIds: string(idsJSON),
		})
		if err != nil {
			return nil, fmt.Errorf("list favorite recipe ids: %w", err)
		}
		for _, id := range favIDs {
			favorites[id] = true
		}
	}

	for _, r := range rows {
		card, err := toCard(r, tagsByRecipe[r.ID], favorites[r.ID],
			authors[r.CreatedBy], authors[r.UpdatedBy])
		if err != nil {
			return nil, err
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

// asciiToUmlaut turns the plain-ASCII German spellings of umlauts and ß
// (ae, oe, ue, ss) back into their diacritic form (ä, ö, ü, ß) — the
// inverse of the umlauts replacer in slug.go.
var asciiToUmlaut = strings.NewReplacer("ae", "ä", "oe", "ö", "ue", "ü", "ss", "ß")

// variants returns term plus its alternate German spelling, when it has
// one: an ASCII term such as "kaese" also gets its umlaut form ("käse"),
// and a term already spelled with umlauts or ß also gets its ASCII
// transliteration ("käse" → "kaese", matching Slugify's convention). This
// is needed because the FTS5 tokenizer's remove_diacritics option folds
// "ä" to "a", not to "ae": an ASCII query would otherwise never match text
// indexed with umlauts, and vice versa.
//
// Precondition: term must already be lower-cased. Both replacers only
// match lower-case patterns ("ae", "oe", "ue", "ss", "ä", "ö", "ü"), so a
// mixed- or upper-case term (e.g. "KAESE") would silently fail to produce
// its umlaut variant. ftsQuery, the only caller, lower-cases each term
// before calling variants; FTS5's own tokenizer case-folds regardless, so
// lower-casing here loses no matches.
func variants(term string) []string {
	out := []string{term}
	if v := asciiToUmlaut.Replace(term); v != term {
		out = append(out, v)
	}
	if v := umlauts.Replace(term); v != term {
		out = append(out, v)
	}
	return out
}

// ftsQuery translates a user search string into an FTS5 MATCH expression
// against recipes_fts: each whitespace-separated term becomes a
// double-quoted prefix match ("term"*, with embedded quotes doubled), its
// alternate German spellings (see variants) are combined with OR, and
// terms are combined with an explicit AND. (FTS5's implicit AND only
// applies between bare phrases; adjacent parenthesized groups, which an OR
// of variants requires, are a syntax error without an explicit operator.)
// An empty (or all-whitespace) input returns "": callers must treat that as
// "no search query" and not run a search query with it.
func ftsQuery(q string) string {
	terms := strings.Fields(q)
	if len(terms) == 0 {
		return ""
	}
	groups := make([]string, len(terms))
	for i, term := range terms {
		term = strings.ToLower(term)
		vs := variants(term)
		quoted := make([]string, len(vs))
		for j, v := range vs {
			quoted[j] = `"` + strings.ReplaceAll(v, `"`, `""`) + `"*`
		}
		if len(quoted) == 1 {
			groups[i] = quoted[0]
		} else {
			groups[i] = "(" + strings.Join(quoted, " OR ") + ")"
		}
	}
	return strings.Join(groups, " AND ")
}
