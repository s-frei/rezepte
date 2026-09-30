package user

import (
	"errors"
	"strings"
	"testing"
)

func TestParseEmail(t *testing.T) {
	cases := []struct {
		in, want string
		err      error
	}{
		{"anna@example.com", "anna@example.com", nil},
		{"  anna@example.com  ", "anna@example.com", nil},
		{"", "", nil},
		{"   ", "", nil},
		{"Anna <anna@example.com>", "", ErrInvalidEmail},
		{"not-an-address", "", ErrInvalidEmail},
		{strings.Repeat("a", 250) + "@x.org", "", ErrInvalidEmail},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got, err := ParseEmail(c.in)
			if !errors.Is(err, c.err) || got != c.want {
				t.Fatalf("ParseEmail(%q) = %q, %v; want %q, %v", c.in, got, err, c.want, c.err)
			}
		})
	}
}
