package mealplan

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
	"uuid"

	"github.com/s-frei/rezepte/service/internal/db"
	"github.com/s-frei/rezepte/service/internal/db/sqlc"
	"github.com/s-frei/rezepte/service/internal/shopping"
)

// purgeAfter is how long a removed entry stays restorable.
const purgeAfter = 24 * time.Hour

// Service stores meal plans. A plan is visible and changeable by its creator
// and its accepted members, and only the creator can delete it; any other
// caller gets ErrNotFound. Everyone has one personal plan, made on first use,
// that cannot be shared, renamed or deleted.
type Service struct {
	conn  *sql.DB
	q     *sqlc.Queries
	now   func() time.Time
	hub   *shopping.Hub
	lists *shopping.Service
	inv   InviteNotifier
}

// InviteNotifier hears of every new invitation, after it was stored; not of
// ones that already existed.
type InviteNotifier interface {
	PlanInvited(ctx context.Context, planID, inviterID, inviteeID string)
}

// SetInviteNotifier hands every new invitation to n; nil (the default) is off.
func (s *Service) SetInviteNotifier(n InviteNotifier) { s.inv = n }

// NewService returns a Service backed by conn; lists is the shopping lists'
// service the plan puts recipes on.
func NewService(conn *sql.DB, lists *shopping.Service) *Service {
	return &Service{conn: conn, q: sqlc.New(conn), now: time.Now, hub: shopping.NewHub(), lists: lists}
}

// SetClock overrides the time source. Intended for tests.
func (s *Service) SetClock(now func() time.Time) { s.now = now }

// Hub is where every committed change is published for the open event
// streams, keyed by plan id; main closes it on shutdown.
func (s *Service) Hub() *shopping.Hub { return s.hub }

// Week is Monday and Sunday of the current week.
func (s *Service) Week() (from, to string) { return week(s.now()) }

func (s *Service) stamp() string { return db.FormatTime(s.now()) }

func newID() string { return uuid.NewV7().String() }

// clean trims text and checks its length in runes.
func clean(field, text string, maxRunes int) (string, error) {
	text = strings.TrimSpace(text)
	if n := utf8.RuneCountInString(text); n == 0 || n > maxRunes {
		return "", invalid(fmt.Sprintf("%s must be 1-%d characters", field, maxRunes))
	}
	return text, nil
}

func checkServings(n int) error {
	if n < 1 || n > 99 {
		return invalid("servings must be 1-99")
	}
	return nil
}

func person(id, name, color string, avatar *string) Person {
	return Person{ID: id, DisplayName: name, Color: color, AvatarID: avatar}
}

// Plans returns the caller's plans, the personal one first (made here on
// first use), then the most recently changed, and their pending invitations.
func (s *Service) Plans(ctx context.Context, userID string) ([]PlanSummary, []PlanInvitation, error) {
	if err := s.q.InsertPersonalMealPlan(ctx, sqlc.InsertPersonalMealPlanParams{ID: newID(), UserID: userID, Now: s.stamp()}); err != nil {
		return nil, nil, fmt.Errorf("insert personal meal plan: %w", err)
	}
	rows, err := s.q.ListMealPlans(ctx, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("list meal plans: %w", err)
	}
	ids := make([]string, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}
	members, _, err := planMembers(ctx, s.q, ids, false)
	if err != nil {
		return nil, nil, err
	}
	plans := make([]PlanSummary, 0, len(rows))
	for _, r := range rows {
		plans = append(plans, PlanSummary{
			ID: r.ID, Name: r.Name, Personal: r.Personal == 1, UpdatedAt: r.UpdatedAt,
			Owner: person(r.CreatedBy, r.OwnerName, r.OwnerColor, r.OwnerAvatarID), Members: members[r.ID],
		})
	}
	inv, err := s.q.ListMealPlanInvitations(ctx, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("list meal plan invitations: %w", err)
	}
	invitations := make([]PlanInvitation, 0, len(inv))
	for _, r := range inv {
		invitations = append(invitations, PlanInvitation{PlanID: r.ID, Name: r.Name, Inviter: person(r.CreatedBy, r.OwnerName, r.OwnerColor, r.OwnerAvatarID)})
	}
	return plans, invitations, nil
}

