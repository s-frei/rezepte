package shopping

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"github.com/s-frei/rezepte/service/internal/auth"
)

type (
	// SummaryList is the response body of the list-shopping-lists operation.
	SummaryList struct {
		Items       []Summary    `json:"items"`
		Invitations []Invitation `json:"invitations"`
	}
	nameBody struct {
		Name string `json:"name" minLength:"1" maxLength:"80" doc:"List name; surrounding whitespace is trimmed"`
	}
	addRecipeBody struct {
		RecipeID    string           `json:"recipeId" minLength:"1"`
		Servings    int              `json:"servings" minimum:"1" maximum:"99"`
		Ingredients []IngredientPick `json:"ingredients" minItems:"1" doc:"The recipe ingredients to copy, by group and position"`
	}
	servingsBody struct {
		Servings int `json:"servings" minimum:"1" maximum:"99"`
	}
	textBody struct {
		Text string `json:"text" minLength:"1" maxLength:"200" doc:"Free text such as \"2 l Milch\""`
	}
	checkBody struct {
		ItemIDs []string `json:"itemIds" maxItems:"500"`
		Checked bool     `json:"checked"`
	}
	removeBody struct {
		ItemIDs []string `json:"itemIds" maxItems:"500"`
	}
	departmentBody struct {
		Name       string     `json:"name" minLength:"1" maxLength:"200"`
		Department Department `json:"department" enum:"produce,bakery,chilled,meat,pantry,frozen,drinks,household,other"`
	}

	listID struct {
		ID string `path:"id"`
	}
	removeEntryIn struct {
		ID      string `path:"id"`
		EntryID string `path:"entryId"`
	}
	inviteIn struct {
		ID   string `path:"id"`
		Body struct {
			UserID string `json:"userId" minLength:"1"`
		}
	}
	memberIn struct {
		ID     string `path:"id"`
		UserID string `path:"userId"`
	}
	listOut   struct{ Body List }
	emptyOut  struct{}
	summaries struct{ Body SummaryList }

	createIn struct{ Body nameBody }
	renameIn struct {
		ID   string `path:"id"`
		Body nameBody
	}
	addRecipeIn struct {
		ID   string `path:"id"`
		Body addRecipeBody
	}
	servingsIn struct {
		ID      string `path:"id"`
		EntryID string `path:"entryId"`
		Body    servingsBody
	}
	textIn struct {
		ID   string `path:"id"`
		Body textBody
	}
	checkIn struct {
		ID   string `path:"id"`
		Body checkBody
	}
	removeIn struct {
		ID   string `path:"id"`
		Body removeBody
	}
	departmentIn struct{ Body departmentBody }
)

