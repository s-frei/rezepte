// Package userapi exposes user management (admin only) over HTTP. It is a
// separate package because it needs auth.UserFrom and auth.SessionSecurity,
// and package auth already imports package user.
package userapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/user"
)

// Response is a user as the API shows it (no secrets).
type Response struct {
	ID        string    `json:"id" doc:"User id"`
	Username  string    `json:"username" doc:"Login name"`
	Role      string    `json:"role" enum:"admin,user" doc:"Authorization role"`
	CreatedAt time.Time `json:"createdAt" doc:"When the account was created"`
}

// List is the response body of list-users.
type List struct {
	Items []Response `json:"items"`
}

type listOutput struct {
	Body List
}

type createInput struct {
	Body struct {
		Username string `json:"username" minLength:"1" maxLength:"64"`
		Password string `json:"password" minLength:"8" maxLength:"128"`
		Role     string `json:"role" enum:"admin,user"`
	}
}

type userOutput struct {
	Body Response
}

type updateInput struct {
	ID   string `path:"id"`
	Body struct {
		Password *string `json:"password,omitempty" minLength:"8" maxLength:"128" doc:"New password; ends all of the user's sessions"`
		Role     *string `json:"role,omitempty" enum:"admin,user"`
	}
}

type deleteInput struct {
	ID string `path:"id"`
}

type deleteOutput struct{}

// requireAdmin returns the calling user or a 401/403 huma error. Every
// operation in this package calls it first; there is no permission table,
// the role on the session's user is the whole authorization model.
func requireAdmin(ctx context.Context) (user.User, error) {
	u, ok := auth.UserFrom(ctx)
	if !ok {
		return user.User{}, huma.Error401Unauthorized("authentication required")
	}
	if u.Role != user.RoleAdmin {
		return user.User{}, huma.Error403Forbidden("admin role required")
	}
	return u, nil
}

func toResponse(u user.User) Response {
	return Response{ID: u.ID, Username: u.Username, Role: string(u.Role), CreatedAt: u.CreatedAt}
}

// Register installs list, create, update and delete for users. All four
// require an admin session.
func Register(api huma.API, users *user.Service, sessions *auth.Service) {
	huma.Register(api, huma.Operation{
		OperationID: "list-users",
		Method:      http.MethodGet,
		Path:        "/api/v1/users",
		Summary:     "List users",
		Tags:        []string{"users"},
		Security:    auth.SessionSecurity,
		Errors:      []int{401, 403},
	}, func(ctx context.Context, _ *struct{}) (*listOutput, error) {
		if _, err := requireAdmin(ctx); err != nil {
			return nil, err
		}
		list, err := users.List(ctx)
		if err != nil {
			return nil, err
		}
		items := make([]Response, 0, len(list))
		for _, u := range list {
			items = append(items, toResponse(u))
		}
		return &listOutput{Body: List{Items: items}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "create-user",
		Method:        http.MethodPost,
		Path:          "/api/v1/users",
		Summary:       "Create a user",
		Tags:          []string{"users"},
		Security:      auth.SessionSecurity,
		DefaultStatus: http.StatusCreated,
		Errors:        []int{401, 403, 409},
	}, func(ctx context.Context, in *createInput) (*userOutput, error) {
		if _, err := requireAdmin(ctx); err != nil {
			return nil, err
		}
		u, err := users.Create(ctx, in.Body.Username, in.Body.Password, user.Role(in.Body.Role))
		if errors.Is(err, user.ErrUsernameTaken) {
			return nil, huma.Error409Conflict("username already taken")
		}
		if err != nil {
			return nil, err
		}
		return &userOutput{Body: toResponse(u)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-user",
		Method:      http.MethodPatch,
		Path:        "/api/v1/users/{id}",
		Summary:     "Change a user's role and/or reset the password",
		Tags:        []string{"users"},
		Security:    auth.SessionSecurity,
		Errors:      []int{401, 403, 404, 409, 422},
	}, func(ctx context.Context, in *updateInput) (*userOutput, error) {
		if _, err := requireAdmin(ctx); err != nil {
			return nil, err
		}
		if in.Body.Password == nil && in.Body.Role == nil {
			return nil, huma.Error422UnprocessableEntity("password or role required")
		}
		var (
			u   user.User
			err error
		)
		if in.Body.Role != nil {
			u, err = users.SetRole(ctx, in.ID, user.Role(*in.Body.Role))
		} else {
			u, err = users.ByID(ctx, in.ID)
		}
		if errors.Is(err, user.ErrNotFound) {
			return nil, huma.Error404NotFound("user not found")
		}
		if errors.Is(err, user.ErrLastAdmin) {
			return nil, huma.Error409Conflict("the last admin cannot be demoted")
		}
		if err != nil {
			return nil, err
		}
		if in.Body.Password != nil {
			if err := users.SetPassword(ctx, in.ID, *in.Body.Password); err != nil {
				return nil, err
			}
			// A reset means the old credential is compromised or forgotten:
			// every device logged in with it goes.
			if err := sessions.DeleteUserSessionsExcept(ctx, in.ID, ""); err != nil {
				return nil, err
			}
		}
		return &userOutput{Body: toResponse(u)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "delete-user",
		Method:        http.MethodDelete,
		Path:          "/api/v1/users/{id}",
		Summary:       "Delete a user",
		Tags:          []string{"users"},
		Security:      auth.SessionSecurity,
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{401, 403, 404, 409},
	}, func(ctx context.Context, in *deleteInput) (*deleteOutput, error) {
		actor, err := requireAdmin(ctx)
		if err != nil {
			return nil, err
		}
		err = users.Delete(ctx, actor.ID, in.ID)
		switch {
		case errors.Is(err, user.ErrSelfDelete):
			return nil, huma.Error409Conflict("cannot delete yourself")
		case errors.Is(err, user.ErrLastAdmin):
			return nil, huma.Error409Conflict("the last admin cannot be deleted")
		case errors.Is(err, user.ErrNotFound):
			return nil, huma.Error404NotFound("user not found")
		case err != nil:
			return nil, err
		}
		return &deleteOutput{}, nil
	})
}
