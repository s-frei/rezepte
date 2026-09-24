package recipe

import (
	"errors"
	"testing"
)

func groupsFixture() []IngredientGroup {
	gr, va := "Grütze", "Vanillesoße"
	return []IngredientGroup{
		{Name: &gr, Ingredients: []Ingredient{{Name: "Beeren"}, {Name: "Zucker"}, {Name: "Traubensaft"}}},
		{Name: &va, Ingredients: []Ingredient{{Name: "Milch"}, {Name: "Zucker"}}},
	}
}

func strptr(s string) *string { return &s }

func TestResolveRefsPicksTheNamedGroup(t *testing.T) {
	steps := []Step{
		{Text: "Saft mit Zucker aufkochen.", References: []IngredientRef{
			{Word: "Zucker", GroupName: strptr("Grütze"), IngredientName: "Zucker"},
		}},
		{Text: "Eigelb mit Zucker schlagen.", References: []IngredientRef{
			{Word: "Zucker", GroupName: strptr("Vanillesoße"), IngredientName: "Zucker"},
		}},
	}
	got, err := resolveRefs(groupsFixture(), steps)
	if err != nil {
		t.Fatalf("resolveRefs: %v", err)
	}
	if got[[2]int{0, 0}] != (refTarget{Group: 0, Ingredient: 1}) {
		t.Errorf("step 0 resolved to %+v, want group 0 ingredient 1", got[[2]int{0, 0}])
	}
	if got[[2]int{1, 0}] != (refTarget{Group: 1, Ingredient: 1}) {
		t.Errorf("step 1 resolved to %+v, want group 1 ingredient 1", got[[2]int{1, 0}])
	}
}

func TestResolveRefsAllowsADifferentWord(t *testing.T) {
	steps := []Step{{Text: "Saft mit Zucker aufkochen.", References: []IngredientRef{
		{Word: "Saft", GroupName: strptr("Grütze"), IngredientName: "Traubensaft"},
	}}}
	got, err := resolveRefs(groupsFixture(), steps)
	if err != nil {
		t.Fatalf("resolveRefs: %v", err)
	}
	if got[[2]int{0, 0}] != (refTarget{Group: 0, Ingredient: 2}) {
		t.Errorf("got %+v, want group 0 ingredient 2", got[[2]int{0, 0}])
	}
}

func TestResolveRefsRejectsWordNotInText(t *testing.T) {
	steps := []Step{{Text: "Alles verrühren.", References: []IngredientRef{
		{Word: "Zucker", GroupName: strptr("Grütze"), IngredientName: "Zucker"},
	}}}
	_, err := resolveRefs(groupsFixture(), steps)
	var refErr *RefError
	if !errors.As(err, &refErr) || refErr.Field != "word" {
		t.Fatalf("got %v, want a RefError on word", err)
	}
}

func TestResolveRefsRejectsUnknownIngredient(t *testing.T) {
	steps := []Step{{Text: "Mehl zugeben.", References: []IngredientRef{
		{Word: "Mehl", GroupName: strptr("Grütze"), IngredientName: "Mehl"},
	}}}
	_, err := resolveRefs(groupsFixture(), steps)
	var refErr *RefError
	if !errors.As(err, &refErr) || refErr.Field != "ingredientName" {
		t.Fatalf("got %v, want a RefError on ingredient", err)
	}
}

func TestResolveRefsRejectsDuplicateWordInOneStep(t *testing.T) {
	steps := []Step{{Text: "Zucker und Zucker.", References: []IngredientRef{
		{Word: "Zucker", GroupName: strptr("Grütze"), IngredientName: "Zucker"},
		{Word: "Zucker", GroupName: strptr("Vanillesoße"), IngredientName: "Zucker"},
	}}}
	_, err := resolveRefs(groupsFixture(), steps)
	var refErr *RefError
	if !errors.As(err, &refErr) || refErr.Field != "word" {
		t.Fatalf("got %v, want a RefError on word", err)
	}
}

func TestResolveRefsRefusesAmbiguousGroup(t *testing.T) {
	dup := "Teig"
	groups := []IngredientGroup{
		{Name: &dup, Ingredients: []Ingredient{{Name: "Salz"}}},
		{Name: &dup, Ingredients: []Ingredient{{Name: "Salz"}}},
	}
	steps := []Step{{Text: "Salz zugeben.", References: []IngredientRef{
		{Word: "Salz", GroupName: &dup, IngredientName: "Salz"},
	}}}
	_, err := resolveRefs(groups, steps)
	var refErr *RefError
	if !errors.As(err, &refErr) || refErr.Field != "groupName" {
		t.Fatalf("got %v, want a RefError on group", err)
	}
}

func TestResolveRefsRefusesAmbiguousIngredientInOneGroup(t *testing.T) {
	groups := []IngredientGroup{
		{Name: nil, Ingredients: []Ingredient{{Name: "Butter"}, {Name: "Butter"}}},
	}
	steps := []Step{{Text: "Butter schmelzen.", References: []IngredientRef{
		{Word: "Butter", GroupName: nil, IngredientName: "Butter"},
	}}}
	_, err := resolveRefs(groups, steps)
	var refErr *RefError
	if !errors.As(err, &refErr) || refErr.Field != "ingredientName" {
		t.Fatalf("got %v, want a RefError on ingredient", err)
	}
}

func TestContainsWordRespectsUnicodeBoundaries(t *testing.T) {
	cases := []struct {
		text, word string
		want       bool
	}{
		{"Fleisch in Öl anbraten.", "Öl", true},
		{"Äpfel schälen.", "Äpfel", true},
		{"Kartoffeln in Salzwasser kochen.", "Salz", false},
		{"Zwiebel- und Gurkenstreifen belegen.", "Zwiebel", true},
		{"Brötchen einweichen.", "Ei", false},
		{"Zwiebelöl kalt pressen.", "Zwiebel", false},                 // ö abuts the word on the right
		{"Röstzwiebel darüberstreuen.", "zwiebel", false},             // case-sensitive AND 'st' prefix abuts on the left
		{"Kartoffelöl für Salzwasser verwenden.", "Kartoffel", false}, // ö abuts on the right, strengthens first discriminator
	}
	for _, c := range cases {
		if got := containsWord(c.text, c.word); got != c.want {
			t.Errorf("containsWord(%q, %q) = %v, want %v", c.text, c.word, got, c.want)
		}
	}
}
