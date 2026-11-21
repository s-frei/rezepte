package mcpserver

import (
	"context"
	"errors"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/mealplan"
	"github.com/s-frei/rezepte/service/internal/shopping"
)

func init() {
	tools = append(tools,
		tool{auth.ScopePlanRead, addListMealPlans},
		tool{auth.ScopePlanRead, addGetMealPlan},
		tool{auth.ScopePlanWrite, addAddToMealPlan},
		tool{auth.ScopePlanWrite, addUpdateMealPlanEntry},
		tool{auth.ScopePlanWrite, addRemoveMealPlanEntry},
		tool{auth.ScopePlanWrite, also(auth.ScopeListsWrite, addPutMealPlanOnList)},
	)
}

// rangeIn is the span of days a tool answers for; both left out means the
// current week.
type rangeIn struct {
	From string `json:"from,omitempty" jsonschema:"First day of the range as YYYY-MM-DD; with to, at most 62 days; defaults to Monday of this week"`
	To   string `json:"to,omitempty" jsonschema:"Last day of the range as YYYY-MM-DD, included; defaults to Sunday of this week"`
}

type planIn struct {
	PlanID string `json:"planId,omitempty" jsonschema:"The plan's id, from list_meal_plans; defaults to the token owner's personal plan"`
	rangeIn
}

type entryIn struct {
	PlanID  string `json:"planId" jsonschema:"The plan's id, from list_meal_plans"`
	EntryID string `json:"entryId" jsonschema:"The entry's id, from get_meal_plan"`
	rangeIn
}

// planError is toolError for the meal plan service's refusals.
func planError(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, mealplan.ErrNotFound):
		return errors.New("meal plan not found")
	case errors.Is(err, shopping.ErrNotFound):
		return errors.New("shopping list not found")
	case errors.Is(err, mealplan.ErrInvalid), errors.Is(err, mealplan.ErrRecipeNotFound):
		return err
	}
	return toolError(ctx, err)
}

// span is the range asked for, the current week when both ends are left out.
func (c caller) span(r rangeIn) (string, string, error) {
	if r.From == "" && r.To == "" {
		from, to := c.plans.Week()
		return from, to, nil
	}
	if r.From == "" || r.To == "" {
		return "", "", errors.New("pass both from and to, or neither")
	}
	return r.From, r.To, nil
}

// planID is id, or the token owner's personal plan when empty.
func (c caller) planID(ctx context.Context, id string) (string, error) {
	if id != "" {
		return id, nil
	}
	plans, _, err := c.plans.Plans(ctx, c.user.ID)
	if err != nil {
		return "", planError(ctx, err)
	}
	for _, p := range plans {
		if p.Personal {
			return p.ID, nil
		}
	}
	return "", errors.New("meal plan not found")
}

func addListMealPlans(s *mcp.Server, c caller) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_meal_plans",
		Description: "List the token owner's meal plans (the personal one and shared ones), with their members, plus invitations to plans of others that are still waiting (accepting is app-only).",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: new(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		plans, inv, err := c.plans.Plans(ctx, c.user.ID)
		if err != nil {
			return nil, nil, planError(ctx, err)
		}
		return nil, map[string]any{"plans": plans, "invitations": inv}, nil
	})
}

func addGetMealPlan(s *mcp.Server, c caller) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_meal_plan",
		Description: "Read a meal plan for a range of days (default: this week, Monday to Sunday): its entries by day and position, and the tray of entries with no day. Each entry is a recipe (recipeId, slug, servings) or a free text, with the lists it is on. Without planId the token owner's personal plan is read.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: new(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in planIn) (*mcp.CallToolResult, any, error) {
		from, to, err := c.span(in.rangeIn)
		if err != nil {
			return nil, nil, err
		}
		id, err := c.planID(ctx, in.PlanID)
		if err != nil {
			return nil, nil, err
		}
		p, err := c.plans.Get(ctx, c.user.ID, id, from, to)
		if err != nil {
			return nil, nil, planError(ctx, err)
		}
		return nil, p, nil
	})
}

type addToPlanIn struct {
	planIn
	Day      string `json:"day,omitempty" jsonschema:"The day as YYYY-MM-DD; without it the entry goes on the tray"`
	RecipeID string `json:"recipeId,omitempty" jsonschema:"The recipe's id"`
	Slug     string `json:"slug,omitempty" jsonschema:"The recipe's slug, as in its URL"`
	Title    string `json:"title,omitempty" jsonschema:"A free text instead of a recipe, e.g. Leftovers, up to 120 characters"`
	Servings *int   `json:"servings,omitempty" jsonschema:"Servings, 1 to 99; defaults to the recipe's own; not for a free text"`
}

func addAddToMealPlan(s *mcp.Server, c caller) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "add_to_meal_plan",
		Description: "Plan a recipe or a free text at the end of a day, or on the tray without a day. Pass exactly one of recipeId, slug or title. Without planId the token owner's personal plan is used. Returns the plan for the range (default: this week).",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: new(false), OpenWorldHint: new(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in addToPlanIn) (*mcp.CallToolResult, any, error) {
		from, to, err := c.span(in.rangeIn)
		if err != nil {
			return nil, nil, err
		}
		id, err := c.planID(ctx, in.PlanID)
		if err != nil {
			return nil, nil, err
		}
		add := mealplan.AddEntry{Servings: in.Servings}
		if in.Day != "" {
			add.Day = &in.Day
		}
		if in.Title != "" {
			if in.RecipeID != "" || in.Slug != "" {
				return nil, nil, errors.New("pass exactly one of recipeId, slug or title")
			}
			add.Title = &in.Title
		} else {
			r, err := c.recipe(ctx, "recipeId", in.RecipeID, in.Slug)
			if err != nil {
				return nil, nil, err
			}
			add.RecipeID = &r.ID
		}
		p, err := c.plans.Add(ctx, c.user.ID, id, from, to, add)
		if err != nil {
			return nil, nil, planError(ctx, err)
		}
		return nil, p, nil
	})
}

