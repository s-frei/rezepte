package mealplan

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/db/sqlc"
	"github.com/s-frei/rezepte/service/internal/shopping"
)

type (
	// PlanOverview is the response body of the list-meal-plans operation.
	PlanOverview struct {
		Plans       []PlanSummary    `json:"plans"`
		Invitations []PlanInvitation `json:"invitations"`
	}
	// PlansWeek is the response body of the meal-plans-week operation.
	PlansWeek struct {
		From        string           `json:"from"`
		To          string           `json:"to"`
		Plans       []WeekPlan       `json:"plans"`
		Invitations []PlanInvitation `json:"invitations"`
	}
	// Range is the week (or any span) a Plan answer carries entries for;
	// exported because huma skips unexported embedded fields.
	Range struct {
		From string `query:"from" format:"date" doc:"First day (YYYY-MM-DD) of the entries to answer with; with to left out too, the current week"`
		To   string `query:"to" format:"date" doc:"Last day (YYYY-MM-DD), inclusive, at most 62 days after from"`
	}
	planNameBody struct {
		Name string `json:"name" minLength:"1" maxLength:"80" doc:"Plan name; surrounding whitespace is trimmed"`
	}
	planAddBody struct {
		Day      *string `json:"day,omitempty" format:"date" doc:"The day (YYYY-MM-DD); left out, the tray"`
		RecipeID *string `json:"recipeId,omitempty" doc:"The recipe to plan; give this or title"`
		Title    *string `json:"title,omitempty" maxLength:"120" doc:"A free text such as \"Reste\"; give this or recipeId"`
		Servings *int    `json:"servings,omitempty" minimum:"1" maximum:"99" doc:"A recipe's servings; left out, the recipe's own"`
	}
	planEntryBody struct {
		Day      OptionalDay `json:"day,omitempty" doc:"The day to move to (YYYY-MM-DD), or null for the tray; left out, the entry keeps its day"`
		Position *int        `json:"position,omitempty" minimum:"0" doc:"The place within the day, from 0; clamped to the end. Left out with day given, the end"`
		Servings *int        `json:"servings,omitempty" minimum:"1" maximum:"99" doc:"A recipe entry's servings"`
	}
	planPickBody struct {
		EntryID     string                    `json:"entryId" minLength:"1"`
		Ingredients []shopping.IngredientPick `json:"ingredients" minItems:"1" doc:"The recipe ingredients to copy, by group and position"`
	}
	planShoppingBody struct {
		ListID      *string        `json:"listId,omitempty" doc:"A list the caller can access; give this or newListName"`
		NewListName *string        `json:"newListName,omitempty" maxLength:"80" doc:"The name of a new list of the caller's"`
		Picks       []planPickBody `json:"picks" minItems:"1" maxItems:"200"`
		AlsoListed  bool           `json:"alsoListed,omitempty" doc:"Skip an entry only when it is on this list already; left out, an entry on any list the caller can access is skipped"`
	}
	// ToListResult is the response body of the plan-to-shopping-list operation.
	ToListResult struct {
		Plan   Plan   `json:"plan"`
		ListID string `json:"listId"`
	}
	planStapleBody struct {
		Name   string `json:"name" minLength:"1" maxLength:"200"`
		Staple bool   `json:"staple" doc:"true marks the name as a staple, false unmarks it"`
	}
	// StapleList is the response body of the staples operations.
	StapleList struct {
		Staples []Staple `json:"staples"`
	}
	// PreviewList is the response body of the shopping-preview operation.
	PreviewList struct {
		Entries []PreviewEntry `json:"entries"`
	}

	planID struct {
		ID string `path:"id"`
	}
	planIn struct {
		ID string `path:"id"`
		Range
	}
	entryIn struct {
		ID      string `path:"id"`
		EntryID string `path:"entryId"`
		Range
	}
	createIn struct {
		Range
		Body planNameBody
	}
	renameIn struct {
		ID string `path:"id"`
		Range
		Body planNameBody
	}
	addIn struct {
		ID string `path:"id"`
		Range
		Body planAddBody
	}
	patchEntryIn struct {
		ID      string `path:"id"`
		EntryID string `path:"entryId"`
		Range
		Body planEntryBody
	}
	shoppingIn struct {
		ID string `path:"id"`
		Range
		Body planShoppingBody
	}
	stapleIn struct {
		ID   string `path:"id"`
		Body planStapleBody
	}
	previewIn struct {
		ID string `path:"id"`
		Range
		Tray  bool   `query:"tray" doc:"Also offer the entries on the tray"`
		Entry string `query:"entry" doc:"Offer only this recipe entry, on its day or the tray, whatever from, to and tray say"`
	}
	inviteIn struct {
		ID string `path:"id"`
		Range
		Body struct {
			UserID string `json:"userId" minLength:"1"`
		}
	}
	memberIn struct {
		ID     string `path:"id"`
		UserID string `path:"userId"`
		Range
	}

	planOut     struct{ Body Plan }
	weekOut     struct{ Body PlansWeek }
	emptyOut    struct{}
	overviewOut struct{ Body PlanOverview }
	toListOut   struct{ Body ToListResult }
	staplesOut  struct{ Body StapleList }
	previewOut  struct{ Body PreviewList }
)