// Create makes an empty shared plan and returns it for the current week.
func (s *Service) Create(ctx context.Context, userID, name string) (Plan, error) {
	name, err := clean("name", name, 80)
	if err != nil {
		return Plan{}, err
	}
	id, now := newID(), s.stamp()
	if err := s.q.InsertMealPlan(ctx, sqlc.InsertMealPlanParams{ID: id, Name: &name, CreatedBy: userID, CreatedAt: now, UpdatedAt: now, UpdatedBy: &userID}); err != nil {
		return Plan{}, fmt.Errorf("insert meal plan: %w", err)
	}
	from, to := week(s.now())
	return s.Get(ctx, userID, id, from, to)
}

// Rename changes a shared plan's name and returns it for the current week.
func (s *Service) Rename(ctx context.Context, userID, id, name string) (Plan, error) {
	name, err := clean("name", name, 80)
	if err != nil {
		return Plan{}, err
	}
	from, to := week(s.now())
	return s.mutate(ctx, userID, id, from, to, func(q *sqlc.Queries) error {
		n, err := q.RenameMealPlan(ctx, sqlc.RenameMealPlanParams{Name: &name, ID: id})
		if err != nil {
			return fmt.Errorf("rename meal plan: %w", err)
		}
		if n == 0 {
			return personal()
		}
		return nil
	})
}

// Delete removes a shared plan with everything on it, its blocks on every
// list too; only its creator can.
func (s *Service) Delete(ctx context.Context, userID, id string) error {
	var lists []string
	err := db.Tx(ctx, s.conn, func(q *sqlc.Queries) error {
		ids, err := q.ListMealPlanEntryIDs(ctx, id)
		if err != nil {
			return fmt.Errorf("list meal plan entries: %w", err)
		}
		if lists, err = shopping.FollowPlan(ctx, q, ids, nil); err != nil {
			return err
		}
		n, err := q.DeleteMealPlan(ctx, sqlc.DeleteMealPlanParams{ID: id, CreatedBy: userID})
		if err != nil {
			return fmt.Errorf("delete meal plan: %w", err)
		}
		if n == 0 {
			return ErrNotFound
		}
		return nil
	})
	if errors.Is(err, ErrNotFound) {
		p, err := s.q.GetMealPlanCreator(ctx, id)
		if err == nil && p.CreatedBy == userID && p.Personal == 1 {
			return personal()
		}
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("get meal plan creator: %w", err)
		}
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	s.hub.Publish(id)
	s.lists.Hub().Publish(lists...)
	return nil
}

// Get loads a plan with its entries from from to to (inclusive, YYYY-MM-DD,
// at most 62 days apart) and on the tray.
func (s *Service) Get(ctx context.Context, userID, id, from, to string) (Plan, error) {
	if err := checkRange(from, to); err != nil {
		return Plan{}, err
	}
	return get(ctx, s.q, userID, id, from, to)
}

