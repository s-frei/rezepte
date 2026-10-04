package recipe

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/s-frei/rezepte/service/internal/auth"
)

// CommentList is the response body of list-comments.
type CommentList struct {
	Comments []Comment `json:"comments"`
}

type commentListOutput struct{ Body CommentList }

type commentBody struct {
	Body string `json:"body" doc:"Plain text, 1 to 2000 characters after trimming"`
}

type addCommentInput struct {
	ID   string `path:"id"`
	Body commentBody
}

type editCommentInput struct {
	CommentID int64 `path:"commentId"`
	Body      commentBody
}

type commentIDInput struct {
	CommentID int64 `path:"commentId"`
}

type markSeenInput struct {
	ID   string `path:"id"`
	Body struct {
		UpTo int64 `json:"upTo" minimum:"0" doc:"The highest comment id the page showed; the watermark is raised to it, never past the recipe's newest comment"`
	}
}

type listCommentsInput struct {
	RecipeID string `query:"recipeId" required:"true" doc:"The recipe whose comments to list"`
}

type commentOutput struct{ Body Comment }

// commentError maps the service's comment errors to HTTP; nil passes
// anything else through unchanged.
func commentError(err error) error {
	switch {
	case errors.Is(err, ErrNotFound):
		return huma.Error404NotFound("recipe not found")
	case errors.Is(err, ErrCommentNotFound):
		return huma.Error404NotFound(err.Error())
	case errors.Is(err, ErrNotCommentAuthor), errors.Is(err, ErrCommentDeleteForbidden):
		return huma.Error403Forbidden(err.Error())
	case errors.Is(err, ErrInvalidComment):
		return huma.Error422UnprocessableEntity(err.Error())
	}
	return err
}

// registerComments installs the comment operations. Reading takes
// recipes:read, writing recipes:write, like the recipe itself.
func registerComments(api huma.API, svc *Service) {
	huma.Register(api, huma.Operation{
		OperationID: "list-comments",
		Method:      http.MethodGet,
		Path:        "/api/v1/comments",
		Summary:     "List a recipe's comments, oldest first",
		Description: "new marks comments by somebody else since the caller last opened the recipe's comments, for the recipe's author and members with a comment on it. Listing does not mark anything seen.",
		Tags:        []string{"comments"},
		Security:    auth.Protected(auth.ScopeRecipesRead),
		Errors:      []int{404},
	}, func(ctx context.Context, in *listCommentsInput) (*commentListOutput, error) {
		u, err := auth.CurrentUser(ctx)
		if err != nil {
			return nil, err
		}
		list, err := svc.Comments(ctx, u, in.RecipeID)
		if err != nil {
			return nil, commentError(err)
		}
		return &commentListOutput{Body: CommentList{Comments: list}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "add-comment",
		Method:        http.MethodPost,
		Path:          "/api/v1/recipes/{id}/comments",
		Summary:       "Write a comment on a recipe",
		Description:   "Any member may write on any recipe, locked ones included. The comment is the caller's.",
		Tags:          []string{"comments"},
		Security:      auth.Protected(auth.ScopeRecipesWrite),
		DefaultStatus: http.StatusCreated,
		Errors:        []int{404, 422},
	}, func(ctx context.Context, in *addCommentInput) (*commentOutput, error) {
		u, err := auth.CurrentUser(ctx)
		if err != nil {
			return nil, err
		}
		c, err := svc.AddComment(ctx, u, in.ID, in.Body.Body)
		if err != nil {
			return nil, commentError(err)
		}
		return &commentOutput{Body: c}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "edit-comment",
		Method:      http.MethodPatch,
		Path:        "/api/v1/comments/{commentId}",
		Summary:     "Change the text of one's own comment",
		Tags:        []string{"comments"},
		Security:    auth.Protected(auth.ScopeRecipesWrite),
		Errors:      []int{403, 404, 422},
	}, func(ctx context.Context, in *editCommentInput) (*commentOutput, error) {
		u, err := auth.CurrentUser(ctx)
		if err != nil {
			return nil, err
		}
		c, err := svc.EditComment(ctx, u, in.CommentID, in.Body.Body)
		if err != nil {
			return nil, commentError(err)
		}
		return &commentOutput{Body: c}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "delete-comment",
		Method:        http.MethodDelete,
		Path:          "/api/v1/comments/{commentId}",
		Summary:       "Delete a comment",
		Description:   "The person who wrote it, or an admin.",
		Tags:          []string{"comments"},
		Security:      auth.Protected(auth.ScopeRecipesWrite),
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{403, 404},
	}, func(ctx context.Context, in *commentIDInput) (*favoriteOutput, error) {
		u, err := auth.CurrentUser(ctx)
		if err != nil {
			return nil, err
		}
		if err := svc.DeleteComment(ctx, u, in.CommentID); err != nil {
			return nil, commentError(err)
		}
		return &favoriteOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "mark-comments-seen",
		Method:        http.MethodPut,
		Path:          "/api/v1/recipes/{id}/comments/seen",
		Summary:       "Mark a recipe's comments as seen by the caller",
		Description:   "The recipe page calls this when the reader first opens its comments tab, with the highest comment id it loaded. Comments above it stay new. The editor, cook mode and MCP reads do not call it. It needs only recipes:read on purpose: it moves the caller's own unread marker and changes nothing anybody else sees, so a read-only client that shows comments can clear them.",
		Tags:          []string{"comments"},
		Security:      auth.Protected(auth.ScopeRecipesRead),
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{404},
	}, func(ctx context.Context, in *markSeenInput) (*favoriteOutput, error) {
		u, err := auth.CurrentUser(ctx)
		if err != nil {
			return nil, err
		}
		if err := svc.MarkCommentsSeen(ctx, u.ID, in.ID, in.Body.UpTo); err != nil {
			return nil, commentError(err)
		}
		return &favoriteOutput{}, nil
	})
}
