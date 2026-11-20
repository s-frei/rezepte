package shopping

import (
	"strings"
	"testing"

	"github.com/s-frei/rezepte/service/internal/ingredient"
)

func TestEachWordHasOneDepartment(t *testing.T) {
	seen := map[string]Department{}
	for dep, list := range words {
		for _, w := range strings.Split(list, ",") {
			k := ingredient.Key(w)
			if prev, ok := seen[k]; ok && prev != dep {
				t.Errorf("%q is in both %s and %s", k, prev, dep)
			}
			seen[k] = dep
		}
	}
}

func TestShortWordsDoNotMatchBySuffix(t *testing.T) {
	for _, w := range []string{"graham", "goat", "toast"} {
		if got := DefaultDepartment(ingredient.Key(w)); got != Other {
			t.Errorf("%s → %s", w, got)
		}
	}
}
