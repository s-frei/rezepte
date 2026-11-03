package shopping

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
	"uuid"

	"github.com/s-frei/rezepte/service/internal/db"
	"github.com/s-frei/rezepte/service/internal/db/sqlc"
)

// Service stores shopping lists. A list is visible and changeable only by
// its creator; any other caller gets ErrNotFound.
type Service struct {
	conn *sql.DB
	q    *sqlc.Queries
	now  func() time.Time
}

// NewService returns a Service backed by conn.
func NewService(conn *sql.DB) *Service {
	return &Service{conn: conn, q: sqlc.New(conn), now: time.Now}
}

// SetClock overrides the time source. Intended for tests.
func (s *Service) SetClock(now func() time.Time) { s.now = now }

func (s *Service) stamp() string { return db.FormatTime(s.now()) }

func newID() string { return uuid.NewV7().String() }

func invalid(msg string) error { return fmt.Errorf("%w: %s", ErrInvalid, msg) }

// clean trims text and checks its length in runes.
func clean(field, text string, maxRunes int) (string, error) {
	text = strings.TrimSpace(text)
	if n := utf8.RuneCountInString(text); n == 0 || n > maxRunes {
		return "", invalid(fmt.Sprintf("%s must be 1-%d characters", field, maxRunes))
	}
	return text, nil
}

// Lists returns the caller's lists, most recently changed first.
func (s *Service) Lists(ctx context.Context, userID string) ([]Summary, error) {
	rows, err := s.q.ListShoppingLists(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list shopping lists: %w", err)
	}
	titles, err := s.q.ListShoppingEntryTitlesByCreator(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list shopping entries: %w", err)
	}
	byList := map[string][]SummaryRecipe{}
	for _, t := range titles {
		byList[t.ListID] = append(byList[t.ListID], SummaryRecipe{Title: t.Title, Servings: int(t.Servings)})
	}
	out := make([]Summary, 0, len(rows))
	for _, r := range rows {
		updated, err := db.ParseTime(r.UpdatedAt)
		if err != nil {
			return nil, err
		}
		recipes := byList[r.ID]
		if recipes == nil {
			recipes = []SummaryRecipe{}
		}
		out = append(out, Summary{ID: r.ID, Name: r.Name, OpenCount: int(r.OpenCount), DoneCount: int(r.DoneCount), Recipes: recipes, UpdatedAt: updated})
	}
	return out, nil
}

// Create makes an empty list.
func (s *Service) Create(ctx context.Context, userID, name string) (List, error) {
	name, err := clean("name", name, 80)
	if err != nil {
		return List{}, err
	}
	id, now := newID(), s.stamp()
	if err := s.q.InsertShoppingList(ctx, sqlc.InsertShoppingListParams{ID: id, Name: name, CreatedBy: userID, CreatedAt: now, UpdatedAt: now}); err != nil {
		return List{}, fmt.Errorf("insert shopping list: %w", err)
	}
	return s.Get(ctx, userID, id)
}

// Get loads one list with entries and items, quantities scaled to each
// entry's servings and every item sorted into its department.
func (s *Service) Get(ctx context.Context, userID, id string) (List, error) {
	return get(ctx, s.q, userID, id)
}

