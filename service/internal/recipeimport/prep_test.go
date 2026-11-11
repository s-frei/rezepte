package recipeimport_test

import (
	"slices"
	"testing"

	"github.com/s-frei/rezepte/service/internal/recipeimport"
)

func TestPrepLines(t *testing.T) {
	type L = recipeimport.PrepLine
	for _, tc := range []struct {
		name, in string
		want     []L
	}{
		{"fence with info string", "Hier:\n```text\nLinsen-Dal\n- 1 EL Öl\n```\nGuten Appetit!", []L{{"Linsen-Dal", false, 3}, {"1 EL Öl", true, 4}}},
		{"no fence keeps all", "A\n\nB", []L{{"A", false, 1}, {"", false, 2}, {"B", false, 3}}},
		{"checkbox", "- [x] 2 l Milch", []L{{"2 l Milch", true, 1}}},
		{"bold whole line", "**Für den Teig:**", []L{{"Für den Teig:", false, 1}}},
		{"heading", "## Zutaten", []L{{"Zutaten", false, 1}}},
		{"minus temperature stays", "-5 °C kühlen", []L{{"-5 °C kühlen", false, 1}}},
		{"star word stays", "*optional", []L{{"*optional", false, 1}}},
		{"crlf and spaces", "•  1   EL Öl\r\n__Teig__", []L{{"1 EL Öl", true, 1}, {"Teig", false, 2}}},
	} {
		if got := recipeimport.PrepLines(tc.in); !slices.Equal(got, tc.want) {
			t.Errorf("%s: PrepLines(%q) = %+v; want %+v", tc.name, tc.in, got, tc.want)
		}
	}
}
