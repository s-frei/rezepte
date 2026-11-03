package mcpserver

import (
	"context"
	"errors"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/shopping"
)

// maxShoppingLines is how many lines one add_shopping_items call takes.
const maxShoppingLines = 50

func init() {
	tools = append(tools,
		tool{auth.ScopeListsRead, addListShoppingLists},
		tool{auth.ScopeListsRead, addGetShoppingList},
		tool{auth.ScopeListsWrite, addCreateShoppingList},
		tool{auth.ScopeListsWrite, addRecipeToShoppingList},
		tool{auth.ScopeListsWrite, addShoppingItems},
		tool{auth.ScopeListsWrite, addCheckShoppingItems},
		tool{auth.ScopeListsWrite, addRemoveShoppingItems},
	)
}

type listIn struct {
	ListID string `json:"listId" jsonschema:"The shopping list's id, from list_shopping_lists"`
}

type itemIDsIn struct {
	listIn
	ItemIDs []string `json:"itemIds" jsonschema:"Item ids from get_shopping_list"`
}

type checkItemsIn struct {
	itemIDsIn
	Checked bool `json:"checked" jsonschema:"true to tick the items off, false to untick them"`
}

type addRecipeIn struct {
	listIn
	RecipeID    string                    `json:"recipeId,omitempty" jsonschema:"The recipe's id"`
	Slug        string                    `json:"slug,omitempty" jsonschema:"The recipe's slug, as in its URL"`
	Servings    *int                      `json:"servings,omitempty" jsonschema:"Servings to shop for, 1 to 99; defaults to the recipe's own"`
	Ingredients []shopping.IngredientPick `json:"ingredients,omitempty" jsonschema:"Which ingredients to add, by group and index as in get_recipe, both counted from 0; defaults to all of them"`
}

type addItemsIn struct {
	listIn
	Items []string `json:"items" jsonschema:"1 to 50 lines, each as typed in the app, e.g. 2 l Milch"`
}

// shoppingError is toolError for the shopping service's refusals.
func shoppingError(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, shopping.ErrNotFound):
		return errors.New("shopping list not found")
	case errors.Is(err, shopping.ErrInvalid), errors.Is(err, shopping.ErrRecipeNotFound):
		return err
	}
	return toolError(ctx, err)
}

func addListShoppingLists(s *mcp.Server, c caller) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_shopping_lists",
		Description: "List the token owner's shopping lists, most recently changed first, with their open and done row counts, plus invitations to lists of others that are still waiting (accepting is app-only).",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: new(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		lists, err := c.lists.Lists(ctx, c.user.ID)
		if err != nil {
			return nil, nil, shoppingError(ctx, err)
		}
		inv, err := c.lists.Invitations(ctx, c.user.ID)
		if err != nil {
			return nil, nil, shoppingError(ctx, err)
		}
		return nil, map[string]any{"lists": lists, "invitations": inv}, nil
	})
}

func addGetShoppingList(s *mcp.Server, c caller) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_shopping_list",
		Description: "Read one shopping list: its recipe entries and its items. Item amounts are already scaled to each entry's servings. Items with the same nameKey are one row in the app. Each item has a department and a checked flag; its id is what check_shopping_items and remove_shopping_items take.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: new(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in listIn) (*mcp.CallToolResult, any, error) {
		l, err := c.lists.Get(ctx, c.user.ID, in.ListID)
		if err != nil {
			return nil, nil, shoppingError(ctx, err)
		}
		return nil, l, nil
	})
}

func addCreateShoppingList(s *mcp.Server, c caller) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "create_shopping_list",
		Description: "Create an empty shopping list owned by the token owner. Sharing, renaming and deleting lists are app-only.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: new(false), OpenWorldHint: new(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in struct {
		Name string `json:"name" jsonschema:"The list's name, e.g. Weekend"`
	}) (*mcp.CallToolResult, any, error) {
		l, err := c.lists.Create(ctx, c.user.ID, in.Name)
		if err != nil {
			return nil, nil, shoppingError(ctx, err)
		}
		return nil, l, nil
	})
}

func addRecipeToShoppingList(s *mcp.Server, c caller) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "add_recipe_to_shopping_list",
		Description: "Copy a recipe's ingredients onto a shopping list as one entry. Pass exactly one of recipeId or slug. Without servings the recipe's own are used; without ingredients all are added. Amounts come back already scaled to the servings, and items with the same nameKey merge into one row in the app.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: new(false), OpenWorldHint: new(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in addRecipeIn) (*mcp.CallToolResult, any, error) {
		r, err := c.recipe(ctx, in.RecipeID, in.Slug)
		if err != nil {
			return nil, nil, err
		}
		servings := r.Servings
		if in.Servings != nil {
			servings = *in.Servings
		}
		picks := in.Ingredients
		if len(picks) == 0 {
			for g, group := range r.IngredientGroups {
				for n := range group.Ingredients {
					picks = append(picks, shopping.IngredientPick{Group: g, Index: n})
				}
			}
		}
		l, err := c.lists.AddRecipe(ctx, c.user.ID, in.ListID, r.ID, servings, picks)
		if err != nil {
			return nil, nil, shoppingError(ctx, err)
		}
		return nil, l, nil
	})
}

func addShoppingItems(s *mcp.Server, c caller) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "add_shopping_items",
		Description: "Add free items to a shopping list, one line each as typed in the app, e.g. \"2 l Milch\" or \"Brot\". 1 to 50 lines, checked up front; a line that fails midway leaves the earlier ones added, and the error says how many. Items with the same nameKey merge into one row in the app.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: new(false), OpenWorldHint: new(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in addItemsIn) (*mcp.CallToolResult, any, error) {
		if n := len(in.Items); n < 1 || n > maxShoppingLines {
			return nil, nil, fmt.Errorf("pass 1 to %d items, got %d; nothing was added", maxShoppingLines, n)
		}
		var l shopping.List
		for i, text := range in.Items {
			var err error
			if l, err = c.lists.AddFree(ctx, c.user.ID, in.ListID, text); err != nil {
				return nil, nil, fmt.Errorf("item %d: %w; %d of %d lines were added", i+1, shoppingError(ctx, err), i, len(in.Items))
			}
		}
		return nil, l, nil
	})
}

func addCheckShoppingItems(s *mcp.Server, c caller) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "check_shopping_items",
		Description: "Tick items off a shopping list, or untick them. Takes item ids from get_shopping_list; ids not on the list are ignored.",
		Annotations: &mcp.ToolAnnotations{IdempotentHint: true, DestructiveHint: new(false), OpenWorldHint: new(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in checkItemsIn) (*mcp.CallToolResult, any, error) {
		l, err := c.lists.SetChecked(ctx, c.user.ID, in.ListID, in.ItemIDs, in.Checked)
		if err != nil {
			return nil, nil, shoppingError(ctx, err)
		}
		return nil, l, nil
	})
}

func addRemoveShoppingItems(s *mcp.Server, c caller) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "remove_shopping_items",
		Description: "Delete items from a shopping list. Takes item ids from get_shopping_list; ids not on the list are ignored.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: new(true), IdempotentHint: true, OpenWorldHint: new(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in itemIDsIn) (*mcp.CallToolResult, any, error) {
		l, err := c.lists.RemoveItems(ctx, c.user.ID, in.ListID, in.ItemIDs)
		if err != nil {
			return nil, nil, shoppingError(ctx, err)
		}
		return nil, l, nil
	})
}