func get(ctx context.Context, q *sqlc.Queries, userID, id string) (List, error) {
	row, err := q.GetShoppingList(ctx, sqlc.GetShoppingListParams{ID: id, CreatedBy: userID})
	if errors.Is(err, sql.ErrNoRows) {
		return List{}, ErrNotFound
	}
	if err != nil {
		return List{}, fmt.Errorf("get shopping list: %w", err)
	}
	l := List{ID: row.ID, Name: row.Name, CreatedBy: row.CreatedBy, Entries: []Entry{}, Items: []Item{}}
	if l.CreatedAt, err = db.ParseTime(row.CreatedAt); err != nil {
		return List{}, err
	}
	if l.UpdatedAt, err = db.ParseTime(row.UpdatedAt); err != nil {
		return List{}, err
	}
	entries, err := q.ListShoppingEntries(ctx, id)
	if err != nil {
		return List{}, fmt.Errorf("list shopping entries: %w", err)
	}
	type span struct{ base, servings float64 }
	spans := map[string]span{}
	for _, e := range entries {
		l.Entries = append(l.Entries, Entry{
			ID: e.ID, RecipeID: e.RecipeID, RecipeSlug: e.RecipeSlug, CoverImageID: e.CoverImageID,
			Title: e.Title, BaseServings: int(e.BaseServings), Servings: int(e.Servings),
		})
		spans[e.ID] = span{float64(e.BaseServings), float64(e.Servings)}
	}
	items, err := q.ListShoppingItems(ctx, id)
	if err != nil {
		return List{}, fmt.Errorf("list shopping items: %w", err)
	}
	overrides, err := q.ListDepartmentOverrides(ctx)
	if err != nil {
		return List{}, fmt.Errorf("list department overrides: %w", err)
	}
	override := make(map[string]Department, len(overrides))
	for _, o := range overrides {
		override[o.NameKey] = Department(o.Department)
	}
	// Counts are rows of the department view, one per name key: open while
	// any item behind it is, done once all are (ListShoppingLists agrees).
	open := map[string]bool{}
	for _, r := range items {
		it := Item{
			ID: r.ID, EntryID: r.EntryID, Name: r.Name, NameKey: r.NameKey,
			Quantity: r.Quantity, Unit: r.Unit, Note: r.Note, Group: r.GroupName,
			Checked: r.CheckedAt != nil,
		}
		if sp, ok := spans[deref(r.EntryID)]; ok && r.Quantity != nil {
			v := RoundScaled(ScaleQuantity(*r.Quantity, sp.base, sp.servings))
			it.Quantity = &v
		}
		if d, ok := override[r.NameKey]; ok {
			it.Department = d
		} else {
			it.Department = DefaultDepartment(r.NameKey)
		}
		if r.EntryID == nil && r.AddedBy != nil && r.AdderName != nil {
			it.AddedBy = &Adder{ID: *r.AddedBy, DisplayName: *r.AdderName, Color: deref(r.AdderColor), AvatarID: r.AdderAvatarID}
		}
		open[r.NameKey] = open[r.NameKey] || !it.Checked
		l.Items = append(l.Items, it)
	}
	for _, o := range open {
		if o {
			l.OpenCount++
		} else {
			l.DoneCount++
		}
	}
	return l, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// Rename changes a list's name.
func (s *Service) Rename(ctx context.Context, userID, id, name string) (List, error) {
	name, err := clean("name", name, 80)
	if err != nil {
		return List{}, err
	}
	n, err := s.q.RenameShoppingList(ctx, sqlc.RenameShoppingListParams{Name: name, UpdatedAt: s.stamp(), ID: id, CreatedBy: userID})
	if err != nil {
		return List{}, fmt.Errorf("rename shopping list: %w", err)
	}
	if n == 0 {
		return List{}, ErrNotFound
	}
	return s.Get(ctx, userID, id)
}

// Delete removes a list with everything on it.
func (s *Service) Delete(ctx context.Context, userID, id string) error {
	n, err := s.q.DeleteShoppingList(ctx, sqlc.DeleteShoppingListParams{ID: id, CreatedBy: userID})
	if err != nil {
		return fmt.Errorf("delete shopping list: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// mutate runs fn in a transaction after touching the list (which proves it
// is the caller's), then returns the list as it is afterwards.
func (s *Service) mutate(ctx context.Context, userID, listID string, fn func(q *sqlc.Queries) error) (List, error) {
	var l List
	err := db.Tx(ctx, s.conn, func(q *sqlc.Queries) error {
		n, err := q.TouchShoppingList(ctx, sqlc.TouchShoppingListParams{UpdatedAt: s.stamp(), ID: listID, CreatedBy: userID})
		if err != nil {
			return fmt.Errorf("touch shopping list: %w", err)
		}
		if n == 0 {
			return ErrNotFound
		}
		if err := fn(q); err != nil {
			return err
		}
		l, err = get(ctx, q, userID, listID)
		return err
	})
	return l, err
}

// AddRecipe copies the included ingredients of a recipe onto a list as one
// entry at the given servings. The copy keeps the recipe's amounts; later
// edits to the recipe do not reach it.
func (s *Service) AddRecipe(ctx context.Context, userID, listID, recipeID string, servings int, include []IngredientPick) (List, error) {
	if servings < 1 || servings > 99 {
		return List{}, invalid("servings must be 1-99")
	}
	return s.mutate(ctx, userID, listID, func(q *sqlc.Queries) error {
		rec, err := q.GetRecipe(ctx, recipeID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrRecipeNotFound
		}
		if err != nil {
			return fmt.Errorf("get recipe: %w", err)
		}
		if len(include) == 0 {
			return invalid("pick at least one ingredient")
		}
		groups, err := q.ListIngredientGroupsByRecipe(ctx, recipeID)
		if err != nil {
			return fmt.Errorf("list ingredient groups: %w", err)
		}
		ings, err := q.ListIngredientsByRecipe(ctx, recipeID)
		if err != nil {
			return fmt.Errorf("list ingredients: %w", err)
		}
		byGroup := map[string][]sqlc.Ingredient{}
		for _, i := range ings {
			byGroup[i.GroupID] = append(byGroup[i.GroupID], i)
		}
		pos, err := q.MaxShoppingItemPosition(ctx, listID)
		if err != nil {
			return fmt.Errorf("max item position: %w", err)
		}
		now, entryID := s.stamp(), newID()
		// Validate every ref before writing anything.
		picked := make([]sqlc.Ingredient, 0, len(include))
		pickedGroup := make([]*string, 0, len(include))
		for _, ref := range include {
			if ref.Group < 0 || ref.Group >= len(groups) || ref.Index < 0 || ref.Index >= len(byGroup[groups[ref.Group].ID]) {
				return invalid(fmt.Sprintf("ingredient %d/%d does not exist", ref.Group, ref.Index))
			}
			picked = append(picked, byGroup[groups[ref.Group].ID][ref.Index])
			pickedGroup = append(pickedGroup, groups[ref.Group].Name)
		}
		if err := q.InsertShoppingEntry(ctx, sqlc.InsertShoppingEntryParams{
			ID: entryID, ListID: listID, RecipeID: &recipeID, Title: rec.Title,
			BaseServings: rec.Servings, Servings: int64(servings), CreatedAt: now,
		}); err != nil {
			return fmt.Errorf("insert shopping entry: %w", err)
		}
		for i, ing := range picked {
			pos++
			if err := q.InsertShoppingItem(ctx, sqlc.InsertShoppingItemParams{
				ID: newID(), ListID: listID, EntryID: &entryID, Name: ing.Name, NameKey: NameKey(ing.Name),
				Quantity: ing.Quantity, Unit: ing.Unit, Note: ing.Note, GroupName: pickedGroup[i],
				Position: pos, CreatedAt: now,
			}); err != nil {
				return fmt.Errorf("insert shopping item: %w", err)
			}
		}
		return nil
	})
}

// SetServings changes an entry's servings; its items rescale and keep ticks.
func (s *Service) SetServings(ctx context.Context, userID, listID, entryID string, servings int) (List, error) {
	if servings < 1 || servings > 99 {
		return List{}, invalid("servings must be 1-99")
	}
	return s.mutate(ctx, userID, listID, func(q *sqlc.Queries) error {
		n, err := q.SetShoppingEntryServings(ctx, sqlc.SetShoppingEntryServingsParams{Servings: int64(servings), ID: entryID, ListID: listID})
		if err != nil {
			return fmt.Errorf("set entry servings: %w", err)
		}
		if n == 0 {
			return ErrNotFound
		}
		return nil
	})
}

// RemoveEntry drops a recipe entry and its items.
func (s *Service) RemoveEntry(ctx context.Context, userID, listID, entryID string) (List, error) {
	return s.mutate(ctx, userID, listID, func(q *sqlc.Queries) error {
		n, err := q.DeleteShoppingEntry(ctx, sqlc.DeleteShoppingEntryParams{ID: entryID, ListID: listID})
		if err != nil {
			return fmt.Errorf("delete shopping entry: %w", err)
		}
		if n == 0 {
			return ErrNotFound
		}
		return nil
	})
}

// AddFree adds one item typed as text such as "2 l Milch".
func (s *Service) AddFree(ctx context.Context, userID, listID, text string) (List, error) {
	text, err := clean("text", text, 200)
	if err != nil {
		return List{}, err
	}
	qty, unit, name, note := ParseFree(text)
	return s.mutate(ctx, userID, listID, func(q *sqlc.Queries) error {
		pos, err := q.MaxShoppingItemPosition(ctx, listID)
		if err != nil {
			return fmt.Errorf("max item position: %w", err)
		}
		if err := q.InsertShoppingItem(ctx, sqlc.InsertShoppingItemParams{
			ID: newID(), ListID: listID, Name: name, NameKey: NameKey(name),
			Quantity: qty, Unit: unit, Note: note, Position: pos + 1, AddedBy: &userID, CreatedAt: s.stamp(),
		}); err != nil {
			return fmt.Errorf("insert shopping item: %w", err)
		}
		return nil
	})
}

// SetChecked ticks or unticks items; ids that are not on the list are ignored.
func (s *Service) SetChecked(ctx context.Context, userID, listID string, itemIDs []string, checked bool) (List, error) {
	return s.mutate(ctx, userID, listID, func(q *sqlc.Queries) error {
		if len(itemIDs) == 0 {
			return nil
		}
		var at *string
		if checked {
			now := s.stamp()
			at = &now
		}
		if err := q.SetShoppingItemsChecked(ctx, sqlc.SetShoppingItemsCheckedParams{CheckedAt: at, ListID: listID, Ids: itemIDs}); err != nil {
			return fmt.Errorf("set items checked: %w", err)
		}
		return nil
	})
}

// RemoveItems deletes items; ids that are not on the list are ignored.
func (s *Service) RemoveItems(ctx context.Context, userID, listID string, itemIDs []string) (List, error) {
	return s.mutate(ctx, userID, listID, func(q *sqlc.Queries) error {
		if len(itemIDs) == 0 {
			return nil
		}
		if err := q.DeleteShoppingItems(ctx, sqlc.DeleteShoppingItemsParams{ListID: listID, Ids: itemIDs}); err != nil {
			return fmt.Errorf("delete shopping items: %w", err)
		}
		return nil
	})
}

// ClearChecked deletes every ticked item.
func (s *Service) ClearChecked(ctx context.Context, userID, listID string) (List, error) {
	return s.mutate(ctx, userID, listID, func(q *sqlc.Queries) error {
		if err := q.DeleteCheckedShoppingItems(ctx, listID); err != nil {
			return fmt.Errorf("clear checked items: %w", err)
		}
		return nil
	})
}

// SetDepartment sorts every item called name (by NameKey) under d on every
// list of the instance.
func (s *Service) SetDepartment(ctx context.Context, userID, name string, d Department) error {
	key := NameKey(name)
	if key == "" {
		return invalid("name must not be empty")
	}
	if !d.Valid() {
		return invalid("unknown department")
	}
	err := s.q.UpsertDepartmentOverride(ctx, sqlc.UpsertDepartmentOverrideParams{NameKey: key, Department: string(d), UpdatedBy: &userID, UpdatedAt: s.stamp()})
	if err != nil {
		return fmt.Errorf("set department override: %w", err)
	}
	return nil
}

// CountsByCreator returns how many lists each person has.
func (s *Service) CountsByCreator(ctx context.Context) (map[string]int, error) {
	rows, err := s.q.CountShoppingListsByCreator(ctx)
	if err != nil {
		return nil, fmt.Errorf("count shopping lists: %w", err)
	}
	out := make(map[string]int, len(rows))
	for _, r := range rows {
		out[r.CreatedBy] = int(r.Lists)
	}
	return out, nil
}
