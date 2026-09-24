// Package recipe manages recipes: creation, editing, deletion and lookup,
// including tag normalization, slug assignment and full-text search
// indexing.
package recipe

import (
	"errors"
	"time"
)

// ErrNotFound is returned when a recipe id or slug has no match.
var ErrNotFound = errors.New("recipe not found")

// Ingredient is a single ingredient line within an IngredientGroup.
type Ingredient struct {
	Quantity *float64 `json:"quantity" minimum:"0" nullable:"true"`
	Unit     *string  `json:"unit" maxLength:"20" nullable:"true"`
	Name     string   `json:"name" minLength:"1" maxLength:"120"`
	Note     *string  `json:"note" maxLength:"200" nullable:"true"`
}

// IngredientGroup is a named (or unnamed) group of ingredients, used to
// split a recipe's shopping list into sections such as "Sauce" or "Teig".
type IngredientGroup struct {
	Name        *string      `json:"name" maxLength:"60" nullable:"true"`
	Ingredients []Ingredient `json:"ingredients" maxItems:"100"`
}

// Input is the recipe payload accepted by Service.Create and Service.Update.
//
// SourceURL carries a pattern on top of format:"uri" because a URI only
// has to have some scheme: without it "javascript:alert(1)" validates and
// the frontend renders it as a link the user can click.
type Input struct {
	Title            string            `json:"title" minLength:"1" maxLength:"200"`
	Description      string            `json:"description" maxLength:"2000"`
	Servings         int               `json:"servings" minimum:"1" maximum:"99"`
	PrepMinutes      *int              `json:"prepMinutes" minimum:"0" maximum:"1440" nullable:"true"`
	CookMinutes      *int              `json:"cookMinutes" minimum:"0" maximum:"1440" nullable:"true"`
	SourceURL        *string           `json:"sourceUrl" maxLength:"500" format:"uri" pattern:"^https?://" nullable:"true"`
	Tags             []string          `json:"tags" maxItems:"20" minLength:"1" maxLength:"40"`
	IngredientGroups []IngredientGroup `json:"ingredientGroups" minItems:"1" maxItems:"20"`
	Steps            []Step            `json:"steps" maxItems:"50"`
	// EditPolicy is who besides the author and admins may edit. On create,
	// empty means "default". On update, empty keeps the stored policy, so a
	// client that does not know the field cannot reset or trip it.
	EditPolicy Policy `json:"editPolicy,omitempty" enum:"default,open,locked" required:"false"`
}

// Image is a photo attached to a recipe. Width and height describe the
// original JPEG variant on disk (longest side at most 2400 px); the
// variants are served at /images/{recipeId}/{id}/{thumb|detail|original}.jpg.
type Image struct {
	ID       string `json:"id"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Position int    `json:"position"`
}

// Person is somebody a recipe names - who wrote it, who last changed it -
// with what the app shows of them. The whole of it travels with the recipe
// because user management is admin-only: a member could resolve neither the
// id nor the color themselves.
type Person struct {
	ID          string `json:"id"`
	Username    string `json:"username" doc:"Login name, which ?author= filters by"`
	DisplayName string `json:"displayName"`
	Color       string `json:"color"`
}

// Recipe is a stored recipe: an Input plus the fields the service assigns.
type Recipe struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	Input
	CoverImageID *string   `json:"coverImageId" nullable:"true"`
	Images       []Image   `json:"images"`
	CreatedAt    time.Time `json:"createdAt"`
	CreatedBy    Person    `json:"createdBy"`
	UpdatedAt    time.Time `json:"updatedAt"`
	UpdatedBy    Person    `json:"updatedBy"`
	// Favorite reports whether the caller has starred this recipe. It is
	// caller-dependent, not a property (*Service).ByID or (*Service).BySlug
	// fill in themselves: every handler operation that returns a Recipe
	// populates it by calling fillCaller (handler.go) after loading it, and
	// every mcpserver tool that returns one does the same through its
	// caller.fill. A future caller of ByID/BySlug - a new handler operation,
	// say - gets false unless it also calls fillCaller or
	// (*Service).IsFavorite itself.
	Favorite bool `json:"favorite"`
	// Locked is the effective state: the recipe's policy resolved against
	// the household default. The Can* fields are for the caller and, like
	// Favorite, are filled by the handler (FillAccess), not by ByID/BySlug.
	Locked          bool `json:"locked"`
	CanEdit         bool `json:"canEdit"`
	CanDelete       bool `json:"canDelete"`
	CanChangePolicy bool `json:"canChangePolicy"`
}

// Card is the summary of a recipe shown in listings.
type Card struct {
	ID           string    `json:"id"`
	Slug         string    `json:"slug"`
	Title        string    `json:"title"`
	Tags         []string  `json:"tags"`
	TotalMinutes *int      `json:"totalMinutes" nullable:"true"`
	CoverImageID *string   `json:"coverImageId" nullable:"true"`
	UpdatedAt    time.Time `json:"updatedAt"`
	Favorite     bool      `json:"favorite"`
	// Who wrote the recipe and who last changed it, in the same shape as on
	// Recipe. The card shows them as initials.
	CreatedBy Person `json:"createdBy"`
	UpdatedBy Person `json:"updatedBy"`
}

// TagCount is a tag name paired with how many recipes currently use it.
type TagCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// AuthorCount is an author paired with how many recipes they wrote. Username
// is what the ?author= filter matches and what a filter link therefore has to
// survive a rename with. DisplayName and Color are what the facet shows.
type AuthorCount struct {
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Color       string `json:"color"`
	Count       int    `json:"count"`
}
