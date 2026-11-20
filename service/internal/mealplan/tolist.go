package mealplan

import (
	"context"
	"fmt"

	"github.com/s-frei/rezepte/service/internal/db/sqlc"
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
}

// ToList puts recipe entries of a plan on a list at their planned servings,
// linked so later plan changes follow. An entry already on a list the caller
// can access is skipped; one only on lists they cannot access is added, so
// two people shopping separately each get it. Returns the plan for from..to
// and the list's id.
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
		if name, err = clean("name", *in.NewListName, 80); err != nil {
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
			if l.PlanEntryID != nil {
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
