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
	"github.com/s-frei/rezepte/service/internal/recipe"
	"github.com/s-frei/rezepte/service/internal/recipeimport"
)

// Service stores shopping lists. A list is visible and changeable by its
// creator and its accepted members, and only the creator can delete it; any
// other caller gets ErrNotFound.
type Service struct {
	conn *sql.DB
	q    *sqlc.Queries
	now  func() time.Time
	inv  InviteNotifier
	hub  *Hub
}

// InviteNotifier hears of every new invitation, after it was stored; not of
// an invitation that existed already.
type InviteNotifier interface {
	ListInvited(ctx context.Context, listID, inviterID, inviteeID string)
}

// SetInviteNotifier hands every new invitation to n; nil (the default) is off.
func (s *Service) SetInviteNotifier(n InviteNotifier) { s.inv = n }

// Hub is where every committed change is published for the open event
// streams; main closes it on shutdown.
func (s *Service) Hub() *Hub { return s.hub }

// CanAccess reports whether the person created or accepted the list.
func (s *Service) CanAccess(ctx context.Context, userID, listID string) (bool, error) {
	ok, err := s.q.CanAccessShoppingList(ctx, sqlc.CanAccessShoppingListParams{ID: listID, UserID: userID})
	if err != nil {
		return false, fmt.Errorf("check shopping list access: %w", err)
	}
	return ok, nil
}

// changed bumps the revision of a list whose change did not touch its row
// (members), then tells its open streams.
func (s *Service) changed(ctx context.Context, listID string) error {
	if err := s.q.BumpShoppingListRevision(ctx, listID); err != nil {
		return fmt.Errorf("bump shopping list revision: %w", err)
	}
	s.hub.Publish(listID)
	return nil
}

