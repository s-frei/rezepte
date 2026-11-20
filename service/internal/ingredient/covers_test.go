package ingredient_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/s-frei/rezepte/service/internal/ingredient"
)

func TestCovers(t *testing.T) {
	raw, err := os.ReadFile("testdata/covers.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Have, HaveKey, Need, NeedKey string
		Covers                       bool
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if got := ingredient.Key(r.Have); got != r.HaveKey {
			t.Errorf("Key(%q) = %q, want %q", r.Have, got, r.HaveKey)
		}
		if got := ingredient.Key(r.Need); got != r.NeedKey {
			t.Errorf("Key(%q) = %q, want %q", r.Need, got, r.NeedKey)
		}
		if got := ingredient.Covers(r.HaveKey, r.NeedKey); got != r.Covers {
			t.Errorf("Covers(%q, %q) = %v, want %v", r.Have, r.Need, got, r.Covers)
		}
	}
}

func TestIsWater(t *testing.T) {
	for in, want := range map[string]bool{"wasser": true, "Wasser": true, "water": true, "Mineralwasser": false} {
		if got := ingredient.IsWater(ingredient.Key(in)); got != want {
			t.Errorf("IsWater(Key(%q)) = %v, want %v", in, got, want)
		}
	}
}
