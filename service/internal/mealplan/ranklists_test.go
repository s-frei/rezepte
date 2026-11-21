package mealplan

import "testing"

func TestRankListsCountsEntriesMostFirst(t *testing.T) {
	a, b := LinkedList{ID: "a", Name: "Getränke"}, LinkedList{ID: "b", Name: "Markt"}
	got := rankLists([][]LinkedList{{a}, {b}, {a, b}, {b}, nil})
	if len(got) != 2 || got[0].ID != "b" || got[0].Entries != 3 || got[1].ID != "a" || got[1].Entries != 2 {
		t.Fatalf("rankLists = %+v, want Markt (3) before Getränke (2)", got)
	}
}
