package sqlc

import (
	"context"
	"strings"
)

// ListRecipes and CountRecipes are hand-written like the statements in
// fts.go, for a different reason: which conditions the overview's filter
// needs, and which ORDER BY, depends on the request. sqlc can only generate
// a fixed statement, and one that holds every filter as a switchable
// condition had to be copied four times over (list and count, with and
// without full-text search) and tricked into typing its parameters. Here
// each filter is appended only when it is set, and the statement is built
// from fixed fragments alone: every value travels as a bound parameter.

// RecipeFilter narrows ListRecipes and CountRecipes. Every field at its zero
// value switches its filter off; the ones that are set combine with AND.
type RecipeFilter struct {
	// Match is an FTS5 MATCH expression over recipes_fts.
	Match string
	// Tags keeps the recipes that carry every one of these names.
	Tags []string
	// MaxMinutes keeps the recipes whose prep and cook time sum to between
	// 1 and MaxMinutes, so a recipe with neither time set never matches.
	MaxMinutes int64
	// FavoritesOf keeps the recipes this user id has starred.
	FavoritesOf string
	// Author keeps the recipes created by the user with this username.
	Author string
}

// RecipeSort names an order for ListRecipes.
type RecipeSort string

// The orders ListRecipes knows. Anything else falls back to
// RecipeSortUpdated.
const (
	RecipeSortUpdated RecipeSort = "updated"
	RecipeSortCreated RecipeSort = "created"
	RecipeSortTitle   RecipeSort = "title"
)

// The title order uses the "unicode" collation package db registers, so
// umlauts sort beside their base letter. Every order ends on id DESC: timestamps have second resolution, so a burst
// of writes shares one, and ids are UUIDv7, so the tiebreak still reads
// newest first.
var recipeOrderBy = map[RecipeSort]string{
	RecipeSortUpdated: "r.updated_at DESC, r.id DESC",
	RecipeSortCreated: "r.created_at DESC, r.id DESC",
	RecipeSortTitle:   "r.title COLLATE unicode, r.id DESC",
}

const recipeColumns = `r.id, r.slug, r.title, r.description, r.servings, r.prep_minutes, r.cook_minutes,
       r.source_url, r.cover_image_id, r.created_by, r.created_at, r.updated_by, r.updated_at, r.edit_policy`

// where renders f as a WHERE clause, empty when no filter is set, and the
// arguments its placeholders bind in order.
func (f RecipeFilter) where() (string, []any) {
	var conds []string
	var args []any
	if f.Match != "" {
		conds = append(conds, "r.rowid IN (SELECT rowid FROM recipes_fts WHERE recipes_fts MATCH ?)")
		args = append(args, f.Match)
	}
	if len(f.Tags) > 0 {
		// The HAVING turns "any of" into "all of", and sitting inside the
		// subquery it keeps the outer query at one row per recipe.
		conds = append(conds, `r.id IN (
    SELECT rt.recipe_id
    FROM recipe_tags rt
    JOIN tags t ON t.id = rt.tag_id
    WHERE t.name IN (?`+strings.Repeat(", ?", len(f.Tags)-1)+`)
    GROUP BY rt.recipe_id
    HAVING COUNT(DISTINCT t.name) = ?
  )`)
		for _, tag := range f.Tags {
			args = append(args, tag)
		}
		args = append(args, len(f.Tags))
	}
	if f.MaxMinutes > 0 {
		conds = append(conds, "COALESCE(r.prep_minutes, 0) + COALESCE(r.cook_minutes, 0) BETWEEN 1 AND ?")
		args = append(args, f.MaxMinutes)
	}
	if f.FavoritesOf != "" {
		conds = append(conds, "r.id IN (SELECT recipe_id FROM favorites WHERE user_id = ?)")
		args = append(args, f.FavoritesOf)
	}
	if f.Author != "" {
		conds = append(conds, "r.created_by = (SELECT id FROM users WHERE username = ?)")
		args = append(args, f.Author)
	}
	if len(conds) == 0 {
		return "", nil
	}
	return "\nWHERE " + strings.Join(conds, "\n  AND "), args
}

// ListRecipes returns one page of the recipes f matches, in the given order.
func (q *Queries) ListRecipes(ctx context.Context, f RecipeFilter, sort RecipeSort, limit, offset int64) ([]Recipe, error) {
	orderBy, ok := recipeOrderBy[sort]
	if !ok {
		orderBy = recipeOrderBy[RecipeSortUpdated]
	}
	where, args := f.where()
	query := "SELECT " + recipeColumns + "\nFROM recipes r" + where +
		"\nORDER BY " + orderBy + "\nLIMIT ? OFFSET ?"
	rows, err := q.db.QueryContext(ctx, query, append(args, limit, offset)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Recipe{}
	for rows.Next() {
		var i Recipe
		if err := rows.Scan(
			&i.ID,
			&i.Slug,
			&i.Title,
			&i.Description,
			&i.Servings,
			&i.PrepMinutes,
			&i.CookMinutes,
			&i.SourceUrl,
			&i.CoverImageID,
			&i.CreatedBy,
			&i.CreatedAt,
			&i.UpdatedBy,
			&i.UpdatedAt,
			&i.EditPolicy,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

// CountRecipes returns how many recipes f matches in total.
func (q *Queries) CountRecipes(ctx context.Context, f RecipeFilter) (int64, error) {
	where, args := f.where()
	var n int64
	err := q.db.QueryRowContext(ctx, "SELECT COUNT(*)\nFROM recipes r"+where, args...).Scan(&n)
	return n, err
}
