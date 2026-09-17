package recipe

import (
	"context"
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
// search across title, description, ingredient names and tags; Tag
// restricts results to recipes carrying that exact tag name. When both are
// set they combine with AND. Limit is clamped to [1, 100] (0 defaults to
// 24); Page is clamped to at least 1.
type ListParams struct {
	Query string
	Tag   string
	Page  int
	Limit int
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
// optionally narrowed by full-text search query and/or tag.
func (s *Service) List(ctx context.Context, p ListParams) (Page, error) {
	limit := clampLimit(p.Limit)
	page := p.Page
	if page < 1 {
		page = 1
	}
	offset := int64((page - 1) * limit)

	rows, total, err := s.listRows(ctx, p.Tag, ftsQuery(p.Query), int64(limit), offset)
	if err != nil {
		return Page{}, err
	}
	items, err := s.toCards(ctx, rows)
	if err != nil {
		return Page{}, err
	}
	return Page{Items: items, Page: page, Limit: limit, Total: int(total)}, nil
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

// listRows runs the list/search query matching which of tag and query are
// set, returning the matching rows for one page plus the total number of
// matches. query is already an FTS5 match expression (see ftsQuery) or "".
func (s *Service) listRows(ctx context.Context, tag, query string, limit, offset int64) ([]sqlc.Recipe, int64, error) {
	switch {
	case tag != "" && query != "":
		rows, err := s.q.SearchRecipesByTag(ctx, sqlc.SearchRecipesByTagParams{Tag: tag, Query: query, Limit: limit, Offset: offset})
		if err != nil {
			return nil, 0, fmt.Errorf("search recipes by tag: %w", err)
		}
		total, err := s.q.CountSearchRecipesByTag(ctx, sqlc.CountSearchRecipesByTagParams{Tag: tag, Query: query})
		if err != nil {
			return nil, 0, fmt.Errorf("count search recipes by tag: %w", err)
		}
		return rows, total, nil
	case tag != "":
		rows, err := s.q.ListRecipesByTag(ctx, sqlc.ListRecipesByTagParams{Name: tag, Limit: limit, Offset: offset})
		if err != nil {
			return nil, 0, fmt.Errorf("list recipes by tag: %w", err)
		}
		total, err := s.q.CountRecipesByTag(ctx, tag)
		if err != nil {
			return nil, 0, fmt.Errorf("count recipes by tag: %w", err)
		}
		return rows, total, nil
	case query != "":
		rows, err := s.q.SearchRecipes(ctx, sqlc.SearchRecipesParams{Query: query, Limit: limit, Offset: offset})
		if err != nil {
			return nil, 0, fmt.Errorf("search recipes: %w", err)
		}
		total, err := s.q.CountSearchRecipes(ctx, query)
		if err != nil {
			return nil, 0, fmt.Errorf("count search recipes: %w", err)
		}
		return rows, total, nil
	default:
		rows, err := s.q.ListRecipes(ctx, sqlc.ListRecipesParams{Limit: limit, Offset: offset})
		if err != nil {
			return nil, 0, fmt.Errorf("list recipes: %w", err)
		}
		total, err := s.q.CountRecipes(ctx)
		if err != nil {
			return nil, 0, fmt.Errorf("count recipes: %w", err)
		}
		return rows, total, nil
	}
}

// toCards loads the tags for rows in one batch and assembles them into
// Cards, in the same order as rows. It always returns a non-nil slice.
func (s *Service) toCards(ctx context.Context, rows []sqlc.Recipe) ([]Card, error) {
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
	for _, r := range rows {
		card, err := toCard(r, tagsByRecipe[r.ID])
		if err != nil {
			return nil, err
		}
		items = append(items, card)
	}
	return items, nil
}

// toCard builds a Card from a stored recipe row and its tag names.
func toCard(row sqlc.Recipe, tags []string) (Card, error) {
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
