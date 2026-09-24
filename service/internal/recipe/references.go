package recipe

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// IngredientRef ties one word of a step's text to one ingredient of the recipe.
//
// The anchor is deliberately made of names rather than indices. Ingredients have
// no stable identity - writeChildren mints a fresh UUIDv7 for every one of them
// on every save - and a positional anchor would silently point at a neighbor
// once anything above it is deleted. A name that no longer resolves leaves the
// word as the word the author wrote, so every failure mode is "no reference"
// rather than "wrong reference".
//
// Word and Ingredient are separate so a sentence can say "Fleisch" while the
// list says "Rinderbraten". Group breaks the tie between two entries of the same
// name; nil means the group without a name.
type IngredientRef struct {
	Word           string  `json:"word" minLength:"1" maxLength:"120" doc:"The word in the step's text that names the ingredient"`
	GroupName      *string `json:"groupName" maxLength:"60" nullable:"true" doc:"Name of the ingredient group, null for the unnamed group"`
	IngredientName string  `json:"ingredientName" minLength:"1" maxLength:"120" doc:"Name of the ingredient this word points at"`
}

// Step is one preparation step and the ingredients its text points at.
type Step struct {
	Text       string          `json:"text" minLength:"1" maxLength:"2000"`
	References []IngredientRef `json:"references" maxItems:"30" required:"false"`
}

// RefError says which reference could not be resolved and why. The handler turns
// it into a 422 whose location names the exact JSON path.
type RefError struct {
	Step  int
	Ref   int
	Field string
	Msg   string
}

func (e *RefError) Error() string {
	return fmt.Sprintf("steps[%d].references[%d].%s: %s", e.Step, e.Ref, e.Field, e.Msg)
}

// Location is the RFC 9457 pointer huma expects.
func (e *RefError) Location() string {
	return fmt.Sprintf("body.steps[%d].references[%d].%s", e.Step, e.Ref, e.Field)
}

// refTarget is where a reference landed, as positions within the submitted
// document. writeChildren turns these into the ids it has just minted.
type refTarget struct {
	Group      int
	Ingredient int
}

// resolveRefs maps every reference onto the ingredient it names.
//
// Ambiguity is refused rather than resolved: two groups sharing a name, two
// unnamed groups, or the same ingredient name twice within one group all yield
// a RefError. Taking the first match would show a quantity that is not the one
// the author meant, and a wrong quantity is worse than none.
func resolveRefs(groups []IngredientGroup, steps []Step) (map[[2]int]refTarget, error) {
	out := make(map[[2]int]refTarget)
	for si, step := range steps {
		seen := make(map[string]struct{}, len(step.References))
		for ri, ref := range step.References {
			word := strings.TrimSpace(ref.Word)
			if word == "" {
				return nil, &RefError{Step: si, Ref: ri, Field: "word", Msg: "must not be empty"}
			}
			if _, dup := seen[word]; dup {
				return nil, &RefError{Step: si, Ref: ri, Field: "word", Msg: fmt.Sprintf("%q is referenced twice in this step", word)}
			}
			seen[word] = struct{}{}
			if !containsWord(step.Text, word) {
				return nil, &RefError{Step: si, Ref: ri, Field: "word", Msg: fmt.Sprintf("%q does not occur in the step's text", word)}
			}

			gi, err := findGroup(groups, ref.GroupName)
			if err != nil {
				return nil, &RefError{Step: si, Ref: ri, Field: "groupName", Msg: err.Error()}
			}
			ii, err := findIngredient(groups[gi], strings.TrimSpace(ref.IngredientName))
			if err != nil {
				return nil, &RefError{Step: si, Ref: ri, Field: "ingredientName", Msg: err.Error()}
			}
			out[[2]int{si, ri}] = refTarget{Group: gi, Ingredient: ii}
		}
	}
	return out, nil
}

func findGroup(groups []IngredientGroup, name *string) (int, error) {
	want := ""
	if name != nil {
		want = strings.TrimSpace(*name)
	}
	found := -1
	for i, g := range groups {
		have := ""
		if g.Name != nil {
			have = strings.TrimSpace(*g.Name)
		}
		if have != want {
			continue
		}
		if found >= 0 {
			if want == "" {
				return 0, fmt.Errorf("the recipe has more than one unnamed group, so this reference is not unique")
			}
			return 0, fmt.Errorf("more than one group is called %q", want)
		}
		found = i
	}
	if found < 0 {
		if want == "" {
			return 0, fmt.Errorf("the recipe has no unnamed group")
		}
		return 0, fmt.Errorf("no group called %q", want)
	}
	return found, nil
}

func findIngredient(group IngredientGroup, name string) (int, error) {
	found := -1
	for i, ing := range group.Ingredients {
		if strings.TrimSpace(ing.Name) != name {
			continue
		}
		if found >= 0 {
			return 0, fmt.Errorf("%q appears more than once in this group, so this reference is not unique", name)
		}
		found = i
	}
	if found < 0 {
		return 0, fmt.Errorf("no ingredient %q in this group", name)
	}
	return found, nil
}

// containsWord reports whether word occurs in text as a whole word.
//
// Go's regexp \b is ASCII-only, which would break on "Öl" and "Äpfel" - not an
// edge case in a German recipe book - so the boundaries are checked against
// unicode.IsLetter and IsDigit instead.
//
// Known divergence from the frontend, recorded rather than removed:
// unicode.IsDigit is \p{Nd} (decimal digits), while wordPattern in
// frontend/src/lib/recipe/references.ts uses \p{N}, which also covers "½" and
// "Ⅷ". The two therefore disagree about a word boundary next to one of those,
// and only about that. It cannot produce a wrong quantity in either direction:
// here a disagreement rejects the reference with a 422, and there it leaves the
// word unannotated - both are "no reference", which is the only failure mode
// this feature allows. Aligning them would mean changing what one side accepts,
// for characters no recipe in the corpus contains.
func containsWord(text, word string) bool {
	if word == "" {
		return false
	}
	for off := 0; off <= len(text)-len(word); {
		i := strings.Index(text[off:], word)
		if i < 0 {
			return false
		}
		i += off
		if !alnumBefore(text, i) && !alnumAt(text, i+len(word)) {
			return true
		}
		off = i + 1
	}
	return false
}

func alnumBefore(s string, i int) bool {
	if i <= 0 {
		return false
	}
	r, _ := utf8.DecodeLastRuneInString(s[:i])
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

func alnumAt(s string, i int) bool {
	if i >= len(s) {
		return false
	}
	r, _ := utf8.DecodeRuneInString(s[i:])
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}
