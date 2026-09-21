// Package recipe manages recipes: creation, editing, deletion and lookup,
// including tag normalisation, slug assignment and full-text search
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
	Steps            []string          `json:"steps" maxItems:"50" minLength:"1" maxLength:"2000"`
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

// Recipe is a stored recipe: an Input plus the fields the service assigns.
type Recipe struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	Input
	CoverImageID *string   `json:"coverImageId" nullable:"true"`
	Images       []Image   `json:"images"`
	CreatedBy    string    `json:"createdBy"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedBy    string    `json:"updatedBy"`
	UpdatedAt    time.Time `json:"updatedAt"`
	// The display names behind CreatedBy and UpdatedBy, and the palette token
	// each of those people chose. They travel with the recipe because the card
	// paints the author circles with them and user management is admin-only -
	// a member could resolve neither the ids nor the colours themselves.
	CreatedByName  string `json:"createdByName"`
	CreatedByColor string `json:"createdByColor"`
	UpdatedByName  string `json:"updatedByName"`
	UpdatedByColor string `json:"updatedByColor"`
	// Favourite reports whether the caller has starred this recipe. It is
	// endpoint-dependent, not a property (*Service).ByID or (*Service).BySlug
	// fill in themselves: only the get-recipe and get-recipe-by-slug handler
	// operations populate it, each by calling fillFavourite (handler.go)
	// after loading the Recipe. create-recipe and update-recipe return it as
	// the zero value false unconditionally - Create's is accurate (nothing
	// can have favourited a recipe that didn't exist a moment ago), Update's
	// is not (an existing favourite is silently dropped from the response).
	// A future caller of ByID/BySlug - a new handler operation, say - gets
	// false the same way unless it also calls fillFavourite or
	// (*Service).IsFavourite itself.
	Favourite bool `json:"favourite"`
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
	Favourite    bool      `json:"favourite"`
	// Who wrote the recipe and who last changed it, by display name and
	// colour. The card shows them as initials; the same reasoning as on
	// Recipe applies - user management is admin-only, so the ids alone would
	// be useless to the caller.
	CreatedByName  string `json:"createdByName"`
	CreatedByColor string `json:"createdByColor"`
	UpdatedByName  string `json:"updatedByName"`
	UpdatedByColor string `json:"updatedByColor"`
}

// TagCount is a tag name paired with how many recipes currently use it.
type TagCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// AuthorCount is an author paired with how many recipes they wrote. Name is
// the username, because that is what the ?author= filter matches and what a
// filter link therefore has to survive a rename with. DisplayName and Color
// are what the facet shows.
type AuthorCount struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Color       string `json:"color"`
	Count       int    `json:"count"`
}