func get(ctx context.Context, q *sqlc.Queries, userID, id, from, to string) (Plan, error) {
	row, err := q.GetMealPlan(ctx, sqlc.GetMealPlanParams{ID: id, UserID: userID})
	if errors.Is(err, sql.ErrNoRows) {
		return Plan{}, ErrNotFound
	}
	if err != nil {
		return Plan{}, fmt.Errorf("get meal plan: %w", err)
	}
	p := Plan{
		ID: row.ID, Name: row.Name, Personal: row.Personal == 1, Revision: row.Revision, From: from, To: to,
		Owner: person(row.CreatedBy, row.OwnerName, row.OwnerColor, row.OwnerAvatarID), Entries: []PlanEntry{}, Lists: []LinkedList{},
	}
	members, pending, err := planMembers(ctx, q, []string{id}, userID == row.CreatedBy)
	if err != nil {
		return Plan{}, err
	}
	p.Members, p.Pending = members[id], pending[id]

	links, err := q.ListMealPlanEntryLists(ctx, sqlc.ListMealPlanEntryListsParams{PlanID: id, UserID: userID})
	if err != nil {
		return Plan{}, fmt.Errorf("list meal plan entry lists: %w", err)
	}
	byEntry := map[string][]LinkedList{}
	for _, l := range links {
		if l.PlanEntryID != nil {
			byEntry[*l.PlanEntryID] = append(byEntry[*l.PlanEntryID], LinkedList{ID: l.ListID, Name: l.ListName, OpenCount: int(l.OpenCount)})
		}
	}

	rows, err := q.ListMealPlanEntries(ctx, sqlc.ListMealPlanEntriesParams{PlanID: id, FromDay: &from, ToDay: &to})
	if err != nil {
		return Plan{}, fmt.Errorf("list meal plan entries: %w", err)
	}
	type tally struct {
		list    LinkedList
		entries int
	}
	var lists []*tally
	seen := map[string]*tally{}
	for _, r := range rows {
		e := PlanEntry{
			ID: r.ID, Day: r.Day, Position: int(r.Position), RecipeID: r.RecipeID, RecipeSlug: r.RecipeSlug,
			Title: r.Title, Photo: r.CoverImageID, Lists: byEntry[r.ID],
		}
		if e.Lists == nil {
			e.Lists = []LinkedList{}
		}
		if r.Servings != nil {
			n := int(*r.Servings)
			e.Servings = &n
		}
		if r.CreatedBy != nil && r.CreatorName != nil {
			c := person(*r.CreatedBy, *r.CreatorName, *r.CreatorColor, r.CreatorAvatarID)
			e.CreatedBy = &c
		}
		if e.RecipeID != nil && len(e.Lists) == 0 {
			p.Unlisted++
		}
		for _, l := range e.Lists {
			if seen[l.ID] == nil {
				seen[l.ID] = &tally{list: l}
				lists = append(lists, seen[l.ID])
			}
			seen[l.ID].entries++
		}
		p.Entries = append(p.Entries, e)
	}
	slices.SortStableFunc(lists, func(a, b *tally) int { return cmp.Compare(b.entries, a.entries) })
	for _, l := range lists {
		p.Lists = append(p.Lists, l.list)
	}
	return p, nil
}

// planMembers returns the accepted members of the plans by plan id, and the
// pending invitees when withPending is set. Absent plans map to empty slices.
func planMembers(ctx context.Context, q *sqlc.Queries, planIDs []string, withPending bool) (members, pending map[string][]Person, err error) {
	members, pending = map[string][]Person{}, map[string][]Person{}
	for _, id := range planIDs {
		members[id], pending[id] = []Person{}, []Person{}
	}
	if len(planIDs) == 0 {
		return members, pending, nil
	}
	rows, err := q.ListMealPlanMembers(ctx, planIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("list meal plan members: %w", err)
	}
	for _, r := range rows {
		p := person(r.UserID, r.DisplayName, r.Color, r.AvatarID)
		if r.AcceptedAt != nil {
			members[r.PlanID] = append(members[r.PlanID], p)
		} else if withPending {
			pending[r.PlanID] = append(pending[r.PlanID], p)
		}
	}
	return members, pending, nil
}

// mutate runs fn in a transaction after touching the plan (which proves the
// caller has access), then returns the plan as it is afterwards and, once
// committed, tells the hub.
func (s *Service) mutate(ctx context.Context, userID, planID, from, to string, fn func(q *sqlc.Queries) error) (Plan, error) {
	if err := checkRange(from, to); err != nil {
		return Plan{}, err
	}
	var p Plan
	err := db.Tx(ctx, s.conn, func(q *sqlc.Queries) error {
		n, err := q.TouchMealPlan(ctx, sqlc.TouchMealPlanParams{UpdatedAt: s.stamp(), ID: planID, UserID: &userID})
		if err != nil {
			return fmt.Errorf("touch meal plan: %w", err)
		}
		if n == 0 {
			return ErrNotFound
		}
		if err := fn(q); err != nil {
			return err
		}
		p, err = get(ctx, q, userID, planID, from, to)
		return err
	})
	if err != nil {
		return Plan{}, err
	}
	s.hub.Publish(planID)
	return p, nil
}

