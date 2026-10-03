package recipe

import (
	"fmt"
	"strings"
	"unicode"
)

// StepTime marks a duration in a step's text. Like IngredientRef it is
// anchored by the text it stands on, never by a position, and it carries the
// duration as seconds so nothing downstream has to parse a language. The
// maximum is one week: longer than any recipe waits, short enough that a typo
// of an extra zero is refused rather than stored.
type StepTime struct {
	Phrase     string `json:"phrase" minLength:"1" maxLength:"60" doc:"The duration as it stands in the step's text, e.g. \"90 Minuten\""`
	Seconds    int    `json:"seconds" minimum:"1" maximum:"604800" doc:"The duration in seconds; for a range, its lower end"`
	MaxSeconds *int   `json:"maxSeconds,omitempty" minimum:"1" maximum:"604800" required:"false" doc:"The upper end of a range in seconds, absent for a single duration"`
}

// validateTimes checks every step's times: each phrase stands in the text as
// whole words, holds a number, and occurs once per step; a range runs
// upward. The huma tags check lengths and bounds. "Über Nacht" has no number
// and is refused on purpose - only what the author wrote as a number becomes
// a time a cook can rely on.
func validateTimes(steps []Step) error {
	for si, step := range steps {
		seen := make(map[string]struct{}, len(step.Times))
		for ti, tm := range step.Times {
			fail := func(field, msg string) error {
				return &RefError{List: "times", Step: si, Ref: ti, Field: field, Msg: msg}
			}
			phrase := strings.TrimSpace(tm.Phrase)
			if !strings.ContainsFunc(phrase, unicode.IsNumber) {
				return fail("phrase", fmt.Sprintf("%q holds no number", phrase))
			}
			if _, dup := seen[phrase]; dup {
				return fail("phrase", fmt.Sprintf("%q is marked twice in this step", phrase))
			}
			seen[phrase] = struct{}{}
			if !containsWord(step.Text, phrase) {
				return fail("phrase", fmt.Sprintf("%q does not occur in the step's text", phrase))
			}
			if tm.MaxSeconds != nil && *tm.MaxSeconds <= tm.Seconds {
				return fail("maxSeconds", "must be above seconds")
			}
		}
	}
	return nil
}
