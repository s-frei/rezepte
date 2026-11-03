package shopping

import (
	"errors"
	"time"
)

// ErrNotFound is returned for a list that does not exist or is not the
// caller's; the two look the same on purpose.
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
	OpenCount int       `json:"openCount"`
	DoneCount int       `json:"doneCount"`
	Entries   []Entry   `json:"entries"`
	Items     []Item    `json:"items"`
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
	ID         string     `json:"id"`
	EntryID    *string    `json:"entryId" nullable:"true"`
	Name       string     `json:"name"`
	NameKey    string     `json:"nameKey"`
	Quantity   *float64   `json:"quantity" nullable:"true"`
	Unit       *string    `json:"unit" nullable:"true"`
	Note       *string    `json:"note" nullable:"true"`
	Group      *string    `json:"group" nullable:"true"`
	Department Department `json:"department" enum:"produce,bakery,chilled,meat,pantry,frozen,drinks,household,other"`
	AddedBy    *Adder     `json:"addedBy"`
	Checked    bool       `json:"checked"`
}

// Adder is who added a free item.
type Adder struct {
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
}

// SummaryRecipe is a recipe on a list, with its servings.
type SummaryRecipe struct {
	Title    string `json:"title"`
	Servings int    `json:"servings"`
}

// IngredientPick points at one ingredient of a recipe by group and position.
type IngredientPick struct {
	Group int `json:"group" minimum:"0"`
	Index int `json:"index" minimum:"0"`
}