// Register installs the shopping-list operations onto api. Every operation
// needs a session; a list is visible to its creator and accepted members.
func Register(api huma.API, svc *Service) {
	op := func(id, method, path, summary string, status int, errs ...int) huma.Operation {
		return huma.Operation{
			OperationID: id, Method: method, Path: path, Summary: summary,
			Tags: []string{"shopping-lists"}, Security: auth.SessionSecurity,
			DefaultStatus: status, Errors: errs,
		}
	}
	// fail maps service errors; field is the body field a validation error names.
	fail := func(err error, field string) error {
		switch {
		case errors.Is(err, ErrNotFound):
			return huma.Error404NotFound("shopping list not found")
		case errors.Is(err, ErrRecipeNotFound):
			return huma.Error404NotFound("recipe not found")
		case errors.Is(err, ErrInvalid):
			msg := strings.TrimPrefix(err.Error(), ErrInvalid.Error()+": ")
			return huma.Error422UnprocessableEntity("validation failed", &huma.ErrorDetail{Location: "body." + field, Message: msg})
		}
		return err
	}
	caller := func(ctx context.Context) (string, error) {
		u, ok := auth.UserFrom(ctx)
		if !ok {
			return "", huma.Error401Unauthorized("authentication required")
		}
		return u.ID, nil
	}
	// mutation wraps the common shape: authenticate, call, map errors.
	mutation := func(ctx context.Context, field string, f func(uid string) (List, error)) (*listOut, error) {
		uid, err := caller(ctx)
		if err != nil {
			return nil, err
		}
		l, err := f(uid)
		if err != nil {
			return nil, fail(err, field)
		}
		return &listOut{Body: l}, nil
	}

	huma.Register(api, op("list-shopping-lists", http.MethodGet, "/api/v1/shopping-lists", "List the caller's shopping lists", http.StatusOK),
		func(ctx context.Context, _ *struct{}) (*summaries, error) {
			uid, err := caller(ctx)
			if err != nil {
				return nil, err
			}
			items, err := svc.Lists(ctx, uid)
			if err != nil {
				return nil, err
			}
			invs, err := svc.Invitations(ctx, uid)
			if err != nil {
				return nil, err
			}
			return &summaries{Body: SummaryList{Items: items, Invitations: invs}}, nil
		})

	huma.Register(api, op("create-shopping-list", http.MethodPost, "/api/v1/shopping-lists", "Create a shopping list", http.StatusCreated, 422),
		func(ctx context.Context, in *createIn) (*listOut, error) {
			return mutation(ctx, "name", func(uid string) (List, error) { return svc.Create(ctx, uid, in.Body.Name) })
		})

	huma.Register(api, op("get-shopping-list", http.MethodGet, "/api/v1/shopping-lists/{id}", "Get a shopping list", http.StatusOK, 404),
		func(ctx context.Context, in *listID) (*listOut, error) {
			return mutation(ctx, "", func(uid string) (List, error) { return svc.Get(ctx, uid, in.ID) })
		})

	huma.Register(api, op("rename-shopping-list", http.MethodPatch, "/api/v1/shopping-lists/{id}", "Rename a shopping list", http.StatusOK, 404, 422),
		func(ctx context.Context, in *renameIn) (*listOut, error) {
			return mutation(ctx, "name", func(uid string) (List, error) { return svc.Rename(ctx, uid, in.ID, in.Body.Name) })
		})

	huma.Register(api, op("delete-shopping-list", http.MethodDelete, "/api/v1/shopping-lists/{id}", "Delete a shopping list", http.StatusNoContent, 404),
		func(ctx context.Context, in *listID) (*emptyOut, error) {
			uid, err := caller(ctx)
			if err != nil {
				return nil, err
			}
			if err := svc.Delete(ctx, uid, in.ID); err != nil {
				return nil, fail(err, "")
			}
			return &emptyOut{}, nil
		})

	huma.Register(api, op("leave-shopping-list", http.MethodDelete, "/api/v1/shopping-lists/{id}/membership", "Leave a list or decline its invitation", http.StatusNoContent, 404, 422),
		func(ctx context.Context, in *listID) (*emptyOut, error) {
			uid, err := caller(ctx)
			if err != nil {
				return nil, err
			}
			if err := svc.Leave(ctx, uid, in.ID); err != nil {
				return nil, fail(err, "")
			}
			return &emptyOut{}, nil
		})

	huma.Register(api, op("invite-to-shopping-list", http.MethodPost, "/api/v1/shopping-lists/{id}/invitations", "Invite a person to a list", http.StatusOK, 404, 422),
		func(ctx context.Context, in *inviteIn) (*listOut, error) {
			return mutation(ctx, "userId", func(uid string) (List, error) { return svc.Invite(ctx, uid, in.ID, in.Body.UserID) })
		})

	huma.Register(api, op("remove-shopping-list-member", http.MethodDelete, "/api/v1/shopping-lists/{id}/members/{userId}", "Remove a member or withdraw an invitation", http.StatusOK, 404),
		func(ctx context.Context, in *memberIn) (*listOut, error) {
			return mutation(ctx, "", func(uid string) (List, error) { return svc.RemoveMember(ctx, uid, in.ID, in.UserID) })
		})

	huma.Register(api, op("accept-shopping-list-invitation", http.MethodPost, "/api/v1/shopping-lists/{id}/membership", "Accept an invitation to a list", http.StatusOK, 404),
		func(ctx context.Context, in *listID) (*listOut, error) {
			return mutation(ctx, "", func(uid string) (List, error) { return svc.Accept(ctx, uid, in.ID) })
		})

	huma.Register(api, op("add-recipe-to-shopping-list", http.MethodPost, "/api/v1/shopping-lists/{id}/recipes", "Copy a recipe's ingredients onto a list", http.StatusOK, 404, 422),
		func(ctx context.Context, in *addRecipeIn) (*listOut, error) {
			return mutation(ctx, "ingredients", func(uid string) (List, error) {
				return svc.AddRecipe(ctx, uid, in.ID, in.Body.RecipeID, in.Body.Servings, in.Body.Ingredients)
			})
		})

	huma.Register(api, op("set-shopping-entry-servings", http.MethodPatch, "/api/v1/shopping-lists/{id}/recipes/{entryId}", "Change the servings of a recipe on a list", http.StatusOK, 404, 422),
		func(ctx context.Context, in *servingsIn) (*listOut, error) {
			return mutation(ctx, "servings", func(uid string) (List, error) {
				return svc.SetServings(ctx, uid, in.ID, in.EntryID, in.Body.Servings)
			})
		})

	huma.Register(api, op("remove-shopping-entry", http.MethodDelete, "/api/v1/shopping-lists/{id}/recipes/{entryId}", "Remove a recipe and its items from a list", http.StatusOK, 404),
		func(ctx context.Context, in *removeEntryIn) (*listOut, error) {
			return mutation(ctx, "", func(uid string) (List, error) { return svc.RemoveEntry(ctx, uid, in.ID, in.EntryID) })
		})

	huma.Register(api, op("add-shopping-item", http.MethodPost, "/api/v1/shopping-lists/{id}/items", "Add a free-text item", http.StatusOK, 404, 422),
		func(ctx context.Context, in *textIn) (*listOut, error) {
			return mutation(ctx, "text", func(uid string) (List, error) { return svc.AddFree(ctx, uid, in.ID, in.Body.Text) })
		})

	huma.Register(api, op("check-shopping-items", http.MethodPost, "/api/v1/shopping-lists/{id}/items/check", "Tick or untick items", http.StatusOK, 404),
		func(ctx context.Context, in *checkIn) (*listOut, error) {
			return mutation(ctx, "", func(uid string) (List, error) {
				return svc.SetChecked(ctx, uid, in.ID, in.Body.ItemIDs, in.Body.Checked)
			})
		})

	huma.Register(api, op("remove-shopping-items", http.MethodPost, "/api/v1/shopping-lists/{id}/items/remove", "Remove items", http.StatusOK, 404),
		func(ctx context.Context, in *removeIn) (*listOut, error) {
			return mutation(ctx, "", func(uid string) (List, error) { return svc.RemoveItems(ctx, uid, in.ID, in.Body.ItemIDs) })
		})

	huma.Register(api, op("clear-checked-shopping-items", http.MethodPost, "/api/v1/shopping-lists/{id}/clear-checked", "Remove every ticked item", http.StatusOK, 404),
		func(ctx context.Context, in *listID) (*listOut, error) {
			return mutation(ctx, "", func(uid string) (List, error) { return svc.ClearChecked(ctx, uid, in.ID) })
		})

	huma.Register(api, op("set-shopping-department", http.MethodPut, "/api/v1/shopping-departments", "Sort an ingredient name under a department on every list", http.StatusNoContent, 422),
		func(ctx context.Context, in *departmentIn) (*emptyOut, error) {
			uid, err := caller(ctx)
			if err != nil {
				return nil, err
			}
			if err := svc.SetDepartment(ctx, uid, in.Body.Name, in.Body.Department); err != nil {
				return nil, fail(err, "name")
			}
			return &emptyOut{}, nil
		})
}
