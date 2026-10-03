package recipeimport

// Page is what a reader found, before it becomes a recipe.Input.
type Page struct {
	Title       string
	Description string
	Servings    int // 0 = not found
	PrepMinutes *int
	CookMinutes *int
	Ingredients []string // raw lines
	Steps       []string
	Keywords    []string
	Authors     []string // names of every author; Build keeps them out of the tags
	PhotoURL    string   // absolute, "" = none
	SourceURL   string   // canonical or entered, "" for pasted text
	SourceName  string
	Language    string // tag of the table it was read with, "" = every table
}
