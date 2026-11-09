package auth_test

import (
	"bytes"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/s-frei/rezepte/service/internal/auth"
)

var deviceKey = bytes.Repeat([]byte{1}, 32)

func TestParseRoundTrip(t *testing.T) {
	d := auth.NewDevice(deviceKey)
	entries := []auth.DeviceEntry{{UserID: "u1", Token: "tok-1"}, {UserID: "u2", Token: "tok_2"}}
	c := d.Cookie(entries, false)
	if got := d.Parse(c.Value); !slices.Equal(got, entries) {
		t.Fatalf("Parse = %+v, want %+v", got, entries)
	}
}

func TestParseDropsForgedEntries(t *testing.T) {
	d := auth.NewDevice(deviceKey)
	good := d.Cookie([]auth.DeviceEntry{{UserID: "a", Token: "ta"}, {UserID: "b", Token: "tb"}}, false).Value
	goodA, goodB, _ := strings.Cut(good, "~")

	// One character of the MAC changed. Not the last: its low bits are
	// padding, which lenient base64 decoding ignores.
	flipped := []byte(goodA)
	i := len(flipped) - 5
	flipped[i] = map[bool]byte{true: 'B', false: 'A'}[flipped[i] == 'A']
	// A well-formed entry under another key.
	foreign := auth.NewDevice(bytes.Repeat([]byte{2}, 32)).Cookie([]auth.DeviceEntry{{UserID: "c", Token: "tc"}}, false).Value
	// The token swapped: the signature covers it.
	parts := strings.SplitN(goodA, ".", 3)
	swapped := parts[0] + ".other." + parts[2]
	// A part missing.
	missing := parts[0] + "." + parts[2]

	value := strings.Join([]string{string(flipped), foreign, swapped, missing, "garbage", "", "...", goodB}, "~")
	got := d.Parse(value)
	if want := []auth.DeviceEntry{{UserID: "b", Token: "tb"}}; !slices.Equal(got, want) {
		t.Fatalf("Parse = %+v, want %+v", got, want)
	}
	if got := d.Parse(""); len(got) != 0 {
		t.Fatalf("Parse(\"\") = %+v", got)
	}
}

func TestHandoverDedupes(t *testing.T) {
	a1 := auth.DeviceEntry{UserID: "a", Token: "t1"}
	b2 := auth.DeviceEntry{UserID: "b", Token: "t2"}
	b3 := auth.DeviceEntry{UserID: "b", Token: "t3"}

	if got, want := auth.Handover([]auth.DeviceEntry{a1, b2}, &b3, "a"), []auth.DeviceEntry{b3}; !slices.Equal(got, want) {
		t.Fatalf("Handover(prev b, active a) = %+v, want %+v", got, want)
	}
	if got, want := auth.Handover([]auth.DeviceEntry{a1, b2}, nil, "b"), []auth.DeviceEntry{a1}; !slices.Equal(got, want) {
		t.Fatalf("Handover(no prev, active b) = %+v, want %+v", got, want)
	}
	if got, want := auth.Handover([]auth.DeviceEntry{a1, b2}, &b3, "b"), []auth.DeviceEntry{a1}; !slices.Equal(got, want) {
		t.Fatalf("Handover(prev is active) = %+v, want %+v", got, want)
	}
}

func TestHandoverCapsAtTen(t *testing.T) {
	var entries []auth.DeviceEntry
	for i := range 10 {
		entries = append(entries, auth.DeviceEntry{UserID: string(rune('a' + i)), Token: "t"})
	}
	prev := auth.DeviceEntry{UserID: "new", Token: "tn"}
	got := auth.Handover(entries, &prev, "active")
	if len(got) != 10 || got[0] != prev || got[9] != entries[8] {
		t.Fatalf("Handover = %+v, want prev first and %v dropped", got, entries[9])
	}
}

func TestCookieAttributes(t *testing.T) {
	d := auth.NewDevice(deviceKey)
	for _, secure := range []bool{false, true} {
		c := d.Cookie([]auth.DeviceEntry{{UserID: "a", Token: "t"}}, secure)
		if c.Name != auth.AccountsCookieName || c.Path != "/api/v1/auth" || !c.HttpOnly ||
			c.SameSite != http.SameSiteLaxMode || c.Secure != secure || c.MaxAge != 400*24*60*60 || c.Value == "" {
			t.Fatalf("cookie %+v (secure %v)", c, secure)
		}
	}
	gone := d.Cookie(nil, true)
	if gone.MaxAge >= 0 || gone.Value != "" || gone.Path != "/api/v1/auth" || !gone.Secure {
		t.Fatalf("empty list cookie %+v", gone)
	}
}