// OptionalDay is a JSON day that tells a missing key from null: Sent is
// whether the key was there, Value is nil for null (the tray).
type OptionalDay struct {
	Sent  bool
	Value *string
}

// UnmarshalJSON records that the key was sent; encoding/json calls it for
// null too.
func (o *OptionalDay) UnmarshalJSON(b []byte) error {
	o.Sent = true
	if string(b) == "null" {
		o.Value = nil
		return nil
	}
	return json.Unmarshal(b, &o.Value)
}

// Schema describes the day as a nullable date string.
func (OptionalDay) Schema(huma.Registry) *huma.Schema {
	return &huma.Schema{Type: huma.TypeString, Format: "date", Nullable: true}
}

// Register installs the meal-plan operations onto api. Every operation needs
// a session or a token (plan:write, or plan:read for the readers); a plan is
// visible to its creator and accepted members. Operations answering a Plan
// take from and to and answer with the entries of that span and the tray.
func Register(api huma.API, svc *Service) {
	op := func(id, method, path, summary string, status int, errs ...int) huma.Operation {
		return huma.Operation{
			OperationID: id, Method: method, Path: path, Summary: summary,
			Tags: []string{"meal-plans"}, Security: auth.Protected(auth.ScopePlanWrite),
			DefaultStatus: status, Errors: errs,
		}
	}
	// reader marks a read-only operation as reachable with plan:read.
	reader := func(o huma.Operation) huma.Operation {
		o.Security = auth.Protected(auth.ScopePlanRead)
		return o
	}
	// fail maps service errors; field is the body field a validation error names.
	fail := func(err error, field string) error {
		switch {
		case errors.Is(err, ErrNotFound):
			return huma.Error404NotFound("meal plan not found")
		case errors.Is(err, shopping.ErrNotFound):
			return huma.Error404NotFound("shopping list not found")
		case errors.Is(err, ErrRecipeNotFound), errors.Is(err, shopping.ErrRecipeNotFound):
			return huma.Error404NotFound("recipe not found")
		case errors.Is(err, ErrInvalid):
			return invalidField(strings.TrimPrefix(err.Error(), ErrInvalid.Error()+": "), field)
		}
		return err
	}
	// span is the range asked for, the current week when both ends are left
	// out; checked here so its error names the query, not the body.
	span := func(r Range) (string, string, error) {
		if r.From == "" && r.To == "" {
			from, to := week(svc.now())
			return from, to, nil
		}
		if err := checkRange(r.From, r.To); err != nil {
			msg := strings.TrimPrefix(err.Error(), ErrInvalid.Error()+": ")
			return "", "", huma.Error422UnprocessableEntity("validation failed", &huma.ErrorDetail{Location: "query.from", Message: msg})
		}
		return r.From, r.To, nil
	}
	// mutation wraps the common shape: authenticate, resolve the range, call,
	// map errors.
	mutation := func(ctx context.Context, r Range, field string, f func(uid, from, to string) (Plan, error)) (*planOut, error) {
		uid, err := auth.CurrentUserID(ctx)
		if err != nil {
			return nil, err
		}
		from, to, err := span(r)
		if err != nil {
			return nil, err
		}
		p, err := f(uid, from, to)
		if err != nil {
			return nil, fail(err, field)
		}
		return &planOut{Body: p}, nil
	}
	// inRange answers a change whose service call returns the current week
	// with the range asked for instead.
	inRange := func(ctx context.Context, uid, from, to string, p Plan, err error) (Plan, error) {
		if err != nil || p.From == from && p.To == to {
			return p, err
		}
		return svc.Get(ctx, uid, p.ID, from, to)
	}
	command := func(ctx context.Context, f func(uid string) error) (*emptyOut, error) {
		uid, err := auth.CurrentUserID(ctx)
		if err != nil {
			return nil, err
		}
		if err := f(uid); err != nil {
			return nil, fail(err, "")
		}
		return &emptyOut{}, nil
	}

	huma.Register(api, reader(op("list-meal-plans", http.MethodGet, "/api/v1/meal-plans", "List the caller's meal plans and invitations", http.StatusOK)),
		func(ctx context.Context, _ *struct{}) (*overviewOut, error) {
			uid, err := auth.CurrentUserID(ctx)
			if err != nil {
				return nil, err
			}
			plans, invs, err := svc.Plans(ctx, uid)
			if err != nil {
				return nil, err
			}
			return &overviewOut{Body: PlanOverview{Plans: plans, Invitations: invs}}, nil
		})

	plansWeek := reader(op("meal-plans-week", http.MethodGet, "/api/v1/meal-plans/week", "Every plan of the caller's with a week of it", http.StatusOK, 422))
	plansWeek.Description = "The plans as list-meal-plans orders them, each with the entries on the days of the range, the lists the caller can access that they and the tray are on, and how many recipe entries are on none; and the pending invitations. One request for the plans page, however many plans. Left out, the range is the current week in the server's time zone, so a client passes its own."
	huma.Register(api, plansWeek, func(ctx context.Context, in *Range) (*weekOut, error) {
		uid, err := auth.CurrentUserID(ctx)
		if err != nil {
			return nil, err
		}
		from, to, err := span(*in)
		if err != nil {
			return nil, err
		}
		plans, invs, err := svc.PlansWeek(ctx, uid, from, to)
		if err != nil {
			return nil, fail(err, "")
		}
		return &weekOut{Body: PlansWeek{From: from, To: to, Plans: plans, Invitations: invs}}, nil
	})

	huma.Register(api, op("create-meal-plan", http.MethodPost, "/api/v1/meal-plans", "Create a shared meal plan", http.StatusCreated, 422),
		func(ctx context.Context, in *createIn) (*planOut, error) {
			return mutation(ctx, in.Range, "name", func(uid, from, to string) (Plan, error) {
				p, err := svc.Create(ctx, uid, in.Body.Name)
				return inRange(ctx, uid, from, to, p, err)
			})
		})

	huma.Register(api, reader(op("get-meal-plan", http.MethodGet, "/api/v1/meal-plans/{id}", "Get a meal plan for a range of days", http.StatusOK, 404, 422)),
		func(ctx context.Context, in *planIn) (*planOut, error) {
			return mutation(ctx, in.Range, "", func(uid, from, to string) (Plan, error) { return svc.Get(ctx, uid, in.ID, from, to) })
		})

	stream := reader(op("stream-meal-plan", http.MethodGet, "/api/v1/meal-plans/{id}/events", "Follow a meal plan's changes", http.StatusOK, 404))
	stream.Description = "A Server-Sent Events stream: `retry: 3000` first, then `event: changed` after every change to the plan, its JSON data carrying the plan's `revision` " +
		"(refetch the plan to see what changed, unless the one you hold already has that revision), and a `: ping` comment every 25 seconds. The stream ends when the caller loses access to the plan or the server shuts down."
	stream.Responses = map[string]*huma.Response{"200": {
		Description: "The event stream",
		Content:     map[string]*huma.MediaType{"text/event-stream": {Schema: &huma.Schema{Type: "string"}}},
	}}
	huma.Register(api, stream, func(ctx context.Context, in *planID) (*huma.StreamResponse, error) {
		uid, err := auth.CurrentUserID(ctx)
		if err != nil {
			return nil, err
		}
		if err := svc.canAccess(ctx, uid, in.ID); err != nil {
			return nil, fail(err, "")
		}
		return svc.hub.Stream(in.ID, func(ctx context.Context) (int64, error) {
			return svc.q.GetMealPlanRevision(ctx, sqlc.GetMealPlanRevisionParams{ID: in.ID, UserID: uid})
		}), nil
	})

	huma.Register(api, op("rename-meal-plan", http.MethodPatch, "/api/v1/meal-plans/{id}", "Rename a shared meal plan", http.StatusOK, 404, 422),
		func(ctx context.Context, in *renameIn) (*planOut, error) {
			return mutation(ctx, in.Range, "name", func(uid, from, to string) (Plan, error) {
				p, err := svc.Rename(ctx, uid, in.ID, in.Body.Name)
				return inRange(ctx, uid, from, to, p, err)
			})
		})

	huma.Register(api, op("delete-meal-plan", http.MethodDelete, "/api/v1/meal-plans/{id}", "Delete a shared meal plan", http.StatusNoContent, 404, 422),
		func(ctx context.Context, in *planID) (*emptyOut, error) {
			return command(ctx, func(uid string) error { return svc.Delete(ctx, uid, in.ID) })
		})

	huma.Register(api, op("add-meal-plan-entry", http.MethodPost, "/api/v1/meal-plans/{id}/entries", "Plan a recipe or a free text", http.StatusOK, 404, 422),
		func(ctx context.Context, in *addIn) (*planOut, error) {
			return mutation(ctx, in.Range, "", func(uid, from, to string) (Plan, error) {
				return svc.Add(ctx, uid, in.ID, from, to, AddEntry(in.Body))
			})
		})

	huma.Register(api, op("update-meal-plan-entry", http.MethodPatch, "/api/v1/meal-plans/{id}/entries/{entryId}", "Move an entry or change its servings", http.StatusOK, 404, 422),
		func(ctx context.Context, in *patchEntryIn) (*planOut, error) {
			return mutation(ctx, in.Range, "", func(uid, from, to string) (Plan, error) {
				b := in.Body
				return svc.Update(ctx, uid, in.ID, in.EntryID, from, to, EntryChange{Moved: b.Day.Sent, Day: b.Day.Value, Position: b.Position, Servings: b.Servings})
			})
		})

	huma.Register(api, op("remove-meal-plan-entry", http.MethodDelete, "/api/v1/meal-plans/{id}/entries/{entryId}", "Remove an entry, restorable for a day", http.StatusOK, 404, 422),
		func(ctx context.Context, in *entryIn) (*planOut, error) {
			return mutation(ctx, in.Range, "", func(uid, from, to string) (Plan, error) {
				return svc.Remove(ctx, uid, in.ID, in.EntryID, from, to)
			})
		})

	huma.Register(api, op("restore-meal-plan-entry", http.MethodPost, "/api/v1/meal-plans/{id}/entries/{entryId}/restore", "Bring back a removed entry", http.StatusOK, 404, 422),
		func(ctx context.Context, in *entryIn) (*planOut, error) {
			return mutation(ctx, in.Range, "", func(uid, from, to string) (Plan, error) {
				return svc.Restore(ctx, uid, in.ID, in.EntryID, from, to)
			})
		})

	// Putting a plan on a list writes a list, so a token needs lists:write too.
	toList := op("plan-to-shopping-list", http.MethodPost, "/api/v1/meal-plans/{id}/shopping", "Put planned recipes on a shopping list", http.StatusOK, 404, 422)
	toList.Security = auth.Protected(auth.ScopePlanWrite, auth.ScopeListsWrite)
	huma.Register(api, toList,
		func(ctx context.Context, in *shoppingIn) (*toListOut, error) {
			var listID string
			out, err := mutation(ctx, in.Range, "picks", func(uid, from, to string) (Plan, error) {
				t := ToList{ListID: in.Body.ListID, NewListName: in.Body.NewListName, AlsoListed: in.Body.AlsoListed}
				for _, p := range in.Body.Picks {
					t.Picks = append(t.Picks, ToListPick{EntryID: p.EntryID, Include: p.Ingredients})
				}
				p, id, err := svc.ToList(ctx, uid, in.ID, from, to, t)
				listID = id
				return p, err
			})
			if err != nil {
				return nil, err
			}
			return &toListOut{Body: ToListResult{Plan: out.Body, ListID: listID}}, nil
		})

	huma.Register(api, reader(op("meal-plan-shopping-preview", http.MethodGet, "/api/v1/meal-plans/{id}/shopping-preview", "Preview what a range puts on a list", http.StatusOK, 404, 422)),
		func(ctx context.Context, in *previewIn) (*previewOut, error) {
			uid, err := auth.CurrentUserID(ctx)
			if err != nil {
				return nil, err
			}
			from, to, err := span(in.Range)
			if err != nil {
				return nil, err
			}
			var entries []PreviewEntry
			if in.Entry != "" {
				var one PreviewEntry
				one, err = svc.PreviewOne(ctx, uid, in.ID, in.Entry)
				entries = []PreviewEntry{one}
			} else {
				entries, err = svc.Preview(ctx, uid, in.ID, from, to, in.Tray)
			}
			if err != nil {
				return nil, fail(err, "")
			}
			return &previewOut{Body: PreviewList{Entries: entries}}, nil
		})

	huma.Register(api, reader(op("list-meal-plan-staples", http.MethodGet, "/api/v1/meal-plans/{id}/staples", "List a plan's staples", http.StatusOK, 404)),
		func(ctx context.Context, in *planID) (*staplesOut, error) {
			uid, err := auth.CurrentUserID(ctx)
			if err != nil {
				return nil, err
			}
			st, err := svc.Staples(ctx, uid, in.ID)
			if err != nil {
				return nil, fail(err, "")
			}
			return &staplesOut{Body: StapleList{Staples: st}}, nil
		})

	huma.Register(api, op("set-meal-plan-staple", http.MethodPut, "/api/v1/meal-plans/{id}/staples", "Mark or unmark a staple", http.StatusOK, 404, 422),
		func(ctx context.Context, in *stapleIn) (*staplesOut, error) {
			uid, err := auth.CurrentUserID(ctx)
			if err != nil {
				return nil, err
			}
			st, err := svc.SetStaple(ctx, uid, in.ID, in.Body.Name, in.Body.Staple)
			if err != nil {
				return nil, fail(err, "name")
			}
			return &staplesOut{Body: StapleList{Staples: st}}, nil
		})

	huma.Register(api, op("invite-to-meal-plan", http.MethodPost, "/api/v1/meal-plans/{id}/invitations", "Invite a person to a plan", http.StatusOK, 404, 422),
		func(ctx context.Context, in *inviteIn) (*planOut, error) {
			return mutation(ctx, in.Range, "userId", func(uid, from, to string) (Plan, error) {
				p, err := svc.Invite(ctx, uid, in.ID, in.Body.UserID)
				return inRange(ctx, uid, from, to, p, err)
			})
		})

	huma.Register(api, op("remove-meal-plan-member", http.MethodDelete, "/api/v1/meal-plans/{id}/members/{userId}", "Remove a member or withdraw an invitation", http.StatusOK, 404),
		func(ctx context.Context, in *memberIn) (*planOut, error) {
			return mutation(ctx, in.Range, "", func(uid, from, to string) (Plan, error) {
				p, err := svc.RemoveMember(ctx, uid, in.ID, in.UserID)
				return inRange(ctx, uid, from, to, p, err)
			})
		})

	huma.Register(api, op("accept-meal-plan-invitation", http.MethodPost, "/api/v1/meal-plans/{id}/membership", "Accept an invitation to a plan", http.StatusOK, 404),
		func(ctx context.Context, in *planIn) (*planOut, error) {
			return mutation(ctx, in.Range, "", func(uid, from, to string) (Plan, error) {
				p, err := svc.Accept(ctx, uid, in.ID)
				return inRange(ctx, uid, from, to, p, err)
			})
		})

	huma.Register(api, op("leave-meal-plan", http.MethodDelete, "/api/v1/meal-plans/{id}/membership", "Leave a plan or decline its invitation", http.StatusNoContent, 404, 422),
		func(ctx context.Context, in *planID) (*emptyOut, error) {
			return command(ctx, func(uid string) error { return svc.Leave(ctx, uid, in.ID) })
		})
}

// invalidField is a 422 naming the body field when there is one.
func invalidField(msg, field string) error {
	d := &huma.ErrorDetail{Message: msg}
	if field != "" {
		d.Location = "body." + field
	}
	return huma.Error422UnprocessableEntity("validation failed", d)
}
