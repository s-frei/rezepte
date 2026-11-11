package shopping_test

import (
	"testing"

	"github.com/s-frei/rezepte/service/internal/shopping"
)

func TestParseFree(t *testing.T) {
	cases := []struct {
		in   string
		q    *float64
		unit *string
		name string
		note *string
	}{
		{"Milch", nil, nil, "Milch", nil},
		{"2 l Milch", numPtr(2), strPtr("l"), "Milch", nil},
		{"1,5 kg Kartoffeln", numPtr(1.5), strPtr("kg"), "Kartoffeln", nil},
		{"3 Zitronen", numPtr(3), nil, "Zitronen", nil},
		{"  2 cups flour ", numPtr(2), strPtr("cups"), "flour", nil},
		{"500g Mehl", numPtr(500), strPtr("g"), "Mehl", nil},
		{"3Zitronen", numPtr(3), nil, "Zitronen", nil},
		{"2 Dosen Tomaten", numPtr(2), strPtr("Dosen"), "Tomaten", nil},
		{"3 Zehen Knoblauch", numPtr(3), strPtr("Zehen"), "Knoblauch", nil},
		{"3 Gurken", numPtr(3), nil, "Gurken", nil},
		{"2 cup flour", numPtr(2), strPtr("cup"), "flour", nil},
		{"1 fl oz rum", numPtr(1), strPtr("fl oz"), "rum", nil},
		{"Basilikum im Topf, nicht geschnitten – der hält länger", nil, nil, "Basilikum im Topf", strPtr("nicht geschnitten – der hält länger")},
		{"Milch - fettarm", nil, nil, "Milch", strPtr("fettarm")},
		{"500 g Rinderhackfleisch (vom Metzger)", numPtr(500), strPtr("g"), "Rinderhackfleisch", strPtr("vom Metzger")},
		{"Mehl (Type 405), fein", nil, nil, "Mehl (Type 405)", strPtr("fein")},
		{"Self-raising flour", nil, nil, "Self-raising flour", nil},
		{"(Notiz)", nil, nil, "(Notiz)", nil},
	}
	for _, c := range cases {
		it := shopping.ParseFree(c.in)
		q, u, n, note := it.Quantity, it.Unit, it.Name, it.Note
		if !sameNum(q, c.q) || !sameStr(u, c.unit) || n != c.name || !sameStr(note, c.note) {
			t.Errorf("%q → %v %v %q %v", c.in, q, u, n, note)
		}
	}
}

func numPtr(v float64) *float64 { return &v }
func strPtr(v string) *string   { return &v }
func sameNum(a, b *float64) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}
func sameStr(a, b *string) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}
