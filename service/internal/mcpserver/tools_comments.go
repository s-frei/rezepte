package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/recipe"
)

func init() {
	tools = append(tools,
		tool{auth.ScopeRecipesRead, addListComments},
		tool{auth.ScopeRecipesWrite, addAddComment},
	)
}

type addCommentIn struct {
	getIn
	Body string `json:"body" jsonschema:"The comment, plain text, 1 to 2000 characters"`
}

type commentOut struct {
	Author    string     `json:"author"`
	Body      string     `json:"body"`
	CreatedAt time.Time  `json:"createdAt"`
	EditedAt  *time.Time `json:"editedAt,omitempty"`
	New       bool       `json:"new"`
}

func (c caller) recipeID(ctx context.Context, in getIn) (string, error) {
	if (in.ID == "") == (in.Slug == "") {
		return "", errors.New("pass exactly one of id or slug")
	}
	if in.ID != "" {
		return in.ID, nil
	}
	r, err := c.svc.BySlug(ctx, in.Slug)
	if err != nil {
		return "", toolError(ctx, fmt.Errorf("get recipe: %w", err))
	}
	return r.ID, nil
}

func toCommentOut(c recipe.Comment) commentOut {
	author := "former member"
	if c.Author != nil {
		author = c.Author.DisplayName
	}
	return commentOut{Author: author, Body: c.Body, CreatedAt: c.CreatedAt, EditedAt: c.EditedAt, New: c.New}
}

func addListComments(s *mcp.Server, c caller) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_comments",
		Description: "Read a recipe's comments: what household members wrote about cooking it, oldest first. new marks comments by others since the token owner last opened the recipe's comments in the app; reading here does not change that. Pass exactly one of id or slug.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: new(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in getIn) (*mcp.CallToolResult, any, error) {
		id, err := c.recipeID(ctx, in)
		if err != nil {
			return nil, nil, err
		}
		list, err := c.svc.Comments(ctx, c.user, id)
		if err != nil {
			return nil, nil, toolError(ctx, fmt.Errorf("list comments: %w", err))
		}
		out := make([]commentOut, 0, len(list))
		for _, e := range list {
			out = append(out, toCommentOut(e))
		}
		return nil, map[string]any{"comments": out}, nil
	})
}

func addAddComment(s *mcp.Server, c caller) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "add_comment",
		Description: "Write a comment on a recipe as the token owner, visible to the whole household. Plain text, 1 to 2000 characters. Pass exactly one of id or slug.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: new(false), IdempotentHint: false, OpenWorldHint: new(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in addCommentIn) (*mcp.CallToolResult, any, error) {
		id, err := c.recipeID(ctx, in.getIn)
		if err != nil {
			return nil, nil, err
		}
		e, err := c.svc.AddComment(ctx, c.user, id, in.Body)
		if err != nil {
			return nil, nil, toolError(ctx, fmt.Errorf("add comment: %w", err))
		}
		return nil, toCommentOut(e), nil
	})
}