func addUpdateMealPlanEntry(s *mcp.Server, c caller) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "update_meal_plan_entry",
		Description: "Move an entry to another day (it goes to the end of that day), to the tray, or change its servings. Pass day or tray, and servings, as needed; lists the entry is on follow. Returns the plan for the range (default: this week).",
		Annotations: &mcp.ToolAnnotations{IdempotentHint: true, DestructiveHint: new(false), OpenWorldHint: new(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in struct {
		entryIn
		Day      string `json:"day,omitempty" jsonschema:"Move to this day, as YYYY-MM-DD"`
		Tray     bool   `json:"tray,omitempty" jsonschema:"true to move to the tray, the entries with no day"`
		Servings *int   `json:"servings,omitempty" jsonschema:"New servings, 1 to 99; recipes only"`
	}) (*mcp.CallToolResult, any, error) {
		from, to, err := c.span(in.rangeIn)
		if err != nil {
			return nil, nil, err
		}
		if in.Day != "" && in.Tray {
			return nil, nil, errors.New("pass day or tray, not both")
		}
		if in.Day == "" && !in.Tray && in.Servings == nil {
			return nil, nil, errors.New("pass day, tray or servings")
		}
		ch := mealplan.EntryChange{Moved: in.Day != "" || in.Tray, Servings: in.Servings}
		if in.Day != "" {
			ch.Day = &in.Day
		}
		p, err := c.plans.Update(ctx, c.user.ID, in.PlanID, in.EntryID, from, to, ch)
		if err != nil {
			return nil, nil, planError(ctx, err)
		}
		return nil, p, nil
	})
}

func addRemoveMealPlanEntry(s *mcp.Server, c caller) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "remove_meal_plan_entry",
		Description: "Remove an entry from a meal plan, and its recipe from the shopping lists it was put on. The app can undo this for 24 hours. Returns the plan for the range (default: this week).",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: new(true), OpenWorldHint: new(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in entryIn) (*mcp.CallToolResult, any, error) {
		from, to, err := c.span(in.rangeIn)
		if err != nil {
			return nil, nil, err
		}
		p, err := c.plans.Remove(ctx, c.user.ID, in.PlanID, in.EntryID, from, to)
		if err != nil {
			return nil, nil, planError(ctx, err)
		}
		return nil, p, nil
	})
}

type putOnListIn struct {
	PlanID      string `json:"planId" jsonschema:"The plan's id, from list_meal_plans"`
	From        string `json:"from" jsonschema:"First day to shop for, as YYYY-MM-DD"`
	To          string `json:"to" jsonschema:"Last day to shop for, as YYYY-MM-DD, included"`
	ListID      string `json:"listId,omitempty" jsonschema:"An existing shopping list's id, from list_shopping_lists"`
	NewListName string `json:"newListName,omitempty" jsonschema:"Name for a new list; pass this or listId"`
}

func addPutMealPlanOnList(s *mcp.Server, c caller) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "put_meal_plan_on_list",
		Description: "Put the recipes planned from..to on a shopping list at their planned servings, all ingredients except the plan's staples (what the household always has). Later plan changes follow into the list. Recipes already on one of the token owner's lists are skipped. Pass exactly one of listId or newListName. Returns the list's id and the plan.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: new(false), OpenWorldHint: new(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in putOnListIn) (*mcp.CallToolResult, any, error) {
		if (in.ListID == "") == (in.NewListName == "") {
			return nil, nil, errors.New("pass exactly one of listId or newListName")
		}
		prev, err := c.plans.Preview(ctx, c.user.ID, in.PlanID, in.From, in.To, false)
		if err != nil {
			return nil, nil, planError(ctx, err)
		}
		t := mealplan.ToList{}
		if in.ListID != "" {
			t.ListID = &in.ListID
		} else {
			t.NewListName = &in.NewListName
		}
		for _, e := range prev {
			pick := mealplan.ToListPick{EntryID: e.EntryID, Include: []shopping.IngredientPick{}}
			for _, ing := range e.Ingredients {
				if !ing.Staple {
					pick.Include = append(pick.Include, shopping.IngredientPick{Group: ing.Group, Index: ing.Index})
				}
			}
			// An empty include is refused, so a recipe of only staples is left out.
			if len(pick.Include) > 0 {
				t.Picks = append(t.Picks, pick)
			}
		}
		if len(t.Picks) == 0 {
			return nil, nil, errors.New("nothing to put on a list: no recipes in that range, or all their ingredients are staples")
		}
		p, listID, err := c.plans.ToList(ctx, c.user.ID, in.PlanID, in.From, in.To, t)
		if err != nil {
			return nil, nil, planError(ctx, err)
		}
		return nil, map[string]any{"listId": listID, "plan": p}, nil
	})
}
