package recipe

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/s-frei/rezepte/service/internal/db"
	"github.com/s-frei/rezepte/service/internal/db/sqlc"
)

// Service creates, updates, deletes and loads recipes, keeping slugs, tags
// and the full-text search index consistent with the stored documents.
type Service struct {
	conn     *sql.DB
	q        *sqlc.Queries
	now      func() time.Time
	imageDir string // "" disables image directory cleanup on Delete
}

// Option configures a Service.
type Option func(*Service)

// WithImageDir tells Delete where recipe image directories live
// (<dir>/<recipeId>) so it can remove them after the recipe row is gone.
func WithImageDir(dir string) Option {
	return func(s *Service) { s.imageDir = dir }
}

// NewService returns a Service backed by conn.
func NewService(conn *sql.DB, opts ...Option) *Service {
	s := &Service{conn: conn, q: sqlc.New(conn), now: time.Now}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Create stores a new recipe as createdBy, assigning it a slug derived from
// the title (numbered on collision), and indexes it for search.
func (s *Service) Create(ctx context.Context, createdBy string, in Input) (Recipe, error) {
	tags := NormalizeTags(in.Tags)
	var id string
	err := db.Tx(ctx, s.conn, func(q *sqlc.Queries) error {
		slug, err := uniqueSlug(ctx, q, Slugify(in.Title))
		if err != nil {
			return err
		}
		id = uuid.Must(uuid.NewV7()).String()
		now := db.FormatTime(s.now())
		if _, err := q.InsertRecipe(ctx, sqlc.InsertRecipeParams{
			ID:          id,
			Slug:        slug,
			Title:       in.Title,
			Description: in.Description,
			Servings:    int64(in.Servings),
			PrepMinutes: intToInt64Ptr(in.PrepMinutes),
			CookMinutes: intToInt64Ptr(in.CookMinutes),
			SourceUrl:   in.SourceURL,
			CreatedBy:   createdBy,
			CreatedAt:   now,
			UpdatedAt:   now,
		}); err != nil {
			return fmt.Errorf("insert recipe: %w", err)
		}
		if err := writeChildren(ctx, q, id, in); err != nil {
			return err
		}
		if err := writeTags(ctx, q, id, tags); err != nil {
			return err
		}
		return reindex(ctx, q, id, in, tags)
	})
	if err != nil {
		return Recipe{}, err
	}
	return s.ByID(ctx, id)
}

// Update replaces every editable field of the recipe id - title,
// description, servings, prep and cook minutes, source URL - along with all
// its child rows (ingredient groups, ingredients, steps, tags), keeping its
// slug and creation metadata. It returns ErrNotFound when no such recipe
// exists.
func (s *Service) Update(ctx context.Context, id string, in Input) (Recipe, error) {
	tags := NormalizeTags(in.Tags)
	err := db.Tx(ctx, s.conn, func(q *sqlc.Queries) error {
		if _, err := q.GetRecipe(ctx, id); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return fmt.Errorf("get recipe %s: %w", id, err)
		}
		now := db.FormatTime(s.now())
		if _, err := q.UpdateRecipe(ctx, sqlc.UpdateRecipeParams{
			ID:          id,
			Title:       in.Title,
			Description: in.Description,
			Servings:    int64(in.Servings),
			PrepMinutes: intToInt64Ptr(in.PrepMinutes),
			CookMinutes: intToInt64Ptr(in.CookMinutes),
			SourceUrl:   in.SourceURL,
			UpdatedAt:   now,
		}); err != nil {
			return fmt.Errorf("update recipe: %w", err)
		}
		if err := q.DeleteIngredientGroupsByRecipe(ctx, id); err != nil {
			return fmt.Errorf("delete ingredient groups: %w", err)
		}
		if err := q.DeleteStepsByRecipe(ctx, id); err != nil {
			return fmt.Errorf("delete steps: %w", err)
		}
		if err := q.DeleteRecipeTags(ctx, id); err != nil {
			return fmt.Errorf("delete recipe tags: %w", err)
		}
		if err := writeChildren(ctx, q, id, in); err != nil {
			return err
		}
		if err := writeTags(ctx, q, id, tags); err != nil {
			return err
		}
		if err := q.DeleteOrphanTags(ctx); err != nil {
			return fmt.Errorf("delete orphan tags: %w", err)
		}
		return reindex(ctx, q, id, in, tags)
	})
	if err != nil {
		return Recipe{}, err
	}
	return s.ByID(ctx, id)
}

