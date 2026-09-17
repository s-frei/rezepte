package recipe

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/s-frei/rezepte/service/internal/db"
	"github.com/s-frei/rezepte/service/internal/db/sqlc"
)

// Service creates, updates, deletes and loads recipes, keeping slugs, tags
// and the full-text search index consistent with the stored documents.
type Service struct {
	conn *sql.DB
	q    *sqlc.Queries
	now  func() time.Time
}

// NewService returns a Service backed by conn.
func NewService(conn *sql.DB) *Service {
	return &Service{conn: conn, q: sqlc.New(conn), now: time.Now}
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

// Update replaces the title, description and all child rows (ingredient
// groups, ingredients, steps, tags) of the recipe id, keeping its slug and
// creation metadata. It returns ErrNotFound when no such recipe exists.
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
	return db.Tx(ctx, s.conn, func(q *sqlc.Queries) error {
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
		Images:       []struct{}{},
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
