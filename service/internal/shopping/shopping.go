package shopping

import (
	"errors"
	"time"
)

// ErrNotFound is returned for a list that does not exist or is not
// visible to the caller; the two look the same on purpose.
var ErrNotFound = errors.New("shopping list not found")

// ErrRecipeNotFound is returned when the recipe to add does not exist.
var ErrRecipeNotFound = errors.New("recipe not found")

// ErrInvalid is returned, wrapped with a field message, for input that fails
// validation.
var ErrInvalid = errors.New("invalid input")

// List is a shopping list with its recipe entries and items. OpenCount and
// DoneCount count rows (items merged by NameKey), not items.
type List struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedBy string    `json:"createdBy"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	// Revision counts the list's changes; the live stream sends it too.
	Revision  int64   `json:"revision"`
	OpenCount int     `json:"openCount"`
	DoneCount int     `json:"doneCount"`
	Entries   []Entry `json:"entries"`
	Items     []Item  `json:"items"`
	// Owner is the creator. Members are the accepted people, oldest first;
	// Pending are invitees still waiting and is empty unless the caller is
	// the creator.
	Owner   ListPerson   `json:"owner"`
	Members []ListPerson `json:"members"`
	Pending []ListPerson `json:"pending"`
}

// Entry is a recipe copied onto a list. RecipeID, RecipeSlug and CoverImageID
// are nil once the recipe is deleted; Title stays.
type Entry struct {
	ID           string  `json:"id"`
	RecipeID     *string `json:"recipeId" nullable:"true"`
	RecipeSlug   *string `json:"recipeSlug" nullable:"true"`
	CoverImageID *string `json:"coverImageId" nullable:"true"`
	Title        string  `json:"title"`
	BaseServings int     `json:"baseServings"`
	Servings     int     `json:"servings"`
}

// Item is one line of a list. Quantity is already scaled to the entry's
// servings and rounded; AddedBy is set for free items only.
type Item struct {
	ID         string      `json:"id"`
	EntryID    *string     `json:"entryId" nullable:"true"`
	Name       string      `json:"name"`
	NameKey    string      `json:"nameKey"`
	Quantity   *float64    `json:"quantity" nullable:"true"`
	Unit       *string     `json:"unit" nullable:"true"`
	Note       *string     `json:"note" nullable:"true"`
	Group      *string     `json:"group" nullable:"true"`
	Department Department  `json:"department" enum:"produce,bakery,chilled,meat,pantry,frozen,drinks,household,other"`
	AddedBy    *ListPerson `json:"addedBy"`
	Checked    bool        `json:"checked"`
	// CheckedBy is who ticked it; nil while open, or once that account is gone.
	CheckedBy *ListPerson `json:"checkedBy"`
}

// ListPerson is a person shown on a list: its creator, a member, an invitee,
// whoever changed it last or whoever added a free item.
type ListPerson struct {
	ID          string  `json:"id"`
	DisplayName string  `json:"displayName"`
	Color       string  `json:"color"`
	AvatarID    *string `json:"avatarId" nullable:"true"`
}

// Summary is a list as shown in the overview; its counts are rows, as List's.
type Summary struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	OpenCount int             `json:"openCount"`
	DoneCount int             `json:"doneCount"`
	Recipes   []SummaryRecipe `json:"recipes"`
	UpdatedAt time.Time       `json:"updatedAt"`
	Owner     ListPerson      `json:"owner"`
	Members   []ListPerson    `json:"members"`
	// UpdatedBy is nil once the account of whoever changed the list last is gone.
	UpdatedBy *ListPerson `json:"updatedBy"`
}

// Invitation is a list the caller was invited to and has not accepted yet; it
// shows no items or entries.
type Invitation struct {
	ListID    string     `json:"listId"`
	ListName  string     `json:"listName"`
	From      ListPerson `json:"from"`
	OpenCount int        `json:"openCount"`
	InvitedAt time.Time  `json:"invitedAt"`
}

// SummaryRecipe is a recipe on a list, with its servings.
type SummaryRecipe struct {
	Title    string `json:"title"`
	Servings int    `json:"servings"`
	// RecipeID is nil once the recipe is deleted; the app uses it to say a recipe is on a list already.
	RecipeID *string `json:"recipeId"`
}

// IngredientPick points at one ingredient of a recipe by group and position.
type IngredientPick struct {
	Group int `json:"group" minimum:"0"`
	Index int `json:"index" minimum:"0"`
}