// Delete removes the recipe id and all of its children (via ON DELETE
// CASCADE), drops it from the search index and prunes tags left orphaned by
// the deletion. It returns ErrNotFound when no such recipe exists.
func (s *Service) Delete(ctx context.Context, id string) error {
	err := db.Tx(ctx, s.conn, func(q *sqlc.Queries) error {
		rowid, err := q.GetRecipeRowID(ctx, id)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("get recipe rowid %s: %w", id, err)
		}
		n, err := q.DeleteRecipe(ctx, id)
		if err != nil {
			return fmt.Errorf("delete recipe %s: %w", id, err)
		}
		if n == 0 {
			return ErrNotFound
		}
		if err := q.DeleteFTS(ctx, rowid); err != nil {
			return fmt.Errorf("delete fts: %w", err)
		}
		return q.DeleteOrphanTags(ctx)
	})
	if err != nil {
		return err
	}
	if s.imageDir != "" {
		// Best effort: the rows are gone (and with them every reference to
		// the files); a leftover directory is an orphan, not an inconsistency.
		if err := os.RemoveAll(filepath.Join(s.imageDir, id)); err != nil {
			slog.Warn("remove recipe image dir", "recipe", id, "err", err)
		}
	}
	return nil
}

// ByID loads a recipe by id. It returns ErrNotFound when no such recipe
// exists.
func (s *Service) ByID(ctx context.Context, id string) (Recipe, error) {
	row, err := s.q.GetRecipe(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Recipe{}, ErrNotFound
	}
	if err != nil {
		return Recipe{}, fmt.Errorf("get recipe %s: %w", id, err)
	}
	return s.load(ctx, row)
}

// BySlug loads a recipe by slug. It returns ErrNotFound when no such recipe
// exists.
func (s *Service) BySlug(ctx context.Context, slug string) (Recipe, error) {
	row, err := s.q.GetRecipeBySlug(ctx, slug)
	if errors.Is(err, sql.ErrNoRows) {
		return Recipe{}, ErrNotFound
	}
	if err != nil {
		return Recipe{}, fmt.Errorf("get recipe by slug %s: %w", slug, err)
	}
	return s.load(ctx, row)
}

// Tags returns every tag currently used by at least one recipe, together
// with how many recipes use it, most used first.
func (s *Service) Tags(ctx context.Context) ([]TagCount, error) {
	rows, err := s.q.ListTagsWithCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	out := make([]TagCount, len(rows))
	for i, r := range rows {
		out[i] = TagCount{Name: r.Name, Count: int(r.Count)}
	}
	return out, nil
}

// Count returns how many recipes exist.
func (s *Service) Count(ctx context.Context) (int, error) {
	// TagCount: 0, MaxMinutes: 0 and FavouritesOnly: 0 switch those filters
	// off entirely (see the note on ListRecipesFiltered), so TagNames never
	// has to hold real tag names and UserID never has to hold a real user
	// id. All three are now a concrete int64 (see the CAST note on
	// ListRecipesFiltered) whose Go zero value is already 0, but they are
	// still spelled out explicitly rather than leaning on that: this call
	// site broke three times on this branch from omitting one of them
	// while it was still an untyped param whose zero value was nil, not 0
	// - leaving it unset bound NULL and made the condition's "= 0" test
	// false instead of switching the filter off.
	n, err := s.q.CountRecipesFiltered(ctx, sqlc.CountRecipesFilteredParams{
		TagNames: "[]", TagCount: 0, MaxMinutes: 0, FavouritesOnly: 0, UserID: "",
	})
	if err != nil {
		return 0, fmt.Errorf("count recipes: %w", err)
	}
	return int(n), nil
}