// Add plans a recipe or a free text at the end of its day.
func (s *Service) Add(ctx context.Context, userID, planID, from, to string, in AddEntry) (Plan, error) {
	if (in.RecipeID == nil) == (in.Title == nil) {
		return Plan{}, invalid("give either a recipe or a title")
	}
	if err := checkDay(in.Day); err != nil {
		return Plan{}, err
	}
	var title string
	if in.Title != nil {
		if in.Servings != nil {
			return Plan{}, invalid("a free text has no servings")
		}
		var err error
		if title, err = clean("title", *in.Title, 120); err != nil {
			return Plan{}, err
		}
	} else if in.Servings != nil {
		if err := checkServings(*in.Servings); err != nil {
			return Plan{}, err
		}
	}
	return s.mutate(ctx, userID, planID, from, to, func(q *sqlc.Queries) error {
		var servings *int64
		if in.RecipeID != nil {
			rec, err := q.GetRecipe(ctx, *in.RecipeID)
			if errors.Is(err, sql.ErrNoRows) {
				return ErrRecipeNotFound
			}
			if err != nil {
				return fmt.Errorf("get recipe: %w", err)
			}
			title, servings = rec.Title, &rec.Servings
			if in.Servings != nil {
				n := int64(*in.Servings)
				servings = &n
			}
		}
		pos, err := q.NextMealPlanEntryPosition(ctx, sqlc.NextMealPlanEntryPositionParams{PlanID: planID, Day: in.Day})
		if err != nil {
			return fmt.Errorf("next meal plan entry position: %w", err)
		}
		if err := q.InsertMealPlanEntry(ctx, sqlc.InsertMealPlanEntryParams{
			ID: newID(), PlanID: planID, Day: in.Day, Position: pos, RecipeID: in.RecipeID,
			Title: title, Servings: servings, CreatedBy: &userID, CreatedAt: s.stamp(),
		}); err != nil {
			return fmt.Errorf("insert meal plan entry: %w", err)
		}
		return nil
	})
}

// Move puts an entry on day (nil: the tray) at position, clamped to the end
// of that day, and renumbers the day it leaves and the day it enters. The
// lists it is on show its day, so they are bumped and told too.
func (s *Service) Move(ctx context.Context, userID, planID, entryID, from, to string, day *string, position int) (Plan, error) {
	if err := checkDay(day); err != nil {
		return Plan{}, err
	}
	if position < 0 {
		return Plan{}, invalid("position must not be negative")
	}
	var lists []string
	p, err := s.mutate(ctx, userID, planID, from, to, func(q *sqlc.Queries) error {
		e, err := liveEntry(ctx, q, planID, entryID)
		if err != nil {
			return err
		}
		if !sameDay(e.Day, day) {
			if err := insertAt(ctx, q, planID, e.Day, entryID, -1); err != nil {
				return err
			}
		}
		if err := insertAt(ctx, q, planID, day, entryID, position); err != nil {
			return err
		}
		lists, err = shopping.FollowPlan(ctx, q, []string{entryID}, nil)
		return err
	})
	return s.followed(p, err, lists)
}

func sameDay(a, b *string) bool { return a == nil && b == nil || a != nil && b != nil && *a == *b }

// insertAt renumbers the live entries of day to 0..n-1 with entryID at
// position (clamped to the end), or left out when position is negative.
func insertAt(ctx context.Context, q *sqlc.Queries, planID string, day *string, entryID string, position int) error {
	ids, err := q.ListMealPlanDayEntryIDs(ctx, sqlc.ListMealPlanDayEntryIDsParams{PlanID: planID, Day: day})
	if err != nil {
		return fmt.Errorf("list meal plan day entries: %w", err)
	}
	ids = slices.DeleteFunc(ids, func(id string) bool { return id == entryID })
	if position >= 0 {
		ids = slices.Insert(ids, min(position, len(ids)), entryID)
	}
	for i, id := range ids {
		if err := q.PlaceMealPlanEntry(ctx, sqlc.PlaceMealPlanEntryParams{Day: day, Position: int64(i), ID: id, PlanID: planID}); err != nil {
			return fmt.Errorf("place meal plan entry: %w", err)
		}
	}
	return nil
}

// SetServings changes a recipe entry's servings; a free text has none and
// reports ErrNotFound.
func (s *Service) SetServings(ctx context.Context, userID, planID, entryID, from, to string, servings int) (Plan, error) {
	if err := checkServings(servings); err != nil {
		return Plan{}, err
	}
	var lists []string
	p, err := s.mutate(ctx, userID, planID, from, to, func(q *sqlc.Queries) error {
		n := int64(servings)
		rows, err := q.SetMealPlanEntryServings(ctx, sqlc.SetMealPlanEntryServingsParams{Servings: &n, ID: entryID, PlanID: planID})
		if err != nil {
			return fmt.Errorf("set meal plan entry servings: %w", err)
		}
		if rows == 0 {
			return ErrNotFound
		}
		lists, err = shopping.FollowPlan(ctx, q, []string{entryID}, &servings)
		return err
	})
	return s.followed(p, err, lists)
}

