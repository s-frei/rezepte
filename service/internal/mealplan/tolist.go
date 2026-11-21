package mealplan

import (
	"context"
	"fmt"

	"github.com/s-frei/rezepte/service/internal/db/sqlc"
	"github.com/s-frei/rezepte/service/internal/ingredient"
	"github.com/s-frei/rezepte/service/internal/shopping"
)

// ToListPick is a recipe entry to put on a list with the ingredients to
// include.
type ToListPick struct {
	EntryID string                    `json:"entryId"`
	Include []shopping.IngredientPick `json:"include"`
}

// ToList is what ToList puts where: exactly one of ListID (an existing list
// the caller can access) and NewListName (a new list of the caller's).
type ToList struct {
	ListID      *string      `json:"listId,omitempty"`
	NewListName *string      `json:"newListName,omitempty"`
	Picks       []ToListPick `json:"picks"`
	// AlsoListed skips an entry only when it is on this list already, so
	// one entry can go on a second list (its ⋯ "Auf eine Liste…"); without
	// it an entry on any list the caller can access is skipped, as bulk
	// callers and MCP expect.
	AlsoListed bool `json:"alsoListed,omitempty"`
}

// ToList puts recipe entries of a plan on a list at their planned servings,
// linked so later plan changes follow. An entry already on a list the caller
// can access is skipped (with AlsoListed, only one already on this list);
// one only on lists they cannot access is added, so two people shopping
// separately each get it. Returns the plan for from..to and the list's id.
func (s *Service) ToList(ctx context.Context, userID, planID, from, to string, in ToList) (Plan, string, error) {
	if (in.ListID == nil) == (in.NewListName == nil) {
		return Plan{}, "", invalid("give either a list or a new list name")
	}
	if len(in.Picks) == 0 {
		return Plan{}, "", invalid("pick at least one entry")
	}
	var name string
	if in.NewListName != nil {
		var err error
		if name, err = shopping.Clean("name", *in.NewListName, 80); err != nil {
			return Plan{}, "", err
		}
	}
	var listID string
	p, err := s.mutate(ctx, userID, planID, from, to, func(q *sqlc.Queries) error {
		if in.ListID != nil {
			listID = *in.ListID
		} else {
			listID = newID()
			now := s.stamp()
			if err := q.InsertShoppingList(ctx, sqlc.InsertShoppingListParams{ID: listID, Name: name, CreatedBy: userID, CreatedAt: now, UpdatedAt: now, UpdatedBy: &userID}); err != nil {
				return fmt.Errorf("insert shopping list: %w", err)
			}
		}
		links, err := q.ListMealPlanEntryLists(ctx, sqlc.ListMealPlanEntryListsParams{PlanID: planID, UserID: userID})
		if err != nil {
			return fmt.Errorf("list meal plan entry lists: %w", err)
		}
		skip := map[string]bool{}
		for _, l := range links {
			if l.PlanEntryID != nil && (!in.AlsoListed || l.ListID == listID) {
				skip[*l.PlanEntryID] = true
			}
		}
		var adds []shopping.PlannedRecipe
		for _, pick := range in.Picks {
			e, err := liveEntry(ctx, q, planID, pick.EntryID)
			if err != nil {
				return err
			}
			if e.RecipeID == nil || e.Servings == nil {
				return invalid("only a recipe goes on a list")
			}
			if skip[e.ID] {
				continue
			}
			skip[e.ID] = true
			adds = append(adds, shopping.PlannedRecipe{PlanEntryID: e.ID, RecipeID: *e.RecipeID, Servings: int(*e.Servings), Include: pick.Include})
		}
		return s.lists.AddPlanned(ctx, q, userID, listID, adds)
	})
	if err != nil {
		return Plan{}, "", err
	}
	s.lists.Hub().Publish(listID)
	return p, listID, nil
}