// SetFavourite marks recipeID as favourited (on=true) or removes it
// (on=false) for userID. Setting the same state twice is not an error: the
// star is a state, not an event. Both branches key on userID directly, not
// on anything derived from recipeID or an ambient value, so a caller can
// only ever change its own favourites.
//
// Starring (on=true) returns ErrNotFound for a recipe that doesn't exist -
// checked explicitly, the same way Update and Delete check first, rather
// than letting the insert fail: favourites.recipe_id has a foreign key to
// recipes(id), and INSERT OR IGNORE only ignores its own uniqueness
// conflict (a duplicate (user_id, recipe_id) row), not a foreign key
// violation, so an unchecked insert against a missing recipe would surface
// as a raw constraint-violation error instead of the caller-facing
// ErrNotFound the handler maps to 404. Unstarring (on=false) is not
// checked this way on purpose: deleting a favourite that was never there,
// or whose recipe is already gone, is not an error - it's the same
// "already in the desired state" idempotency DELETE gives everywhere else
// in this package.
func (s *Service) SetFavourite(ctx context.Context, userID, recipeID string, on bool) error {
	if on {
		if _, err := s.q.GetRecipe(ctx, recipeID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return fmt.Errorf("get recipe %s: %w", recipeID, err)
		}
		if err := s.q.SetFavourite(ctx, sqlc.SetFavouriteParams{
			UserID: userID, RecipeID: recipeID, CreatedAt: db.FormatTime(s.now()),
		}); err != nil {
			return fmt.Errorf("set favourite: %w", err)
		}
		return nil
	}
	if err := s.q.DeleteFavourite(ctx, sqlc.DeleteFavouriteParams{UserID: userID, RecipeID: recipeID}); err != nil {
		return fmt.Errorf("delete favourite: %w", err)
	}
	return nil
}

// IsFavourite reports whether userID has favourited recipeID. An empty
// userID always reports false without querying, the same guard toCards
// applies to the per-page favourite batch - unauthenticated callers must
// never be told anything is favourited.
func (s *Service) IsFavourite(ctx context.Context, userID, recipeID string) (bool, error) {
	if userID == "" {
		return false, nil
	}
	idsJSON, err := json.Marshal([]string{recipeID})
	if err != nil {
		return false, fmt.Errorf("marshal recipe id: %w", err)
	}
	favIDs, err := s.q.ListFavouriteRecipeIDs(ctx, sqlc.ListFavouriteRecipeIDsParams{
		UserID: userID, RecipeIds: string(idsJSON),
	})
	if err != nil {
		return false, fmt.Errorf("list favourite recipe ids: %w", err)
	}
	return len(favIDs) == 1, nil
}

// load assembles a Recipe from its row plus child tables.
func (s *Service) load(ctx context.Context, row sqlc.Recipe) (Recipe, error) {
	groups, err := s.q.ListIngredientGroupsByRecipe(ctx, row.ID)
	if err != nil {
		return Recipe{}, fmt.Errorf("list ingredient groups: %w", err)
	}
	ingredients, err := s.q.ListIngredientsByRecipe(ctx, row.ID)
	if err != nil {
		return Recipe{}, fmt.Errorf("list ingredients: %w", err)
	}
	steps, err := s.q.ListStepsByRecipe(ctx, row.ID)
	if err != nil {
		return Recipe{}, fmt.Errorf("list steps: %w", err)
	}
	tagNames, err := s.q.ListTagNamesByRecipe(ctx, row.ID)
	if err != nil {
		return Recipe{}, fmt.Errorf("list tags: %w", err)
	}
	imgRows, err := s.q.ListImagesByRecipe(ctx, row.ID)
	if err != nil {
		return Recipe{}, fmt.Errorf("list images: %w", err)
	}
	created, err := db.ParseTime(row.CreatedAt)
	if err != nil {
		return Recipe{}, err
	}
	updated, err := db.ParseTime(row.UpdatedAt)
	if err != nil {
		return Recipe{}, err
	}

	byGroup := make(map[string][]Ingredient, len(groups))
	for _, g := range groups {
		byGroup[g.ID] = []Ingredient{}
	}
	for _, ing := range ingredients {
		byGroup[ing.GroupID] = append(byGroup[ing.GroupID], Ingredient{
			Quantity: ing.Quantity,
			Unit:     ing.Unit,
			Name:     ing.Name,
			Note:     ing.Note,
		})
	}
	outGroups := make([]IngredientGroup, len(groups))
	for i, g := range groups {
		outGroups[i] = IngredientGroup{Name: g.Name, Ingredients: byGroup[g.ID]}
	}

	outSteps := make([]string, len(steps))
	for i, st := range steps {
		outSteps[i] = st.Text
	}

	images := make([]Image, len(imgRows))
	for i, im := range imgRows {
		images[i] = Image{ID: im.ID, Width: int(im.Width), Height: int(im.Height), Position: int(im.Position)}
	}

	return Recipe{
		ID:   row.ID,
		Slug: row.Slug,
		Input: Input{
			Title:            row.Title,
			Description:      row.Description,
			Servings:         int(row.Servings),
			PrepMinutes:      int64ToIntPtr(row.PrepMinutes),
			CookMinutes:      int64ToIntPtr(row.CookMinutes),
			SourceURL:        row.SourceUrl,
			Tags:             tagNames,
			IngredientGroups: outGroups,
			Steps:            outSteps,
		},
		CoverImageID: row.CoverImageID,
		Images:       images,
		CreatedBy:    row.CreatedBy,
		CreatedAt:    created,
		UpdatedAt:    updated,
	}, nil
}

