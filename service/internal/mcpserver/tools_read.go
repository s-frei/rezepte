package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/recipe"
)

func init() {
	tools = append(tools,
		tool{auth.ScopeRecipesRead, addSearchRecipes},
		tool{auth.ScopeRecipesRead, addGetRecipe},
		tool{auth.ScopeRecipesRead, addListTags},
	)
}

// Outputs are typed `any`: the SDK then sets no output schema, so a nil
// slice the service returns as JSON null (an empty tag list, an empty page)
// cannot fail an output-schema check the REST API never had.

type searchIn struct {
	Query      string   `json:"query,omitempty" jsonschema:"Full-text search over title, description, ingredient names and tags; prefix-matched, max 100 characters"`
	Tags       []string `json:"tags,omitempty" jsonschema:"Only recipes carrying all of these tags (see list_tags)"`
	MaxMinutes int      `json:"maxMinutes,omitempty" jsonschema:"Only recipes whose prep plus cook time is at most this many minutes, 0-1440; 0 is off"`
	Favorites  bool     `json:"favorites,omitempty" jsonschema:"Only the token owner's favorites"`
	Author     string   `json:"author,omitempty" jsonschema:"Only recipes written by this username (createdBy.username in get_recipe), max 50 characters"`
	Sort       string   `json:"sort,omitempty" jsonschema:"Result order: updated (default), created or title"`
	Page       int      `json:"page,omitempty" jsonschema:"1-based page, default 1"`
	Limit      int      `json:"limit,omitempty" jsonschema:"Page size, 1-100, default 24"`
}

// maxSearchPage bounds search_recipes' page. REST leaves page open-ended,
// but there a number too large for an int fails huma's query parsing with a
// 422; here it would reach the SDK's decoding into searchIn and come back as
// an error naming that Go type. At the largest page size a million pages
// cover a hundred million recipes, so no real page is cut off.
const maxSearchPage = 1_000_000

// searchSchema is search_recipes' input schema: searchIn's inferred schema
// with the bounds REST's listInput (recipe/handler.go) declares in huma tags,
// which jsonschema-go's struct tags cannot carry. The SDK validates every
// call against it, so a bad value is refused with a message naming the
// field before the handler runs.
func searchSchema() (*jsonschema.Schema, error) {
	s, err := jsonschema.For[searchIn](nil)
	if err != nil {
		return nil, fmt.Errorf("infer search_recipes schema: %w", err)
	}
	p := s.Properties
	p["query"].MaxLength = jsonschema.Ptr(100)
	p["author"].MaxLength = jsonschema.Ptr(50)
	p["maxMinutes"].Minimum, p["maxMinutes"].Maximum = jsonschema.Ptr(0.0), jsonschema.Ptr(1440.0)
	// "" is the default, as REST's ?sort= is: huma applies the default
	// before validating, the service maps "" to updated.
	p["sort"].Enum = []any{"", "updated", "created", "title"}
	p["page"].Minimum, p["page"].Maximum = jsonschema.Ptr(1.0), jsonschema.Ptr(float64(maxSearchPage))
	p["limit"].Minimum, p["limit"].Maximum = jsonschema.Ptr(1.0), jsonschema.Ptr(100.0)
	return s, nil
}

func addSearchRecipes(s *mcp.Server, c caller) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "search_recipes",
		Description: "Search the recipe collection. Returns one page of recipe cards (id, slug, title, tags, total time) and the total number of matches. Use get_recipe for ingredients and steps.",
		InputSchema: c.search,
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: new(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in searchIn) (*mcp.CallToolResult, any, error) {
		page, err := c.svc.List(ctx, recipe.ListParams{
			Query:         in.Query,
			Tags:          recipe.NormalizeTagQuery(in.Tags),
			MaxMinutes:    in.MaxMinutes,
			FavoritesOnly: in.Favorites,
			Author:        in.Author,
			Sort:          in.Sort,
			Page:          in.Page,
			Limit:         in.Limit,
			UserID:        c.user.ID,
		})
		if err != nil {
			return nil, nil, toolError(ctx, fmt.Errorf("search recipes: %w", err))
		}
		return nil, page, nil
	})
}

type getIn struct {
	ID   string `json:"id,omitempty" jsonschema:"The recipe's id"`
	Slug string `json:"slug,omitempty" jsonschema:"The recipe's slug, as in its URL"`
}

func addGetRecipe(s *mcp.Server, c caller) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_recipe",
		Description: "Read one recipe completely: ingredient groups, steps with their ingredient references, tags, authors, whether the token owner starred it and whether they may edit or delete it. Pass exactly one of id or slug.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: new(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in getIn) (*mcp.CallToolResult, any, error) {
		if (in.ID == "") == (in.Slug == "") {
			return nil, nil, errors.New("pass exactly one of id or slug")
		}
		var (
			r   recipe.Recipe
			err error
		)
		if in.ID != "" {
			r, err = c.svc.ByID(ctx, in.ID)
		} else {
			r, err = c.svc.BySlug(ctx, in.Slug)
		}
		if err != nil {
			return nil, nil, toolError(ctx, fmt.Errorf("get recipe: %w", err))
		}
		if err := c.fill(ctx, &r); err != nil {
			return nil, nil, toolError(ctx, err)
		}
		return nil, r, nil
	})
}

func addListTags(s *mcp.Server, c caller) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_tags",
		Description: "List every tag in use with how many recipes carry it, most used first. Prefer these over inventing new tags.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: new(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		tags, err := c.svc.Tags(ctx)
		if err != nil {
			return nil, nil, toolError(ctx, fmt.Errorf("list tags: %w", err))
		}
		return nil, map[string]any{"tags": tags}, nil
	})
}

// toolError turns what the service refuses into a message a model can act
// on. The SDK reports a returned error as a tool result with isError set,
// so these never become protocol errors. Anything unexpected is logged and
// reported as a generic error: its text may carry database internals.
func toolError(ctx context.Context, err error) error {
	var ref *recipe.RefError
	switch {
	case errors.Is(err, recipe.ErrNotFound):
		return errors.New("recipe not found")
	case errors.Is(err, recipe.ErrEditForbidden):
		return errors.New("you may not make this change: only the recipe's author and admins may edit a locked recipe or change its editPolicy")
	case errors.Is(err, recipe.ErrDeleteForbidden):
		return errors.New("only the recipe's author or an admin may delete it")
	case errors.As(err, &ref):
		return errors.New(ref.Error())
	default:
		slog.Default().ErrorContext(ctx, "mcp tool failed", "err", err)
		return errors.New("internal error")
	}
}
