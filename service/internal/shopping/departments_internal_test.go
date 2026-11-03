package shopping

import (
	"strings"
	"testing"
)

func TestEachWordHasOneDepartment(t *testing.T) {
	seen := map[string]Department{}
	for dep, list := range words {
		for _, w := range strings.Split(list, ",") {
			k := NameKey(w)
			if prev, ok := seen[k]; ok && prev != dep {
				t.Errorf("%q is in both %s and %s", k, prev, dep)
			}
			seen[k] = dep
		}
	}
}

func TestShortWordsDoNotMatchBySuffix(t *testing.T) {
	for _, w := range []string{"graham", "goat", "toast"} {
		if got := DefaultDepartment(NameKey(w)); got != Other {
			t.Errorf("%s → %s", w, got)
		}
	}
}
