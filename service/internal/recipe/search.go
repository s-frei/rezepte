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
	// SQL matches on COUNT(DISTINCT t.name) = len(Tags), so a duplicate tag
	// would silently match nothing. The handler also calls
	// NormalizeTagQuery, for the API-level cap on the raw query parameter,
	// but List must not depend on that.
	f := sqlc.RecipeFilter{
		Match:  ftsQuery(p.Query),
		Tags:   NormalizeTagQuery(p.Tags),
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

	rows, err := s.q.ListRecipes(ctx, f, normalizeSort(p.Sort), int64(limit), offset)
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
	return Page{Items: items, Page: page, Limit: limit, Total: int(total)}, nil
}

// normalizeSort maps sort to one of the orders sqlc.ListRecipes knows,
// falling back to the default, most recently updated first, for anything
// else. ListRecipes falls back the same way on its own; normalizing here is
// about a predictable API, since sort only ever selects a fixed ORDER BY.
func normalizeSort(sort string) sqlc.RecipeSort {
	switch s := sqlc.RecipeSort(sort); s {
	case sqlc.RecipeSortCreated, sqlc.RecipeSortTitle:
		return s
	default:
		return sqlc.RecipeSortUpdated
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