// followed tells the open streams of the lists a committed plan change
// reached; err is the change's, and nothing is told when it failed.
func (s *Service) followed(p Plan, err error, lists []string) (Plan, error) {
	if err != nil {
		return Plan{}, err
	}
	s.lists.Hub().Publish(lists...)
	return p, nil
}

// Remove takes an entry off the plan; it keeps its position and stays
// restorable for purgeAfter, and every Remove purges the plan's older
// removals. Its blocks on lists hide until Restore or the purge drops them.
func (s *Service) Remove(ctx context.Context, userID, planID, entryID, from, to string) (Plan, error) {
	var lists []string
	p, err := s.mutate(ctx, userID, planID, from, to, func(q *sqlc.Queries) error {
		now := s.now()
		stamp, cutoff := db.FormatTime(now), db.FormatTime(now.Add(-purgeAfter))
		e, err := liveEntry(ctx, q, planID, entryID)
		if err != nil {
			return err
		}
		if _, err := q.RemoveMealPlanEntry(ctx, sqlc.RemoveMealPlanEntryParams{RemovedAt: &stamp, ID: entryID, PlanID: planID}); err != nil {
			return fmt.Errorf("remove meal plan entry: %w", err)
		}
		if err := insertAt(ctx, q, planID, e.Day, entryID, -1); err != nil {
			return err
		}
		purged, err := q.ListRemovedMealPlanEntryIDs(ctx, sqlc.ListRemovedMealPlanEntryIDsParams{PlanID: planID, Cutoff: &cutoff})
		if err != nil {
			return fmt.Errorf("list removed meal plan entries: %w", err)
		}
		if lists, err = shopping.FollowPlan(ctx, q, append(purged, entryID), nil); err != nil {
			return err
		}
		if err := q.PurgeRemovedMealPlanEntries(ctx, sqlc.PurgeRemovedMealPlanEntriesParams{PlanID: planID, Cutoff: &cutoff}); err != nil {
			return fmt.Errorf("purge removed meal plan entries: %w", err)
		}
		return nil
	})
	return s.followed(p, err, lists)
}

// Restore brings a removed entry back at the position it had, and its blocks
// on lists with their ticks.
func (s *Service) Restore(ctx context.Context, userID, planID, entryID, from, to string) (Plan, error) {
	var lists []string
	p, err := s.mutate(ctx, userID, planID, from, to, func(q *sqlc.Queries) error {
		e, err := q.GetMealPlanEntry(ctx, sqlc.GetMealPlanEntryParams{ID: entryID, PlanID: planID})
		if errors.Is(err, sql.ErrNoRows) || err == nil && e.RemovedAt == nil {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("get meal plan entry: %w", err)
		}
		if _, err := q.RestoreMealPlanEntry(ctx, sqlc.RestoreMealPlanEntryParams{ID: entryID, PlanID: planID}); err != nil {
			return fmt.Errorf("restore meal plan entry: %w", err)
		}
		if lists, err = shopping.FollowPlan(ctx, q, []string{entryID}, nil); err != nil {
			return err
		}
		return insertAt(ctx, q, planID, e.Day, entryID, int(e.Position))
	})
	return s.followed(p, err, lists)
}

// liveEntry loads an entry of the plan that is not removed.
func liveEntry(ctx context.Context, q *sqlc.Queries, planID, entryID string) (sqlc.GetMealPlanEntryRow, error) {
	e, err := q.GetMealPlanEntry(ctx, sqlc.GetMealPlanEntryParams{ID: entryID, PlanID: planID})
	if errors.Is(err, sql.ErrNoRows) || err == nil && e.RemovedAt != nil {
		return e, ErrNotFound
	}
	if err != nil {
		return e, fmt.Errorf("get meal plan entry: %w", err)
	}
	return e, nil
}

