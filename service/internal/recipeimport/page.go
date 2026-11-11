package recipeimport

// Page is what a reader found, before it becomes a recipe.Input.
type Page struct {
	Title       string
	Description string
	Servings    int // 0 = not found
	PrepMinutes *int
	CookMinutes *int
	Groups      []PageGroup
	Steps       []string
	Keywords    []string // the page's own; Build keeps only those matching a tag
	Tags        []string // a "Tags:" line's; Build keeps every one
	Authors     []string // names of every author; Build keeps them out of the tags
	PhotoURL    string   // absolute, "" = none
	SourceURL   string   // canonical or entered, a "Source:" link in pasted text
	SourceName  string
	Language    string // tag of the table it was read with, "" = every table
}

// PageGroup is an ingredient group as written: its name ("" for none) and
// its raw lines.
type PageGroup struct {
	Name  string
	Lines []string
}
