package oidc

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func TestFlowCookieSeal(t *testing.T) {
	l := New(Config{}, nil, nil, false)
	f := flow{State: "s", Intent: intentLink, UserID: "u1", Expires: time.Now().Add(time.Minute)}

	if got, ok := l.open(l.seal(f)); !ok || got.UserID != "u1" || got.State != "s" {
		t.Fatalf("round trip: %+v, %v", got, ok)
	}

	// A payload rewritten to another account keeps its old MAC.
	sealed := l.seal(f)
	_, mac, _ := strings.Cut(sealed, ".")
	forged := flow{State: "s", Intent: intentLink, UserID: "u2", Expires: f.Expires}
	payload, _, _ := strings.Cut(l.seal(forged), ".")
	if _, ok := l.open(payload + "." + mac); ok {
		t.Fatal("accepted a payload under another payload's MAC")
	}

	// Another process's key (a restart) does not open it.
	if _, ok := New(Config{}, nil, nil, false).open(sealed); ok {
		t.Fatal("opened with another key")
	}

	expired := f
	expired.Expires = time.Now().Add(-time.Second)
	if _, ok := l.open(l.seal(expired)); ok {
		t.Fatal("accepted an expired flow")
	}

	for _, v := range []string{"", ".", "x", base64.RawURLEncoding.EncodeToString([]byte("{}")) + ".!!"} {
		if _, ok := l.open(v); ok {
			t.Fatalf("accepted %q", v)
		}
	}
}

func TestSafeNext(t *testing.T) {
	for in, want := range map[string]string{
		"":                     "/",
		"/recipes/x":           "/recipes/x",
		"/recipes?tag=a#h":     "/recipes?tag=a",
		"//evil.example/":      "/",
		`/\evil.example`:       "/",
		"https://evil.example": "/",
		"recipes":              "/",
		"/login?next=/x":       "/",
		"/\t/evil.example":     "/",
	} {
		if got := safeNext(in); got != want {
			t.Errorf("safeNext(%q) = %q, want %q", in, got, want)
		}
	}
}