// NewService returns a Service backed by conn.
func NewService(conn *sql.DB) *Service {
	return &Service{conn: conn, q: sqlc.New(conn), now: time.Now, hub: NewHub()}
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

// Lists returns the lists the caller created or accepted, most recently
// changed first.
func (s *Service) Lists(ctx context.Context, userID string) ([]Summary, error) {
	rows, err := s.q.ListShoppingLists(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list shopping lists: %w", err)
	}
	titles, err := s.q.ListShoppingEntryTitlesForUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list shopping entries: %w", err)
	}
	byList := map[string][]SummaryRecipe{}
	for _, t := range titles {
		byList[t.ListID] = append(byList[t.ListID], SummaryRecipe{Title: t.Title, Servings: int(t.Servings), RecipeID: t.RecipeID})
	}
	ids := make([]string, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}
	members, _, err := listMembers(ctx, s.q, ids, true)
	if err != nil {
		return nil, err
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
		sum := Summary{
			ID: r.ID, Name: r.Name, OpenCount: int(r.OpenCount), DoneCount: int(r.DoneCount), Recipes: recipes, UpdatedAt: updated,
			Owner:   ListPerson{ID: r.CreatedBy, DisplayName: r.OwnerName, Color: r.OwnerColor, AvatarID: r.OwnerAvatarID},
			Members: members[r.ID],
		}
		if r.UpdatedBy != nil && r.UpdaterName != nil {
			sum.UpdatedBy = &ListPerson{ID: *r.UpdatedBy, DisplayName: *r.UpdaterName, Color: deref(r.UpdaterColor), AvatarID: r.UpdaterAvatarID}
		}
		out = append(out, sum)
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
	if err := s.q.InsertShoppingList(ctx, sqlc.InsertShoppingListParams{ID: id, Name: name, CreatedBy: userID, CreatedAt: now, UpdatedAt: now, UpdatedBy: &userID}); err != nil {
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
	row, err := q.GetShoppingList(ctx, sqlc.GetShoppingListParams{ID: id, UserID: userID})
	if errors.Is(err, sql.ErrNoRows) {
		return List{}, ErrNotFound
	}
	if err != nil {
		return List{}, fmt.Errorf("get shopping list: %w", err)
	}
	l := List{
		ID: row.ID, Name: row.Name, CreatedBy: row.CreatedBy, Revision: row.Revision, Entries: []Entry{}, Items: []Item{},
		Owner: ListPerson{ID: row.CreatedBy, DisplayName: row.OwnerName, Color: row.OwnerColor, AvatarID: row.OwnerAvatarID},
	}
	members, pending, err := listMembers(ctx, q, []string{id}, userID == row.CreatedBy)
	if err != nil {
		return List{}, err
	}
	l.Members, l.Pending = members[id], pending[id]
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
			it.AddedBy = &ListPerson{ID: *r.AddedBy, DisplayName: *r.AdderName, Color: deref(r.AdderColor), AvatarID: r.AdderAvatarID}
		}
		if it.Checked && r.CheckedBy != nil && r.CheckerName != nil {
			it.CheckedBy = &ListPerson{ID: *r.CheckedBy, DisplayName: *r.CheckerName, Color: deref(r.CheckerColor), AvatarID: r.CheckerAvatarID}
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

// listMembers returns the accepted members of the lists by list ID, and the
// pending invitees when withPending is set. Absent lists map to empty slices.
func listMembers(ctx context.Context, q *sqlc.Queries, listIDs []string, withPending bool) (members, pending map[string][]ListPerson, err error) {
	members, pending = map[string][]ListPerson{}, map[string][]ListPerson{}
	for _, id := range listIDs {
		members[id], pending[id] = []ListPerson{}, []ListPerson{}
	}
	if len(listIDs) == 0 {
		return members, pending, nil
	}
	rows, err := q.ListShoppingListMembers(ctx, listIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("list shopping list members: %w", err)
	}
	for _, r := range rows {
		p := ListPerson{ID: r.UserID, DisplayName: r.DisplayName, Color: r.Color, AvatarID: r.AvatarID}
		if r.AcceptedAt != nil {
			members[r.ListID] = append(members[r.ListID], p)
		} else if withPending {
			pending[r.ListID] = append(pending[r.ListID], p)
		}
	}
	return members, pending, nil
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
	n, err := s.q.RenameShoppingList(ctx, sqlc.RenameShoppingListParams{Name: name, UpdatedAt: s.stamp(), ID: id, UserID: &userID})
	if err != nil {
		return List{}, fmt.Errorf("rename shopping list: %w", err)
	}
	if n == 0 {
		return List{}, ErrNotFound
	}
	s.hub.Publish(id)
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
	s.hub.Publish(id)
	return nil
}

// Leave ends the caller's membership whatever its state, so it also declines
// an invitation. The creator cannot leave their own list.
func (s *Service) Leave(ctx context.Context, userID, listID string) error {
	creator, err := s.q.GetShoppingListCreator(ctx, listID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("get shopping list creator: %w", err)
	}
	if err == nil && creator == userID {
		return invalid("the creator cannot leave")
	}
	n, err := s.q.DeleteShoppingMembership(ctx, sqlc.DeleteShoppingMembershipParams{ListID: listID, UserID: userID})
	if err != nil {
		return fmt.Errorf("delete shopping membership: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return s.changed(ctx, listID)
}

// ownList reports ErrNotFound unless the caller created the list.
func (s *Service) ownList(ctx context.Context, userID, listID string) error {
	creator, err := s.q.GetShoppingListCreator(ctx, listID)
	if errors.Is(err, sql.ErrNoRows) || err == nil && creator != userID {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("get shopping list creator: %w", err)
	}
	return nil
}

// Invite asks a person to share a list; only the creator can. An existing
// row, pending or accepted, stays as it is and nobody is notified again.
func (s *Service) Invite(ctx context.Context, userID, listID, inviteeID string) (List, error) {
	if err := s.ownList(ctx, userID, listID); err != nil {
		return List{}, err
	}
	if inviteeID == userID {
		return List{}, invalid("you cannot invite yourself")
	}
	if _, err := s.q.GetUserByID(ctx, inviteeID); errors.Is(err, sql.ErrNoRows) {
		return List{}, invalid("no such person")
	} else if err != nil {
		return List{}, fmt.Errorf("get invitee: %w", err)
	}
	n, err := s.q.InsertShoppingMember(ctx, sqlc.InsertShoppingMemberParams{ListID: listID, UserID: inviteeID, InvitedAt: s.stamp()})
	if err != nil {
		return List{}, fmt.Errorf("insert shopping member: %w", err)
	}
	if n > 0 {
		if err := s.changed(ctx, listID); err != nil {
			return List{}, err
		}
		if s.inv != nil {
			s.inv.ListInvited(ctx, listID, userID, inviteeID)
		}
	}
	return s.Get(ctx, userID, listID)
}

// RemoveMember lets the creator withdraw an invitation or remove a member.
func (s *Service) RemoveMember(ctx context.Context, userID, listID, memberID string) (List, error) {
	n, err := s.q.DeleteShoppingMember(ctx, sqlc.DeleteShoppingMemberParams{ListID: listID, MemberID: memberID, UserID: userID})
	if err != nil {
		return List{}, fmt.Errorf("delete shopping member: %w", err)
	}
	if n == 0 {
		return List{}, ErrNotFound
	}
	if err := s.changed(ctx, listID); err != nil {
		return List{}, err
	}
	return s.Get(ctx, userID, listID)
}

// Accept turns the caller's pending invitation into a membership.
func (s *Service) Accept(ctx context.Context, userID, listID string) (List, error) {
	now := s.stamp()
	n, err := s.q.AcceptShoppingMembership(ctx, sqlc.AcceptShoppingMembershipParams{AcceptedAt: &now, ListID: listID, UserID: userID})
	if err != nil {
		return List{}, fmt.Errorf("accept shopping invitation: %w", err)
	}
	if n == 0 {
		return List{}, ErrNotFound
	}
	if err := s.changed(ctx, listID); err != nil {
		return List{}, err
	}
	return s.Get(ctx, userID, listID)
}

// Invitations returns the caller's pending invitations, newest first.
func (s *Service) Invitations(ctx context.Context, userID string) ([]Invitation, error) {
	rows, err := s.q.ListShoppingInvitations(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list shopping invitations: %w", err)
	}
	out := make([]Invitation, 0, len(rows))
	for _, r := range rows {
		at, err := db.ParseTime(r.InvitedAt)
		if err != nil {
			return nil, err
		}
		out = append(out, Invitation{
			ListID: r.ID, ListName: r.Name, OpenCount: int(r.OpenCount), InvitedAt: at,
			From: ListPerson{ID: r.CreatedBy, DisplayName: r.OwnerName, Color: r.OwnerColor, AvatarID: r.OwnerAvatarID},
		})
	}
	return out, nil
}

// mutate runs fn in a transaction after touching the list (which proves the
// caller has access), then returns the list as it is afterwards and, once
// committed, tells the hub.
func (s *Service) mutate(ctx context.Context, userID, listID string, fn func(q *sqlc.Queries) error) (List, error) {
	var l List
	err := db.Tx(ctx, s.conn, func(q *sqlc.Queries) error {
		n, err := q.TouchShoppingList(ctx, sqlc.TouchShoppingListParams{UpdatedAt: s.stamp(), ID: listID, UserID: &userID})
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
	if err != nil {
		return List{}, err
	}
	s.hub.Publish(listID)
	return l, nil
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
	return s.AddItem(ctx, userID, listID, text, nil, nil)
}

// AddItem adds one item from text parsed as AddFree does; a given quantity or
// unit (the app's own fields) replaces the parsed one.
func (s *Service) AddItem(ctx context.Context, userID, listID, text string, quantity *float64, unitIn *string) (List, error) {
	text, err := clean("text", text, 200)
	if err != nil {
		return List{}, err
	}
	qty, unit, name, note := ParseFree(text)
	if quantity != nil {
		qty = quantity
	}
	if unitIn != nil {
		if u := strings.TrimSpace(*unitIn); u != "" {
			unit = &u
		}
	}
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

// ImportList makes a list from pasted text (ParseListText) in one
// transaction. Its name is name if given, else the text's name line, else
// defaultName, cut at a word boundary to 80 characters (recipeimport.Cut).
func (s *Service) ImportList(ctx context.Context, userID, text, name, defaultName string) (List, error) {
	lt, err := ParseListText(text)
	if err != nil {
		return List{}, err
	}
	if name = strings.TrimSpace(name); name == "" {
		name = lt.Name
	}
	if name == "" {
		name = strings.TrimSpace(defaultName)
	}
	name, _ = recipeimport.Cut(name, 80)
	name, err = clean("name", name, 80)
	if err != nil {
		return List{}, err
	}
	id, now := newID(), s.stamp()
	err = db.Tx(ctx, s.conn, func(q *sqlc.Queries) error {
		if err := q.InsertShoppingList(ctx, sqlc.InsertShoppingListParams{ID: id, Name: name, CreatedBy: userID, CreatedAt: now, UpdatedAt: now, UpdatedBy: &userID}); err != nil {
			return fmt.Errorf("insert shopping list: %w", err)
		}
		return insertFree(ctx, q, id, userID, 0, now, lt.Items)
	})
	if err != nil {
		return List{}, err
	}
	s.hub.Publish(id)
	return s.Get(ctx, userID, id)
}

// ImportItems appends the items of pasted text (ParseListText) to a list; a
// name line in the text is skipped, the list keeps its name.
func (s *Service) ImportItems(ctx context.Context, userID, listID, text string) (List, error) {
	lt, err := ParseListText(text)
	if err != nil {
		return List{}, err
	}
	return s.mutate(ctx, userID, listID, func(q *sqlc.Queries) error {
		pos, err := q.MaxShoppingItemPosition(ctx, listID)
		if err != nil {
			return fmt.Errorf("max item position: %w", err)
		}
		return insertFree(ctx, q, listID, userID, pos, s.stamp(), lt.Items)
	})
}

// insertFree adds items as userID's free items after position pos.
func insertFree(ctx context.Context, q *sqlc.Queries, listID, userID string, pos int64, now string, items []recipe.Ingredient) error {
	for _, it := range items {
		pos++
		if err := q.InsertShoppingItem(ctx, sqlc.InsertShoppingItemParams{
			ID: newID(), ListID: listID, Name: it.Name, NameKey: NameKey(it.Name),
			Quantity: it.Quantity, Unit: it.Unit, Note: it.Note, Position: pos, AddedBy: &userID, CreatedAt: now,
		}); err != nil {
			return fmt.Errorf("insert shopping item: %w", err)
		}
	}
	return nil
}

// SetChecked ticks or unticks items; ids that are not on the list are ignored.
func (s *Service) SetChecked(ctx context.Context, userID, listID string, itemIDs []string, checked bool) (List, error) {
	return s.mutate(ctx, userID, listID, func(q *sqlc.Queries) error {
		if len(itemIDs) == 0 {
			return nil
		}
		var at, by *string
		if checked {
			now := s.stamp()
			at, by = &now, &userID
		}
		if err := q.SetShoppingItemsChecked(ctx, sqlc.SetShoppingItemsCheckedParams{CheckedAt: at, CheckedBy: by, ListID: listID, Ids: itemIDs}); err != nil {
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
	if err := s.q.BumpAllShoppingListRevisions(ctx); err != nil {
		return fmt.Errorf("bump shopping list revisions: %w", err)
	}
	s.hub.PublishAll()
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
