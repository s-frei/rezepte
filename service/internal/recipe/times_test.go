package recipe

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateTimesAcceptsPlainAndRange(t *testing.T) {
	steps := []Step{
		{Text: "Etwa 90 Minuten schmoren.", Times: []StepTime{{Phrase: "90 Minuten", Seconds: 5400}}},
		{Text: "Den Teig 20–25 Minuten ruhen lassen.", Times: []StepTime{{Phrase: "20–25 Minuten", Seconds: 1200, MaxSeconds: new(1500)}}},
		{Text: "½ Stunde ziehen lassen.", Times: []StepTime{{Phrase: "½ Stunde", Seconds: 1800}}},
		{Text: "15 Minuten", Times: []StepTime{{Phrase: "15 Minuten", Seconds: 900}}},
	}
	if err := validateTimes(steps); err != nil {
		t.Fatalf("validateTimes: %v", err)
	}
}

func TestValidateTimesRejects(t *testing.T) {
	cases := map[string]struct {
		step  Step
		field string
	}{
		"phrase not in text":      {Step{Text: "Schmoren.", Times: []StepTime{{Phrase: "90 Minuten", Seconds: 5400}}}, "phrase"},
		"phrase inside a word":    {Step{Text: "190 Minuten.", Times: []StepTime{{Phrase: "90 Minuten", Seconds: 5400}}}, "phrase"},
		"phrase abuts a fraction": {Step{Text: "1½ Stunden backen.", Times: []StepTime{{Phrase: "1", Seconds: 3600}}}, "phrase"},
		"phrase without number":   {Step{Text: "Über Nacht kühlen.", Times: []StepTime{{Phrase: "Über Nacht", Seconds: 43200}}}, "phrase"},
		"duplicate phrase":        {Step{Text: "10 Minuten, dann 10 Minuten.", Times: []StepTime{{Phrase: "10 Minuten", Seconds: 600}, {Phrase: "10 Minuten", Seconds: 600}}}, "phrase"},
		"max not above min":       {Step{Text: "20–25 Minuten.", Times: []StepTime{{Phrase: "20–25 Minuten", Seconds: 1200, MaxSeconds: new(1200)}}}, "maxSeconds"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			err := validateTimes([]Step{c.step})
			var re *RefError
			if !errors.As(err, &re) {
				t.Fatalf("err = %v, want *RefError", err)
			}
			if re.List != "times" || re.Field != c.field {
				t.Errorf("got %s.%s, want times.%s", re.List, re.Field, c.field)
			}
			if !strings.HasPrefix(re.Location(), "body.steps[0].times[") {
				t.Errorf("location = %s", re.Location())
			}
		})
	}
}
