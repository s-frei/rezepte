package recipe

import (
	"errors"
	"testing"
)

func TestValidatePhotos(t *testing.T) {
	set := map[string]struct{}{"img1": {}, "img2": {}}
	text := "Nudeln 10 Minuten kochen, dann ins kochende Salzwasser schaben."
	cases := map[string]struct {
		step  Step
		set   map[string]struct{}
		field string // "" = valid
		ref   int    // index of the refused photo
	}{
		"ingredient word inside": {Step{Text: "Das Salzwasser aufkochen.", References: []IngredientRef{{Word: "Salzwasser"}}, Photos: []StepPhoto{{Phrase: "Das Salzwasser aufkochen", ImageID: "img1"}}}, set, "", 0},
		"trimmed":                {Step{Text: text, Photos: []StepPhoto{{Phrase: "  Nudeln ", ImageID: "img1"}}}, set, "", 0},
		"whitespace only":        {Step{Text: text, Photos: []StepPhoto{{Phrase: "   ", ImageID: "img1"}}}, set, "phrase", 0},
		"not in text":            {Step{Text: text, Photos: []StepPhoto{{Phrase: "Reis", ImageID: "img1"}}}, set, "phrase", 0},
		"twice":                  {Step{Text: text, Photos: []StepPhoto{{Phrase: "Nudeln", ImageID: "img1"}, {Phrase: " Nudeln", ImageID: "img2"}}}, set, "phrase", 1},
		"overlaps a time":        {Step{Text: text, Times: []StepTime{{Phrase: "10 Minuten", Seconds: 600}}, Photos: []StepPhoto{{Phrase: "10 Minuten kochen", ImageID: "img1"}}}, set, "phrase", 0},
		"overlaps another photo": {Step{Text: text, Photos: []StepPhoto{{Phrase: "ins kochende Salzwasser", ImageID: "img1"}, {Phrase: "Salzwasser schaben", ImageID: "img2"}}}, set, "phrase", 1},
		"unknown image":          {Step{Text: text, Photos: []StepPhoto{{Phrase: "Nudeln", ImageID: "nope"}}}, set, "imageId", 0},
		"nil set (create)":       {Step{Text: text, Photos: []StepPhoto{{Phrase: "Nudeln", ImageID: "img1"}}}, nil, "imageId", 0},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			err := ValidatePhotos([]Step{c.step}, c.set)
			if c.field == "" {
				if err != nil {
					t.Fatalf("err = %v, want nil", err)
				}
				return
			}
			var re *RefError
			if !errors.As(err, &re) {
				t.Fatalf("err = %v, want *RefError", err)
			}
			if re.List != "photos" || re.Field != c.field || re.Step != 0 || re.Ref != c.ref {
				t.Errorf("got %+v, want photos.%s", re, c.field)
			}
		})
	}
}
