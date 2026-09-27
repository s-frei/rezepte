package preview

import (
	"bytes"
	"testing"
	"time"
)

func testKey(b byte) []byte { return bytes.Repeat([]byte{b}, 32) }

func TestSignerAcceptsItsOwnTokenUntilItExpires(t *testing.T) {
	s := NewSigner(testKey(1))
	now := time.Unix(1_800_000_000, 0)
	s.now = func() time.Time { return now }
	token, expires := s.Sign("recipe-a", time.Hour)
	if want := now.Add(time.Hour); !expires.Equal(want) {
		t.Fatalf("expires = %v, want %v", expires, want)
	}
	if !s.Verify("recipe-a", token) {
		t.Fatal("fresh token refused")
	}
	now = expires.Add(-time.Second)
	if !s.Verify("recipe-a", token) {
		t.Fatal("token refused a second before it expires")
	}
	now = expires
	if s.Verify("recipe-a", token) {
		t.Fatal("expired token accepted")
	}
}

func TestSignerRefusesForeignTokens(t *testing.T) {
	s := NewSigner(testKey(1))
	token, _ := s.Sign("recipe-a", time.Hour)
	otherKey, _ := NewSigner(testKey(2)).Sign("recipe-a", time.Hour)
	flipped := []byte(token)
	flipped[len(flipped)-1] ^= 1
	for name, tc := range map[string]struct{ recipe, token string }{
		"other recipe": {"recipe-b", token},
		"other key":    {"recipe-a", otherKey},
		"tampered":     {"recipe-a", string(flipped)},
		"empty":        {"recipe-a", ""},
		"not base64":   {"recipe-a", "!!!"},
		"too short":    {"recipe-a", token[:10]},
	} {
		if s.Verify(tc.recipe, tc.token) {
			t.Errorf("%s: accepted", name)
		}
	}
}