// writeChildren inserts the ingredient groups (with their ingredients) and
// steps of in under recipeID, in order.
func writeChildren(ctx context.Context, q *sqlc.Queries, recipeID string, in Input) error {
	for gi, group := range in.IngredientGroups {
		groupID := uuid.Must(uuid.NewV7()).String()
		if err := q.InsertIngredientGroup(ctx, sqlc.InsertIngredientGroupParams{
			ID:       groupID,
			RecipeID: recipeID,
			Name:     nonEmpty(group.Name),
			Position: int64(gi),
		}); err != nil {
			return fmt.Errorf("insert ingredient group: %w", err)
		}
		for ii, ing := range group.Ingredients {
			if err := q.InsertIngredient(ctx, sqlc.InsertIngredientParams{
				ID:       uuid.Must(uuid.NewV7()).String(),
				GroupID:  groupID,
				Quantity: ing.Quantity,
				Unit:     nonEmpty(ing.Unit),
				Name:     ing.Name,
				Note:     nonEmpty(ing.Note),
				Position: int64(ii),
			}); err != nil {
				return fmt.Errorf("insert ingredient: %w", err)
			}
		}
	}
	for si, step := range in.Steps {
		if err := q.InsertStep(ctx, sqlc.InsertStepParams{
			ID:       uuid.Must(uuid.NewV7()).String(),
			RecipeID: recipeID,
			Position: int64(si),
			Text:     step,
		}); err != nil {
			return fmt.Errorf("insert step: %w", err)
		}
	}
	return nil
}

// writeTags upserts each of tags and links it to recipeID.
func writeTags(ctx context.Context, q *sqlc.Queries, recipeID string, tags []string) error {
	for _, tag := range tags {
		tagID, err := q.UpsertTag(ctx, sqlc.UpsertTagParams{
			ID:   uuid.Must(uuid.NewV7()).String(),
			Name: tag,
		})
		if err != nil {
			return fmt.Errorf("upsert tag %q: %w", tag, err)
		}
		if err := q.SetRecipeTag(ctx, sqlc.SetRecipeTagParams{
			RecipeID: recipeID,
			TagID:    tagID,
		}); err != nil {
			return fmt.Errorf("set recipe tag %q: %w", tag, err)
		}
	}
	return nil
}

// reindex rebuilds the recipes_fts entry for recipeID from in and tags.
func reindex(ctx context.Context, q *sqlc.Queries, recipeID string, in Input, tags []string) error {
	rowid, err := q.GetRecipeRowID(ctx, recipeID)
	if err != nil {
		return fmt.Errorf("get recipe rowid: %w", err)
	}
	if err := q.DeleteFTS(ctx, rowid); err != nil {
		return fmt.Errorf("delete fts: %w", err)
	}
	var names []string
	for _, group := range in.IngredientGroups {
		for _, ing := range group.Ingredients {
			names = append(names, ing.Name)
		}
	}
	if err := q.InsertFTS(ctx, sqlc.InsertFTSParams{
		Rowid:       rowid,
		Title:       in.Title,
		Description: in.Description,
		Ingredients: strings.Join(names, " "),
		Tags:        strings.Join(tags, " "),
	}); err != nil {
		return fmt.Errorf("insert fts: %w", err)
	}
	return nil
}

// nonEmpty turns a pointer to an empty string into nil, so optional text
// columns store NULL rather than "".
func nonEmpty(p *string) *string {
	if p == nil || *p == "" {
		return nil
	}
	return p
}

// intToInt64Ptr adapts an optional int (Input's minute fields) to the
// *int64 sqlc parameter type; nil stays nil.
func intToInt64Ptr(p *int) *int64 {
	if p == nil {
		return nil
	}
	v := int64(*p)
	return &v
}

// int64ToIntPtr is the inverse of intToInt64Ptr, used when mapping a stored
// row back to an Input.
func int64ToIntPtr(p *int64) *int {
	if p == nil {
		return nil
	}
	v := int(*p)
	return &v
}
