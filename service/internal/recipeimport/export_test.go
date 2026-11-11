package recipeimport

// Test hooks: the language tables and the parsing steps that take one, for
// the tests in recipeimport_test. A new language needs no hook of its own:
// LanguageByTag finds every registered table.
var (
	AnyLanguage          = anyLanguage
	Languages            = languages
	LanguageByTag        = byTag
	ParseLineWith        = parseLine
	ReadTextWith         = readText
	DetectLanguage       = detectLanguage
	StripAuthorFromTitle = stripAuthorFromTitle
	RemovePromo          = removePromo
	SplitTrailingNote    = splitTrailingNote
)

// Tag returns a table's language tag.
func (l *language) Tag() string { return l.tag }

// ReadDuration reads a time value; see readDuration.
var ReadDuration = readDuration

// StructureWords returns the table's headings and detail keys by field.
func (l *language) StructureWords() map[string][]string {
	words := map[string][]string{"ingredientHeadings": l.ingredientHeadings, "stepHeadings": l.stepHeadings}
	for k, f := range l.detailKeys {
		words[f+"Keys"] = append(words[f+"Keys"], k)
	}
	return words
}
