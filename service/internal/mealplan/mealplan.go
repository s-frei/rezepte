// Package mealplan stores meal plans: recipes and free texts on days of a
// week or on no day (the tray), shared like shopping lists.
package mealplan

import (
	"errors"
	"fmt"

	"github.com/s-frei/rezepte/service/internal/shopping"
)

// ErrNotFound is returned for a plan or entry that does not exist or is not
// visible to the caller; the two look the same on purpose.
var ErrNotFound = errors.New("meal plan not found")

// ErrInvalid is returned, wrapped with a field message, for input that fails
// validation.
var ErrInvalid = errors.New("invalid input")

// ErrRecipeNotFound is returned when the recipe to plan does not exist.
var ErrRecipeNotFound = errors.New("recipe not found")

// ErrPersonal is returned, wrapped together with ErrInvalid, when the caller
// tries to share, rename or delete their personal plan.
var ErrPersonal = errors.New("personal plan")

func invalid(msg string) error { return fmt.Errorf("%w: %s", ErrInvalid, msg) }

func personal() error { return fmt.Errorf("%w: %w", ErrInvalid, ErrPersonal) }

// Person is someone shown on a plan, as on a list.
type Person = shopping.ListPerson

// Plan is a plan with its entries between From and To (inclusive) and on the
// tray.
type Plan struct {
	ID string `json:"id"`
	// Name is nil for the personal plan; the client words it.
	Name     *string `json:"name" nullable:"true"`
	Personal bool    `json:"personal"`
	// Revision counts the plan's changes; the live stream sends it too.
	Revision int64 `json:"revision"`
	// Owner is the creator. Members are the accepted people, oldest first;
	// Pending are invitees still waiting and is empty unless the caller is
	// the creator.
	Owner   Person   `json:"owner"`
	Members []Person `json:"members"`
	Pending []Person `json:"pending"`
	From    string   `json:"from"`
	To      string   `json:"to"`
	// Entries are the tray first, then by day and position.
	Entries []PlanEntry `json:"entries"`
	// Unlisted counts the recipe entries on no list the caller can access.
	Unlisted int `json:"unlisted"`
	// Lists are the accessible lists the entries are on, most entries first.
	Lists []LinkedList `json:"lists"`
}

// PlanEntry is a recipe or a free text on a day (YYYY-MM-DD) or on the tray (Day
// nil). RecipeID, RecipeSlug and Photo are nil for free text and once the
// recipe is deleted; Title stays. Servings is nil for free text.
type PlanEntry struct {
	ID         string  `json:"id"`
	Day        *string `json:"day" nullable:"true"`
	Position   int     `json:"position"`
	RecipeID   *string `json:"recipeId" nullable:"true"`
	RecipeSlug *string `json:"recipeSlug" nullable:"true"`
	Title      string  `json:"title"`
	Servings   *int    `json:"servings" nullable:"true"`
	// Photo is the recipe's cover image id.
	Photo *string `json:"photo" nullable:"true"`
	// CreatedBy is nil once the account of whoever added it is gone.
	CreatedBy *Person `json:"createdBy"`
	// Lists are the lists the caller can access that the entry is on.
	Lists []LinkedList `json:"lists"`
}

// LinkedList is a shopping list a plan entry is on; OpenCount counts the
// list's open rows as the list overview does.
type LinkedList struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	OpenCount int    `json:"openCount"`
}

// PlanSummary is a plan as the picker shows it.
type PlanSummary struct {
	ID        string   `json:"id"`
	Name      *string  `json:"name" nullable:"true"`
	Personal  bool     `json:"personal"`
	Owner     Person   `json:"owner"`
	Members   []Person `json:"members"`
	UpdatedAt string   `json:"updatedAt"`
}

// PlanInvitation is a plan the caller was invited to and has not accepted yet.
type PlanInvitation struct {
	PlanID  string  `json:"planId"`
	Name    *string `json:"name" nullable:"true"`
	Inviter Person  `json:"inviter"`
}

// AddEntry is what Add plans: exactly one of RecipeID and Title. A recipe
// entry copies the recipe's title and starts at Servings, else the recipe's
// own; a free text has no servings. Day nil is the tray.
type AddEntry struct {
	Day      *string `json:"day,omitempty"`
	RecipeID *string `json:"recipeId,omitempty"`
	Title    *string `json:"title,omitempty"`
	Servings *int    `json:"servings,omitempty"`
}