// PreviewEntry is a recipe entry as the "Auf die Liste" dialog offers it.
// AlreadyListed is whether it is on a list the caller can access; Day is nil
// on the tray.
type PreviewEntry struct {
	EntryID       string              `json:"entryId"`
	Day           *string             `json:"day" nullable:"true" doc:"The planned day (YYYY-MM-DD); null on the tray"`
	Title         string              `json:"title"`
	Servings      int                 `json:"servings"`
	AlreadyListed bool                `json:"alreadyListed"`
	Ingredients   []PreviewIngredient `json:"ingredients"`
}

// PreviewIngredient is one recipe ingredient, by group and position, its
// quantity scaled to the planned servings; Staple is whether it is a staple
// of the plan.
type PreviewIngredient struct {
	Group    int      `json:"group"`
	Index    int      `json:"index"`
	Name     string   `json:"name"`
	Quantity *float64 `json:"quantity,omitempty"`
	Unit     *string  `json:"unit,omitempty"`
	Staple   bool     `json:"staple"`
}

// Preview lists the recipe entries from..to (and the tray when tray is set)
// with what ToList would put on a list for each.
func (s *Service) Preview(ctx context.Context, userID, planID, from, to string, tray bool) ([]PreviewEntry, error) {
	p, err := s.Get(ctx, userID, planID, from, to)
	if err != nil {
		return nil, err
	}
	staples, err := s.staples(ctx, planID)
	if err != nil {
		return nil, err
	}
	isStaple := map[string]bool{}
	for _, st := range staples {
		isStaple[st.NameKey] = true
	}
	out := []PreviewEntry{}
	for _, e := range p.Entries {
		if e.RecipeID == nil || e.Servings == nil || e.Day == nil && !tray {
			continue
		}
		rec, err := s.q.GetRecipe(ctx, *e.RecipeID)
		if err != nil {
			return nil, fmt.Errorf("get recipe: %w", err)
		}
		groups, err := s.q.ListIngredientGroupsByRecipe(ctx, *e.RecipeID)
		if err != nil {
			return nil, fmt.Errorf("list ingredient groups: %w", err)
		}
		ings, err := s.q.ListIngredientsByRecipe(ctx, *e.RecipeID)
		if err != nil {
			return nil, fmt.Errorf("list ingredients: %w", err)
		}
		pe := PreviewEntry{EntryID: e.ID, Day: e.Day, Title: e.Title, Servings: *e.Servings, AlreadyListed: len(e.Lists) > 0, Ingredients: []PreviewIngredient{}}
		for g, grp := range groups {
			i := 0
			for _, ing := range ings {
				if ing.GroupID != grp.ID {
					continue
				}
				pi := PreviewIngredient{Group: g, Index: i, Name: ing.Name, Unit: ing.Unit, Staple: isStaple[ingredient.Key(ing.Name)]}
				if ing.Quantity != nil {
					v := shopping.RoundScaled(shopping.ScaleQuantity(*ing.Quantity, float64(rec.Servings), float64(*e.Servings)))
					pi.Quantity = &v
				}
				pe.Ingredients = append(pe.Ingredients, pi)
				i++
			}
		}
		out = append(out, pe)
	}
	return out, nil
}

// PreviewOne is Preview for one recipe entry, whatever its day, the tray
// included; a free text is ErrInvalid.
func (s *Service) PreviewOne(ctx context.Context, userID, planID, entryID string) (PreviewEntry, error) {
	e, err := liveEntry(ctx, s.q, planID, entryID)
	if err != nil {
		return PreviewEntry{}, err
	}
	from, to := week(s.now())
	if e.Day != nil {
		from, to = *e.Day, *e.Day
	}
	all, err := s.Preview(ctx, userID, planID, from, to, e.Day == nil)
	if err != nil {
		return PreviewEntry{}, err
	}
	for _, p := range all {
		if p.EntryID == entryID {
			return p, nil
		}
	}
	return PreviewEntry{}, invalid("only a recipe goes on a list")
}