// changed bumps the revision of a plan whose change did not touch its row
// (members), then tells its open streams.
func (s *Service) changed(ctx context.Context, planID string) error {
	if err := s.q.BumpMealPlanRevision(ctx, planID); err != nil {
		return fmt.Errorf("bump meal plan revision: %w", err)
	}
	s.hub.Publish(planID)
	return nil
}

// thisWeek returns the plan as it is for the current week.
func (s *Service) thisWeek(ctx context.Context, userID, planID string) (Plan, error) {
	from, to := week(s.now())
	return s.Get(ctx, userID, planID, from, to)
}

// ownPlan reports ErrNotFound unless the caller created the plan, and
// ErrPersonal when it is their personal one, which is never shared.
func (s *Service) ownPlan(ctx context.Context, userID, planID string) error {
	row, err := s.q.GetMealPlanCreator(ctx, planID)
	if errors.Is(err, sql.ErrNoRows) || err == nil && row.CreatedBy != userID {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("get meal plan creator: %w", err)
	}
	if row.Personal == 1 {
		return personal()
	}
	return nil
}

// Invite asks a person to share a plan; only the creator can. An existing
// row, pending or accepted, stays as it is and nobody is notified again.
func (s *Service) Invite(ctx context.Context, userID, planID, inviteeID string) (Plan, error) {
	if err := s.ownPlan(ctx, userID, planID); err != nil {
		return Plan{}, err
	}
	if inviteeID == userID {
		return Plan{}, invalid("you cannot invite yourself")
	}
	if _, err := s.q.GetUserByID(ctx, inviteeID); errors.Is(err, sql.ErrNoRows) {
		return Plan{}, invalid("no such person")
	} else if err != nil {
		return Plan{}, fmt.Errorf("get invitee: %w", err)
	}
	n, err := s.q.InsertMealPlanMember(ctx, sqlc.InsertMealPlanMemberParams{PlanID: planID, UserID: inviteeID, InvitedAt: s.stamp()})
	if err != nil {
		return Plan{}, fmt.Errorf("insert meal plan member: %w", err)
	}
	if n > 0 {
		if err := s.changed(ctx, planID); err != nil {
			return Plan{}, err
		}
		if s.inv != nil {
			s.inv.PlanInvited(ctx, planID, userID, inviteeID)
		}
	}
	return s.thisWeek(ctx, userID, planID)
}

// Accept turns the caller's pending invitation into a membership.
func (s *Service) Accept(ctx context.Context, userID, planID string) (Plan, error) {
	now := s.stamp()
	n, err := s.q.AcceptMealPlanMembership(ctx, sqlc.AcceptMealPlanMembershipParams{AcceptedAt: &now, PlanID: planID, UserID: userID})
	if err != nil {
		return Plan{}, fmt.Errorf("accept meal plan invitation: %w", err)
	}
	if n == 0 {
		return Plan{}, ErrNotFound
	}
	if err := s.changed(ctx, planID); err != nil {
		return Plan{}, err
	}
	return s.thisWeek(ctx, userID, planID)
}

// Leave ends the caller's membership whatever its state, so it also declines
// an invitation. The creator cannot leave their own plan.
func (s *Service) Leave(ctx context.Context, userID, planID string) error {
	row, err := s.q.GetMealPlanCreator(ctx, planID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("get meal plan creator: %w", err)
	}
	if err == nil && row.CreatedBy == userID {
		return invalid("the creator cannot leave")
	}
	n, err := s.q.DeleteMealPlanMembership(ctx, sqlc.DeleteMealPlanMembershipParams{PlanID: planID, UserID: userID})
	if err != nil {
		return fmt.Errorf("delete meal plan membership: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return s.changed(ctx, planID)
}

// RemoveMember lets the creator withdraw an invitation or remove a member.
func (s *Service) RemoveMember(ctx context.Context, userID, planID, memberID string) (Plan, error) {
	n, err := s.q.DeleteMealPlanMember(ctx, sqlc.DeleteMealPlanMemberParams{PlanID: planID, MemberID: memberID, UserID: userID})
	if err != nil {
		return Plan{}, fmt.Errorf("delete meal plan member: %w", err)
	}
	if n == 0 {
		return Plan{}, ErrNotFound
	}
	if err := s.changed(ctx, planID); err != nil {
		return Plan{}, err
	}
	return s.thisWeek(ctx, userID, planID)
}
