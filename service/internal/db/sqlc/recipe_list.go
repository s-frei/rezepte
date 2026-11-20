package sqlc

import (
	"context"
	"fmt"
	"strings"
)

// ListRecipes, CountRecipes and ListTagCounts are hand-written like the statements in
// fts.go, for a different reason: which conditions the overview's filter
// needs, and which ORDER BY, depends on the request. sqlc can only generate
// a fixed statement, and one that holds every filter as a switchable
// condition had to be copied four times over (list and count, with and
// without full-text search) and tricked into typing its parameters. Here
// each filter is appended only when it is set, and the statement is built
// from fixed fragments alone: every value travels as a bound parameter.

// RecipeFilter narrows ListRecipes, CountRecipes and ListTagCounts. Every field at its zero
// value switches its filter off; the ones that are set combine with AND.
type RecipeFilter struct {
	// Search keeps the recipes that contain every one of these terms.
	Search []SearchTerm
	// Phrase is the folded search words joined by single spaces. It only
	// ranks: a title that folds to exactly the phrase comes first (see rank).
	Phrase string
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

// SearchTerm is one folded word of a full-text search as FTS5 MATCH
// expressions. Exact and Prefix run against recipes_fts (whole words: the
// word itself, a word starting with it); Trigram runs against
// recipes_trigram (any part of a word) and is empty for a word shorter than
// the three characters a trigram needs.
type SearchTerm struct {
	Exact   string
	Prefix  string
	Trigram string
}

// RecipeSort names an order for ListRecipes.
type RecipeSort string

// The orders ListRecipes knows. Anything else falls back to
// RecipeSortUpdated.
const (
	RecipeSortUpdated RecipeSort = "updated"
	RecipeSortCreated RecipeSort = "created"
	RecipeSortTitle   RecipeSort = "title"
	RecipeSortTasty   RecipeSort = "tasty"
)

// The title order uses the "unicode" collation package db registers, so
// umlauts sort beside their base letter. Every order ends on id DESC: timestamps have second resolution, so a burst
// of writes shares one, and ids are UUIDv7, so the tiebreak still reads
// newest first. The tasty order counts each row's marks in a correlated
// subquery on tasty_recipe_idx rather than keeping a counter on recipes: a
// household's collection is small enough that the count is cheap, and a
// counter would be one more thing to keep in step with the tasty table.
var recipeOrderBy = map[RecipeSort]string{
	RecipeSortUpdated: "r.updated_at DESC, r.id DESC",
	RecipeSortCreated: "r.created_at DESC, r.id DESC",
	RecipeSortTitle:   "r.title COLLATE unicode, r.id DESC",
	RecipeSortTasty:   "(SELECT COUNT(*) FROM tasty t WHERE t.recipe_id = r.id) DESC, r.updated_at DESC, r.id DESC",
}

const recipeColumns = `r.id, r.slug, r.title, r.description, r.servings, r.prep_minutes, r.cook_minutes,
       r.source_url, r.source_name, r.cover_image_id, r.created_by, r.created_at, r.updated_by, r.updated_at, r.edit_policy`

// where renders f as a WHERE clause, empty when no filter is set, and the
// arguments its placeholders bind in order.
func (f RecipeFilter) where() (string, []any) {
	var conds []string
	var args []any
	// Every word is found anywhere in a word, or at the start of a word
	// when it is too short for a trigram.
	for _, term := range f.Search {
		if term.Trigram != "" {
			conds = append(conds, "r.rowid IN (SELECT rowid FROM recipes_trigram WHERE recipes_trigram MATCH ?)")
			args = append(args, term.Trigram)
		} else {
			conds = append(conds, "r.rowid IN (SELECT rowid FROM recipes_fts WHERE recipes_fts MATCH ?)")
			args = append(args, term.Prefix)
		}
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

// rankColumns weighs where a search word is found. A title hit outweighs
// every hit of the same word below it put together (3 x (10 + 10 + 1) < 100).
var rankColumns = []struct {
	name   string
	weight int
}{{"title", 100}, {"tags", 10}, {"ingredients", 10}, {"description", 1}}

// rank renders the relevance of a recipe to f.Search as an SQL expression
// and the arguments it binds. A title that is exactly the search leads.
// Below that, every word scores in every column once for each way it is
// found there - as a whole word, as a word start, as part of a word - and
// the scores add up. The three ways nest, so a column scores 3, 2 or 1
// times its weight per word: "Brot" above "Brotsalat" above "Bauernbrot".
//
// Fixed tiers rather than FTS5's bm25(): bm25 weighs a word by how rare it
// is in the collection, so the order would shift as recipes are added, and
// it cannot tell a word from a part of one ("Brotsalat" can outrank "Brot").
// Each part is an uncorrelated IN, so SQLite runs each MATCH once.
func (f RecipeFilter) rank() (string, []any) {
	parts := []string{"1000 * (fold_words(r.title) = ?)"}
	args := []any{f.Phrase}
	add := func(table, column string, weight int, expr string) {
		if expr == "" {
			return
		}
		parts = append(parts, fmt.Sprintf("%d * (r.rowid IN (SELECT rowid FROM %s WHERE %s MATCH ?))", weight, table, table))
		args = append(args, "{"+column+"} : ("+expr+")")
	}
	for _, term := range f.Search {
		for _, c := range rankColumns {
			add("recipes_fts", c.name, c.weight, term.Exact)
			add("recipes_fts", c.name, c.weight, term.Prefix)
			add("recipes_trigram", c.name, c.weight, term.Trigram)
		}
	}
	return strings.Join(parts, " + "), args
}

// tierFields are the fields a hit is grouped by, strongest first; a recipe
// found in none of them was found in its description.
var tierFields = []string{"title", "tags", "ingredients"}

// tier renders where a recipe's strongest find is as an SQL expression - 4
// for the title, 3 tags, 2 ingredients, 1 description - and the arguments
// it binds. Any word counts, matched as the WHERE matches it: in a word
// through recipes_trigram, at a word start through recipes_fts for a word
// too short for a trigram. The overview groups a search by this, and
// recipe.Match computes the same section for each card.
func (f RecipeFilter) tier() (string, []any) {
	var long, short []string
	for _, term := range f.Search {
		if term.Trigram != "" {
			long = append(long, term.Trigram)
		} else {
			short = append(short, term.Prefix)
		}
	}
	var b strings.Builder
	var args []any
	b.WriteString("CASE")
	for i, field := range tierFields {
		var conds []string
		if len(long) > 0 {
			conds = append(conds, "r.rowid IN (SELECT rowid FROM recipes_trigram WHERE recipes_trigram MATCH ?)")
			args = append(args, "{"+field+"} : ("+strings.Join(long, " OR ")+")")
		}
		if len(short) > 0 {
			conds = append(conds, "r.rowid IN (SELECT rowid FROM recipes_fts WHERE recipes_fts MATCH ?)")
			args = append(args, "{"+field+"} : ("+strings.Join(short, " OR ")+")")
		}
		fmt.Fprintf(&b, " WHEN %s THEN %d", strings.Join(conds, " OR "), len(tierFields)+1-i)
	}
	b.WriteString(" ELSE 1 END")
	return b.String(), args
}

// ListRecipes returns one page of the recipes f matches, in the given order.
func (q *Queries) ListRecipes(ctx context.Context, f RecipeFilter, sort RecipeSort, limit, offset int64) ([]Recipe, error) {
	where, args := f.where()
	orderBy, ok := recipeOrderBy[sort]
	if !ok {
		orderBy = recipeOrderBy[RecipeSortUpdated]
	}
	// A search in the default order puts its best hits first; an order
	// the reader picked stays as picked.
	if sort == RecipeSortUpdated && len(f.Search) > 0 {
		tier, tierArgs := f.tier()
		score, scoreArgs := f.rank()
		// Grouped by where the strongest find is, then ranked. Among equal
		// hits the shorter title is the closer one: "Brot" before "Einfaches
		// Brot". ponytail: length is a proxy for how much of the title the
		// search covers; compare matched characters if it misleads.
		orderBy = tier + " DESC, " + score + " DESC, length(r.title), " + recipeOrderBy[RecipeSortUpdated]
		args = append(append(args, tierArgs...), scoreArgs...)
	}
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
			&i.SourceName,
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

// CountSections returns how many of the recipes f matches have their
// strongest find in each field, keyed by RecipeFilter.tier's number: 4
// title, 3 tags, 2 ingredients, 1 description. Without Search it returns
// nil: there is nothing to group.
func (q *Queries) CountSections(ctx context.Context, f RecipeFilter) (map[int]int64, error) {
	if len(f.Search) == 0 {
		return nil, nil
	}
	where, args := f.where()
	tier, tierArgs := f.tier()
	rows, err := q.db.QueryContext(ctx, "SELECT "+tier+" AS tier, COUNT(*)\nFROM recipes r"+where+"\nGROUP BY tier",
		append(tierArgs, args...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := map[int]int64{}
	for rows.Next() {
		var t int
		var n int64
		if err := rows.Scan(&t, &n); err != nil {
			return nil, err
		}
		counts[t] = n
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	return counts, rows.Err()
}

// TagCountRow is a tag name and how many of the recipes a filter matches
// carry it.
type TagCountRow struct {
	Name  string
	Count int64
}

// ListTagCounts returns every tag the recipes f matches carry, with how
// many of them carry it, most used first. With no filter set that is every
// tag in use. It is here rather than generated for the same reason as
// ListRecipes: the overview counts its tag suggestions under the filter it
// shows, so every tag offered narrows the grid without emptying it.
func (q *Queries) ListTagCounts(ctx context.Context, f RecipeFilter) ([]TagCountRow, error) {
	where, args := f.where()
	query := "SELECT t.name, COUNT(*) AS count\nFROM recipes r\nJOIN recipe_tags rt ON rt.recipe_id = r.id\nJOIN tags t ON t.id = rt.tag_id" +
		where + "\nGROUP BY t.id\nORDER BY count DESC, t.name COLLATE unicode"
	rows, err := q.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []TagCountRow{}
	for rows.Next() {
		var i TagCountRow
		if err := rows.Scan(&i.Name, &i.Count); err != nil {
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
