package sign

import (
	"bytes"
	"testing"
	"time"
)

func testKey(b byte) []byte { return bytes.Repeat([]byte{b}, 32) }

func TestSignerAcceptsItsOwnTokenUntilItExpires(t *testing.T) {
	s := New(testKey(1), "test label")
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
	s := New(testKey(1), "test label")
	token, _ := s.Sign("recipe-a", time.Hour)
	otherKey, _ := New(testKey(2), "test label").Sign("recipe-a", time.Hour)
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

func TestSignerSeparatesLabels(t *testing.T) {
	a := New(testKey(1), "rezepte link preview")
	b := New(testKey(1), "rezepte draft photo")
	token, _ := a.Sign("subject", time.Hour)
	if b.Verify("subject", token) {
		t.Fatal("token signed under one label accepted under another")
	}
	if !a.Verify("subject", token) {
		t.Fatal("token refused under its own label")
	}
}

func TestLinkPreviewTokensAreUnchanged(t *testing.T) {
	// Tokens already sent in chats must keep verifying after the move: the MAC
	// input is label NUL subject NUL expiry, exactly what preview signed.
	s := New(testKey(1), "rezepte link preview")
	s.now = func() time.Time { return time.Unix(1_800_000_000, 0) }
	token, _ := s.Sign("recipe-a", time.Hour)
	// Computed with preview.NewSigner before the move (develop @ 476a8b27).
	if want := "trobo0.pBKV9v6-DrbEXXlCovb5GQ"; token != want {
		t.Fatalf("token = %q, want %q", token, want)
	}
}
