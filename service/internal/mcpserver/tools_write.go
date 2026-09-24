package mcpserver

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/recipe"
)

func init() {
	tools = append(tools,
		tool{auth.ScopeRecipesWrite, addCreateRecipe},
		tool{auth.ScopeRecipesWrite, addUpdateRecipe},
		tool{auth.ScopeRecipesDelete, addDeleteRecipe},
		tool{auth.ScopeRecipesWrite, addFavoriteTool(true)},
		tool{auth.ScopeRecipesWrite, addFavoriteTool(false)},
	)
}

// documentHelp is shared by create_recipe and update_recipe: which fields
// take null, and how a step points at an ingredient - the parts of the
// document a model cannot guess from field names.
const documentHelp = " Send every field of recipe. Only prepMinutes, cookMinutes, sourceUrl, a group's name and an " +
	"ingredient's quantity, unit and note take null when unset; description is a string (\"\" when empty) and " +
	"tags, steps and each group's ingredients are arrays ([] when empty), never null; ingredientGroups is never " +
	"null either and holds at least one group (one with name null when the recipe has no sections)." +
	" Every step lists its ingredient references; send [] when it has none. " +
	"A reference ties a word of the step's text to one ingredient: word must occur in the text as a whole word, " +
	"ingredientName is the ingredient's name and groupName its group's name (null for the unnamed group). " +
	"Ambiguous targets are refused, never guessed. " +
	"editPolicy is optional: default follows the household setting, open lets every member edit, locked keeps " +
	"the recipe to its author and admins; left out on update, the stored policy stays."

type createIn struct {
	Recipe recipe.Input `json:"recipe"`
}

func addCreateRecipe(s *mcp.Server, c caller) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "create_recipe",
		Description: "Create a recipe as the token's owner and return it." + documentHelp,
		InputSchema: c.create,
		Annotations: &mcp.ToolAnnotations{DestructiveHint: new(false), OpenWorldHint: new(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in createIn) (*mcp.CallToolResult, any, error) {
		r, err := c.svc.Create(ctx, c.user.ID, in.Recipe)
		if err != nil {
			return nil, nil, toolError(ctx, fmt.Errorf("create recipe: %w", err))
		}
		if err := c.fill(ctx, &r); err != nil {
			return nil, nil, toolError(ctx, err)
		}
		return nil, r, nil
	})
}

type updateIn struct {
	ID     string       `json:"id"`
	Recipe recipe.Input `json:"recipe"`
}

func addUpdateRecipe(s *mcp.Server, c caller) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "update_recipe",
		Description: "Replace a recipe's whole document and return it. Take the document from get_recipe and " +
			"send back only the editable fields in recipe: title, description, servings, prepMinutes, cookMinutes, " +
			"sourceUrl, tags, ingredientGroups, steps and optionally editPolicy. Anything else left out is removed. " +
			"The slug never changes." + documentHelp,
		InputSchema: c.update,
		// DestructiveHint stays at its default (true): replacing the whole
		// document can remove content the caller did not mean to touch.
		Annotations: &mcp.ToolAnnotations{IdempotentHint: true, OpenWorldHint: new(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in updateIn) (*mcp.CallToolResult, any, error) {
		r, err := c.svc.Update(ctx, in.ID, c.user, in.Recipe)
		if err != nil {
			return nil, nil, toolError(ctx, fmt.Errorf("update recipe: %w", err))
		}
		if err := c.fill(ctx, &r); err != nil {
			return nil, nil, toolError(ctx, err)
		}
		return nil, r, nil
	})
}

type idIn struct {
	ID string `json:"id" jsonschema:"The recipe's id"`
}

func addDeleteRecipe(s *mcp.Server, c caller) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "delete_recipe",
		Description: "Delete a recipe with its photos. This cannot be undone.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: new(true), IdempotentHint: true, OpenWorldHint: new(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in idIn) (*mcp.CallToolResult, any, error) {
		if err := c.svc.Delete(ctx, in.ID, c.user); err != nil {
			return nil, nil, toolError(ctx, fmt.Errorf("delete recipe: %w", err))
		}
		return nil, map[string]any{"deleted": in.ID}, nil
	})
}

// addFavoriteTool builds add_favorite (on) or remove_favorite (off). Adding
// to a missing recipe is "recipe not found"; removing is idempotent, as in
// REST, so removing a star that is not there succeeds.
func addFavoriteTool(on bool) func(*mcp.Server, caller) {
	name, desc := "remove_favorite", "Remove a recipe from the token owner's favorites."
	if on {
		name, desc = "add_favorite", "Add a recipe to the token owner's favorites."
	}
	return func(s *mcp.Server, c caller) {
		mcp.AddTool(s, &mcp.Tool{
			Name:        name,
			Description: desc,
			Annotations: &mcp.ToolAnnotations{DestructiveHint: new(false), IdempotentHint: true, OpenWorldHint: new(false)},
		}, func(ctx context.Context, _ *mcp.CallToolRequest, in idIn) (*mcp.CallToolResult, any, error) {
			if err := c.svc.SetFavorite(ctx, c.user.ID, in.ID, on); err != nil {
				return nil, nil, toolError(ctx, fmt.Errorf("%s: %w", name, err))
			}
			return nil, map[string]any{"id": in.ID, "favorite": on}, nil
		})
	}
}
