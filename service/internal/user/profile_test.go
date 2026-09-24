package user

import (
	"errors"
	"strings"
	"testing"
)

func TestParseColor(t *testing.T) {
	for _, c := range Colors {
		got, err := ParseColor(string(c))
		if err != nil || got != c {
			t.Errorf("ParseColor(%q) = %q, %v; want %q, nil", c, got, err, c)
		}
	}
	for _, in := range []string{"", "AMBER", "chartreuse", "amber "} {
		if _, err := ParseColor(in); !errors.Is(err, ErrInvalidColor) {
			t.Errorf("ParseColor(%q) error = %v; want ErrInvalidColor", in, err)
		}
	}
}

func TestPaletteOrderIsFixed(t *testing.T) {
	want := []Color{"amber", "clay", "rose", "plum", "sage", "olive", "teal", "slate"}
	if len(Colors) != len(want) {
		t.Fatalf("len(Colors) = %d; want %d", len(Colors), len(want))
	}
	for i := range want {
		if Colors[i] != want[i] {
			t.Errorf("Colors[%d] = %q; want %q", i, Colors[i], want[i])
		}
	}
}

func TestNormalizeDisplayName(t *testing.T) {
	tests := []struct {
		name        string
		displayName string
		username    string
		want        string
		wantErr     error
	}{
		{"trims", "  Sam  ", "sam", "Sam", nil},
		{"empty falls back to the username", "", "sam", "sam", nil},
		{"whitespace only falls back too", "   ", " sam ", "sam", nil},
		{"keeps inner spaces", "Sam der Koch", "sam", "Sam der Koch", nil},
		{"64 runes are allowed", strings.Repeat("ä", 64), "sam", strings.Repeat("ä", 64), nil},
		{"65 runes are refused", strings.Repeat("ä", 65), "sam", "", ErrDisplayNameTooLong},
		{"a newline is refused", "Sam\nKoch", "sam", "", ErrInvalidDisplayName},
		{"a tab is refused", "Sam\tKoch", "sam", "", ErrInvalidDisplayName},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeDisplayName(tt.displayName, tt.username)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v; want %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("got %q; want %q", got, tt.want)
			}
		})
	}
}

func TestFillPaletteReturnsEveryColorInOrder(t *testing.T) {
	got := fillPalette(map[Color]int{"sage": 2, "amber": 1})
	if len(got) != len(Colors) {
		t.Fatalf("len = %d; want %d", len(got), len(Colors))
	}
	for i, c := range Colors {
		if got[i].Color != c {
			t.Fatalf("got[%d].Color = %q; want %q", i, got[i].Color, c)
		}
	}
	if got[0].Count != 1 || got[4].Count != 2 || got[1].Count != 0 {
		t.Errorf("counts = %+v; want amber 1, clay 0, sage 2", got)
	}
}

func TestParseLocale(t *testing.T) {
	for _, want := range Locales {
		got, err := ParseLocale(string(want))
		if err != nil {
			t.Fatalf("ParseLocale(%q): %v", want, err)
		}
		if got != want {
			t.Errorf("ParseLocale(%q) = %q", want, got)
		}
	}
}

func TestParseLocaleRejectsUnknown(t *testing.T) {
	for _, in := range []string{"", "EN", "xx", "en-US", "de_DE"} {
		if _, err := ParseLocale(in); !errors.Is(err, ErrInvalidLocale) {
			t.Errorf("ParseLocale(%q) error = %v, want ErrInvalidLocale", in, err)
		}
	}
}

func TestBaseLocaleIsOneOfLocales(t *testing.T) {
	if _, err := ParseLocale(string(BaseLocale)); err != nil {
		t.Errorf("BaseLocale %q is not accepted by ParseLocale: %v", BaseLocale, err)
	}
}

func TestLeastUsed(t *testing.T) {
	empty := fillPalette(map[Color]int{})
	if got := leastUsed(empty); got != "amber" {
		t.Errorf("empty table: got %q; want amber", got)
	}

	taken := map[Color]int{}
	for _, c := range Colors {
		taken[c] = 1
	}
	taken["rose"] = 0
	if got := leastUsed(fillPalette(taken)); got != "rose" {
		t.Errorf("one free: got %q; want rose", got)
	}

	full := map[Color]int{}
	for _, c := range Colors {
		full[c] = 2
	}
	full["teal"] = 1
	full["clay"] = 1
	if got := leastUsed(fillPalette(full)); got != "clay" {
		t.Errorf("tie: got %q; want clay (palette order)", got)
	}
}
