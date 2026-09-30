package oidc

import (
	"context"
	"encoding/base64"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/db/dbtest"
	"github.com/s-frei/rezepte/service/internal/user"
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

// TestIdentitiesKeyedByConfiguredIssuer: go-oidc accepts Google's scheme-less
// iss for the configured https://accounts.google.com, which the fake provider
// cannot issue (go-oidc allows that exception for Google only). So the
// callback's three endings get such a token directly, and every one must
// store or find the identity under the configured issuer.
func TestIdentitiesKeyedByConfiguredIssuer(t *testing.T) {
	ctx := context.Background()
	conn := dbtest.Open(t)
	users := user.NewService(conn, "")
	sessions := auth.NewService(conn, users)
	const issuer = "https://accounts.google.com"
	l := New(Config{Issuer: issuer}, sessions, users, false)
	l.logger = slog.New(slog.DiscardHandler)
	sam, err := users.Create(ctx, user.CreateParams{Username: "sam", Password: "pw", Role: user.RoleAdmin})
	if err != nil {
		t.Fatal(err)
	}
	anna, err := users.Create(ctx, user.CreateParams{Username: "anna", Role: user.RoleUser})
	if err != nil {
		t.Fatal(err)
	}
	token := func(sub string) *gooidc.IDToken {
		return &gooidc.IDToken{Issuer: "accounts.google.com", Subject: sub}
	}
	location := func(rec *httptest.ResponseRecorder) string { return rec.Header().Get("Location") }

	link, err := sessions.IssueSetupLink(ctx, sam, anna.ID)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	l.setup(rec, httptest.NewRequest(http.MethodGet, "/", nil), flow{Intent: intentSetup, Setup: auth.SetupLinkHash(link.Token)}, token("sub-anna"), claims{})
	if location(rec) != "/" {
		t.Fatalf("setup ended at %q", location(rec))
	}

	sess, err := sessions.StartSession(ctx, sam)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: sess.Token})
	rec = httptest.NewRecorder()
	l.link(rec, req, flow{Intent: intentLink, UserID: sam.ID}, token("sub-sam"), claims{})
	if location(rec) != "/settings?oidc=linked" {
		t.Fatalf("link ended at %q", location(rec))
	}

	for sub, want := range map[string]string{"sub-anna": "anna", "sub-sam": "sam"} {
		if u, err := users.ByIdentity(ctx, issuer, sub); err != nil || u.Username != want {
			t.Errorf("%s under the configured issuer: %q, %v", sub, u.Username, err)
		}
	}

	rec = httptest.NewRecorder()
	l.login(rec, httptest.NewRequest(http.MethodGet, "/", nil), flow{Intent: intentLogin}, token("sub-anna"))
	if location(rec) != "/" {
		t.Fatalf("login ended at %q", location(rec))
	}
}

// TestFlowCookieSecure: an https public URL makes the flow cookie Secure
// whatever REZEPTE_SECURE_COOKIES says; on http it follows the setting.
func TestFlowCookieSecure(t *testing.T) {
	for _, tc := range []struct {
		publicURL string
		secure    bool
		want      bool
	}{
		{"https://rezepte.example", false, true},
		{"https://rezepte.example", true, true},
		{"http://localhost:8060", false, false},
		{"http://localhost:8060", true, true},
	} {
		l := New(Config{PublicURL: tc.publicURL}, nil, nil, tc.secure)
		for _, c := range []http.Cookie{l.flowCookie("v", 60), l.flowCookie("", -1)} {
			if c.Secure != tc.want {
				t.Errorf("%s, secure cookies %v: Secure = %v, want %v", tc.publicURL, tc.secure, c.Secure, tc.want)
			}
		}
	}
}
