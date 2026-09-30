package oidc_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/avatar"
	"github.com/s-frei/rezepte/service/internal/config"
	"github.com/s-frei/rezepte/service/internal/db/dbtest"
	"github.com/s-frei/rezepte/service/internal/httpserver"
	"github.com/s-frei/rezepte/service/internal/image"
	"github.com/s-frei/rezepte/service/internal/oidc"
	"github.com/s-frei/rezepte/service/internal/user"
	"github.com/s-frei/rezepte/service/internal/userapi"
)

const publicURL = "http://localhost:8060"

// env is the real server with OIDC wired the way cmd/rezepte wires it, a
// fake provider behind it, and two accounts: sam (admin, password "pw") and
// anna (member, no password).
type env struct {
	h         http.Handler
	idp       *fakeIdP
	users     *user.Service
	sessions  *auth.Service
	tokens    *auth.TokenService
	sam, anna user.User
}

// newEnv runs against a fresh fake provider.
func newEnv(t *testing.T) *env {
	t.Helper()
	idp := newFakeIdP(t)
	e := newServer(t, &oidc.Config{
		PublicURL: publicURL, Issuer: idp.URL, ClientID: idp.clientID, ClientSecret: "secret", Name: "Test IdP",
	})
	e.idp = idp
	return e
}

// newServer builds the server; a nil cfg leaves OIDC off, as cmd/rezepte does
// without REZEPTE_OIDC_ISSUER.
func newServer(t *testing.T, cfg *oidc.Config) *env {
	t.Helper()
	ctx := context.Background()
	conn := dbtest.Open(t)
	users := user.NewService(conn, "")
	sam, err := users.Create(ctx, user.CreateParams{Username: "sam", Password: "pw", Role: user.RoleAdmin})
	if err != nil {
		t.Fatal(err)
	}
	anna, err := users.Create(ctx, user.CreateParams{Username: "anna", Role: user.RoleUser})
	if err != nil {
		t.Fatal(err)
	}
	appCfg, _ := config.LoadFrom(map[string]string{})
	sessions := auth.NewService(conn, users)
	tokens := auth.NewTokenService(conn, users)
	srv := httpserver.New(appCfg, slog.New(slog.DiscardHandler), fstest.MapFS{},
		httpserver.WithAPIMiddleware(auth.Middleware(sessions, tokens, false)),
		httpserver.WithSecuritySchemes(auth.SecuritySchemes()))
	auth.Register(srv.API(), sessions, false)
	issuer := ""
	if cfg != nil {
		issuer = cfg.Issuer
	}
	userapi.Register(srv.API(), users, sessions, avatar.NewService(conn, t.TempDir(), image.NewService(conn, t.TempDir())), issuer)
	var login *oidc.Login
	if cfg != nil {
		login = oidc.New(*cfg, sessions, users, false)
	}
	oidc.Register(srv.API(), login)
	return &env{h: srv.Handler(), users: users, sessions: sessions, tokens: tokens, sam: sam, anna: anna}
}

func send(h http.Handler, req *http.Request, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	req.Host = "localhost:8060"
	for _, c := range cookies {
		if c != nil {
			req.AddCookie(c)
		}
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func api(h http.Handler, method, path, body string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", publicURL)
	return send(h, req, cookies...)
}

// start posts the start form the way the SPA's ProviderButton does.
func start(h http.Handler, form url.Values, origin string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/oidc/start", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", origin)
	return send(h, req, cookies...)
}

// flow starts intent and returns the provider's authorize URL and the flow
// cookie.
func flow(t *testing.T, h http.Handler, intent string, extra url.Values, session *http.Cookie) (string, *http.Cookie) {
	t.Helper()
	form := url.Values{"intent": {intent}}
	for k, v := range extra {
		form[k] = v
	}
	rec := start(h, form, publicURL, session)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("start %s: status %d: %s", intent, rec.Code, rec.Body.String())
	}
	fc := cookieNamed(rec, "rezepte_oidc")
	if fc == nil {
		t.Fatalf("start %s: no flow cookie, location %s", intent, rec.Header().Get("Location"))
	}
	return rec.Header().Get("Location"), fc
}

func callback(t *testing.T, h http.Handler, state, code string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	q := url.Values{"state": {state}, "code": {code}}
	return send(h, httptest.NewRequest(http.MethodGet, "/api/v1/auth/oidc/callback?"+q.Encode(), nil), cookies...)
}

// run does a whole flow: start, the provider approving claims, callback.
func (e *env) run(t *testing.T, intent string, extra url.Values, session *http.Cookie, c idClaims) *httptest.ResponseRecorder {
	t.Helper()
	authorize, fc := flow(t, e.h, intent, extra, session)
	state, code := e.idp.approve(t, authorize, c)
	return callback(t, e.h, state, code, fc, session)
}

func cookieNamed(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func wantRedirect(t *testing.T, rec *httptest.ResponseRecorder, want string) {
	t.Helper()
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != want {
		t.Fatalf("got %d to %q, want 303 to %q (%s)", rec.Code, rec.Header().Get("Location"), want, rec.Body.String())
	}
}

func wantFailed(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if rec.Code != http.StatusSeeOther || !strings.HasSuffix(rec.Header().Get("Location"), "?oidc=failed") {
		t.Fatalf("got %d to %q, want 303 to ...?oidc=failed", rec.Code, rec.Header().Get("Location"))
	}
	if c := cookieNamed(rec, auth.CookieName); c != nil && c.Value != "" {
		t.Fatal("a failed flow set a session cookie")
	}
}

func (e *env) passwordLogin(t *testing.T) *http.Cookie {
	t.Helper()
	rec := api(e.h, http.MethodPost, "/api/v1/auth/login", `{"username":"sam","password":"pw"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("login: %d %s", rec.Code, rec.Body.String())
	}
	return cookieNamed(rec, auth.CookieName)
}

func (e *env) me(t *testing.T, session *http.Cookie) string {
	t.Helper()
	rec := api(e.h, http.MethodGet, "/api/v1/auth/me", "", session)
	if rec.Code != http.StatusOK {
		t.Fatalf("me: %d %s", rec.Code, rec.Body.String())
	}
	var body struct{ Username string }
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return body.Username
}

func (e *env) link(t *testing.T, u user.User, subject string) {
	t.Helper()
	if err := e.users.LinkIdentity(context.Background(), u.ID, e.idp.URL, subject); err != nil {
		t.Fatal(err)
	}
}

func (e *env) owner(t *testing.T, subject string) string {
	t.Helper()
	u, err := e.users.ByIdentity(context.Background(), e.idp.URL, subject)
	if errors.Is(err, user.ErrNotFound) {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	return u.Username
}

func (e *env) linkedAnyone(t *testing.T) bool {
	t.Helper()
	linked, err := e.users.LinkedUserIDs(context.Background(), e.idp.URL)
	if err != nil {
		t.Fatal(err)
	}
	return len(linked) > 0
}

func TestLoginWithLinkedIdentity(t *testing.T) {
	e := newEnv(t)
	e.link(t, e.anna, "sub-anna")
	rec := e.run(t, "login", nil, nil, idClaims{Subject: "sub-anna"})
	wantRedirect(t, rec, "/")
	session := cookieNamed(rec, auth.CookieName)
	if session == nil || !session.HttpOnly || session.Path != "/" {
		t.Fatalf("session cookie %+v", session)
	}
	if cookieNamed(rec, auth.LocaleCookieName) == nil {
		t.Fatal("no locale cookie")
	}
	if got := e.me(t, session); got != "anna" {
		t.Fatalf("signed in as %q, want anna", got)
	}
}

func TestLoginWithUnlinkedIdentity(t *testing.T) {
	e := newEnv(t)
	rec := e.run(t, "login", nil, nil, idClaims{Subject: "sub-nobody"})
	wantRedirect(t, rec, "/login?oidc=unlinked")
	if cookieNamed(rec, auth.CookieName) != nil {
		t.Fatal("session cookie set for an unlinked identity")
	}
}

func TestLoginKeepsSafeNext(t *testing.T) {
	e := newEnv(t)
	e.link(t, e.anna, "sub-anna")
	rec := e.run(t, "login", url.Values{"next": {"/recipes/x"}}, nil, idClaims{Subject: "sub-anna"})
	wantRedirect(t, rec, "/recipes/x")
}

func TestLoginDropsForeignNext(t *testing.T) {
	for _, next := range []string{"//evil.example/", `/\evil.example`, "https://evil.example/", "/login?next=/x"} {
		e := newEnv(t)
		e.link(t, e.anna, "sub-anna")
		rec := e.run(t, "login", url.Values{"next": {next}}, nil, idClaims{Subject: "sub-anna"})
		wantRedirect(t, rec, "/")
	}
}

func TestLinkFromProfile(t *testing.T) {
	e := newEnv(t)
	session := e.passwordLogin(t)
	rec := e.run(t, "link", nil, session, idClaims{Subject: "sub-sam", Email: "sam@example.com", EmailVerified: true})
	wantRedirect(t, rec, "/settings?oidc=linked")
	if got := e.owner(t, "sub-sam"); got != "sam" {
		t.Fatalf("identity belongs to %q, want sam", got)
	}
	sam, _ := e.users.ByID(context.Background(), e.sam.ID)
	if sam.Email != "sam@example.com" || !sam.EmailVerified {
		t.Fatalf("email %q verified=%v", sam.Email, sam.EmailVerified)
	}
}

func TestLinkIdentityOfAnotherAccount(t *testing.T) {
	e := newEnv(t)
	e.link(t, e.anna, "sub-anna")
	rec := e.run(t, "link", nil, e.passwordLogin(t), idClaims{Subject: "sub-anna"})
	wantRedirect(t, rec, "/settings?oidc=taken")
	if got := e.owner(t, "sub-anna"); got != "anna" {
		t.Fatalf("identity belongs to %q, want anna", got)
	}
}

func TestLinkNeedsASession(t *testing.T) {
	e := newEnv(t)
	rec := start(e.h, url.Values{"intent": {"link"}}, publicURL)
	if rec.Code != http.StatusSeeOther || !strings.HasPrefix(rec.Header().Get("Location"), "/login") {
		t.Fatalf("got %d to %q, want 303 to /login", rec.Code, rec.Header().Get("Location"))
	}
	if cookieNamed(rec, "rezepte_oidc") != nil {
		t.Fatal("flow cookie set without a session")
	}
}

// TestLinkAnotherAccountSignedInMidFlow: the browser still has a valid
// session at the callback, but it is somebody else's - the account that
// started the flow signed out and another signed in meanwhile.
func TestLinkAnotherAccountSignedInMidFlow(t *testing.T) {
	e := newEnv(t)
	authorize, fc := flow(t, e.h, "link", nil, e.passwordLogin(t))
	sess, err := e.sessions.StartSession(context.Background(), e.anna)
	if err != nil {
		t.Fatal(err)
	}
	annaSession := &http.Cookie{Name: auth.CookieName, Value: sess.Token}
	state, code := e.idp.approve(t, authorize, idClaims{Subject: "sub-sam"})
	wantRedirect(t, callback(t, e.h, state, code, fc, annaSession), "/settings?oidc=failed")
	if e.linkedAnyone(t) {
		t.Fatal("identity linked to the account signed in mid-flow")
	}
}

func TestLinkSessionChangedMidFlow(t *testing.T) {
	e := newEnv(t)
	session := e.passwordLogin(t)
	authorize, fc := flow(t, e.h, "link", nil, session)
	if rec := api(e.h, http.MethodPost, "/api/v1/auth/logout", "", session); rec.Code != http.StatusNoContent {
		t.Fatalf("logout: %d", rec.Code)
	}
	state, code := e.idp.approve(t, authorize, idClaims{Subject: "sub-sam"})
	rec := callback(t, e.h, state, code, fc, session)
	wantRedirect(t, rec, "/settings?oidc=failed")
	if e.linkedAnyone(t) {
		t.Fatal("identity linked after the session ended")
	}
}

func TestSetupViaProvider(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	link, err := e.sessions.IssueSetupLink(ctx, e.sam, e.anna.ID)
	if err != nil {
		t.Fatal(err)
	}
	old, err := e.sessions.StartSession(ctx, e.anna)
	if err != nil {
		t.Fatal(err)
	}
	rec := e.run(t, "setup", url.Values{"setup": {link.Token}}, nil, idClaims{Subject: "sub-anna", Email: "anna@example.com"})
	wantRedirect(t, rec, "/")
	if got := e.me(t, cookieNamed(rec, auth.CookieName)); got != "anna" {
		t.Fatalf("signed in as %q, want anna", got)
	}
	if _, err := e.sessions.PeekSetupLink(ctx, link.Token); !errors.Is(err, auth.ErrNoSetupLink) {
		t.Fatalf("setup link still open: %v", err)
	}
	if got := e.owner(t, "sub-anna"); got != "anna" {
		t.Fatalf("identity belongs to %q, want anna", got)
	}
	if _, err := e.sessions.Authenticate(ctx, old.Token); !errors.Is(err, auth.ErrNoSession) {
		t.Fatalf("anna's earlier session survived: %v", err)
	}
	anna, _ := e.users.ByID(ctx, e.anna.ID)
	if anna.Email != "anna@example.com" || anna.EmailVerified {
		t.Fatalf("email %q verified=%v", anna.Email, anna.EmailVerified)
	}
}

// TestSetupFlowCookieHoldsNoToken: the flow cookie carries the setup link's
// hash, never the token, and the hash is enough to finish the setup.
func TestSetupFlowCookieHoldsNoToken(t *testing.T) {
	e := newEnv(t)
	link, err := e.sessions.IssueSetupLink(context.Background(), e.sam, e.anna.ID)
	if err != nil {
		t.Fatal(err)
	}
	authorize, fc := flow(t, e.h, "setup", url.Values{"setup": {link.Token}}, nil)
	sealed, _, _ := strings.Cut(fc.Value, ".")
	payload, err := base64.RawURLEncoding.DecodeString(sealed)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), link.Token) || strings.Contains(fc.Value, link.Token) {
		t.Fatalf("flow cookie carries the raw setup token: %s", payload)
	}
	if !strings.Contains(string(payload), auth.SetupLinkHash(link.Token)) {
		t.Fatalf("flow cookie lacks the setup link's hash: %s", payload)
	}
	state, code := e.idp.approve(t, authorize, idClaims{Subject: "sub-anna"})
	wantRedirect(t, callback(t, e.h, state, code, fc), "/")
	if got := e.owner(t, "sub-anna"); got != "anna" {
		t.Fatalf("identity belongs to %q, want anna", got)
	}
}

func TestSetupWithUsedLink(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	link, err := e.sessions.IssueSetupLink(ctx, e.sam, e.anna.ID)
	if err != nil {
		t.Fatal(err)
	}
	authorize, fc := flow(t, e.h, "setup", url.Values{"setup": {link.Token}}, nil)
	if _, err := e.sessions.ConsumeSetupLink(ctx, link.Token); err != nil {
		t.Fatal(err)
	}
	state, code := e.idp.approve(t, authorize, idClaims{Subject: "sub-anna"})
	wantRedirect(t, callback(t, e.h, state, code, fc), "/welcome?oidc=failed")
	if e.linkedAnyone(t) {
		t.Fatal("identity linked through a used setup link")
	}
}

func TestSetupWithUnknownLink(t *testing.T) {
	e := newEnv(t)
	rec := start(e.h, url.Values{"intent": {"setup"}, "setup": {"nope"}}, publicURL)
	wantRedirect(t, rec, "/welcome?oidc=failed")
}

func TestCallbackRejectsWrongState(t *testing.T) {
	e := newEnv(t)
	session := e.passwordLogin(t)
	authorize, fc := flow(t, e.h, "link", nil, session)
	state, code := e.idp.approve(t, authorize, idClaims{Subject: "sub-sam"})
	wantFailed(t, callback(t, e.h, state+"x", code, fc, session))
	if e.linkedAnyone(t) {
		t.Fatal("identity linked with a wrong state")
	}
}

func TestCallbackRejectsTamperedFlowCookie(t *testing.T) {
	e := newEnv(t)
	e.link(t, e.anna, "sub-anna")
	authorize, fc := flow(t, e.h, "login", nil, nil)
	state, code := e.idp.approve(t, authorize, idClaims{Subject: "sub-anna"})
	// A byte of the MAC, not the payload: the payload stays valid JSON, so
	// only the HMAC check can refuse it.
	b := []byte(fc.Value)
	i := len(b) - 2
	if b[i] == 'A' {
		b[i] = 'B'
	} else {
		b[i] = 'A'
	}
	fc.Value = string(b)
	wantFailed(t, callback(t, e.h, state, code, fc))
}

// TestCallbackRunsOnce: a finished flow cannot be finished again. Two things
// stop it independently - the provider refuses a used code (the fake deletes
// it, as real providers do), and the callback clears the flow cookie, so a
// browser has nothing to carry into a second callback even with a fresh code.
func TestCallbackRunsOnce(t *testing.T) {
	e := newEnv(t)
	e.link(t, e.anna, "sub-anna")
	authorize, fc := flow(t, e.h, "login", nil, nil)
	state, code := e.idp.approve(t, authorize, idClaims{Subject: "sub-anna"})
	first := callback(t, e.h, state, code, fc)
	wantRedirect(t, first, "/")
	if c := cookieNamed(first, "rezepte_oidc"); c == nil || c.MaxAge >= 0 {
		t.Fatalf("flow cookie not cleared: %+v", c)
	}
	// The used code, with the old cookie replayed by hand: the provider says no.
	wantFailed(t, callback(t, e.h, state, code, fc))
	// A fresh, valid code for the same flow, without the cleared cookie.
	_, fresh := e.idp.approve(t, authorize, idClaims{Subject: "sub-anna"})
	wantFailed(t, callback(t, e.h, state, fresh))
}

func TestCallbackClearsTheFlowCookieOnFailure(t *testing.T) {
	e := newEnv(t)
	rec := callback(t, e.h, "s", "c")
	wantFailed(t, rec)
	if c := cookieNamed(rec, "rezepte_oidc"); c == nil || c.MaxAge >= 0 || c.Path != "/api/v1/auth/oidc/" {
		t.Fatalf("flow cookie not cleared: %+v", c)
	}
}

func tamperTest(t *testing.T, tamper func(map[string]any) map[string]any) {
	t.Helper()
	e := newEnv(t)
	e.link(t, e.anna, "sub-anna")
	e.idp.tamper = tamper
	wantFailed(t, e.run(t, "login", nil, nil, idClaims{Subject: "sub-anna"}))
}

func TestCallbackRejectsWrongNonce(t *testing.T) {
	tamperTest(t, func(c map[string]any) map[string]any { c["nonce"] = "x"; return c })
}

func TestCallbackRejectsExpiredToken(t *testing.T) {
	tamperTest(t, func(c map[string]any) map[string]any {
		c["exp"] = time.Now().Add(-time.Hour).Unix()
		return c
	})
}

func TestCallbackRejectsWrongAudience(t *testing.T) {
	tamperTest(t, func(c map[string]any) map[string]any { c["aud"] = "other"; return c })
}

func TestCallbackRejectsWrongIssuer(t *testing.T) {
	tamperTest(t, func(c map[string]any) map[string]any { c["iss"] = "https://evil.example"; return c })
}

func TestCallbackRejectsForeignSignature(t *testing.T) {
	e := newEnv(t)
	e.link(t, e.anna, "sub-anna")
	foreign, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	e.idp.signer = foreign
	wantFailed(t, e.run(t, "login", nil, nil, idClaims{Subject: "sub-anna"}))
}

// TestCallbackRejectsWrongVerifier proves the fake provider's PKCE check is
// live: a token request whose verifier does not hash to the challenge
// approved at authorize time is refused, and so is the sign-in.
func TestCallbackRejectsWrongVerifier(t *testing.T) {
	e := newEnv(t)
	e.link(t, e.anna, "sub-anna")
	authorize, fc := flow(t, e.h, "login", nil, nil)
	state, code := e.idp.approve(t, authorize, idClaims{Subject: "sub-anna"})
	e.idp.mu.Lock()
	g := e.idp.codes[code]
	g.challenge = b64([]byte("another verifier's challenge"))
	e.idp.codes[code] = g
	e.idp.mu.Unlock()
	wantFailed(t, callback(t, e.h, state, code, fc))
}

func TestCallbackProviderError(t *testing.T) {
	e := newEnv(t)
	e.link(t, e.anna, "sub-anna")
	authorize, fc := flow(t, e.h, "login", nil, nil)
	state := mustParse(t, authorize).Query().Get("state")
	q := url.Values{"error": {"access_denied"}, "state": {state}}
	rec := send(e.h, httptest.NewRequest(http.MethodGet, "/api/v1/auth/oidc/callback?"+q.Encode(), nil), fc)
	wantRedirect(t, rec, "/login?oidc=failed")
}

func TestUnverifiedEmailIsStoredUnverified(t *testing.T) {
	e := newEnv(t)
	rec := e.run(t, "link", nil, e.passwordLogin(t), idClaims{Subject: "sub-sam", Email: "sam@example.com"})
	wantRedirect(t, rec, "/settings?oidc=linked")
	sam, _ := e.users.ByID(context.Background(), e.sam.ID)
	if sam.Email != "sam@example.com" || sam.EmailVerified {
		t.Fatalf("email %q verified=%v", sam.Email, sam.EmailVerified)
	}
}

// TestEmailVerifiedForms: providers disagree on email_verified's type. Only
// JSON true and the string "true" (any case) mean verified; no form of it
// fails the sign-in.
func TestEmailVerifiedForms(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value any
		want  bool
	}{
		{"bool true", true, true},
		{"string true", "TRUE", true},
		{"string false", "false", false},
		{"number", 1, false},
		{"object", map[string]any{"value": true}, false},
		{"missing", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			e.idp.tamper = func(c map[string]any) map[string]any {
				if tc.value == nil {
					delete(c, "email_verified")
				} else {
					c["email_verified"] = tc.value
				}
				return c
			}
			rec := e.run(t, "link", nil, e.passwordLogin(t), idClaims{Subject: "sub-sam", Email: "sam@example.com"})
			wantRedirect(t, rec, "/settings?oidc=linked")
			sam, _ := e.users.ByID(context.Background(), e.sam.ID)
			if sam.Email != "sam@example.com" || sam.EmailVerified != tc.want {
				t.Fatalf("email %q verified=%v, want verified=%v", sam.Email, sam.EmailVerified, tc.want)
			}
		})
	}
}

// TestMalformedClaimsDoNotFailLogin: an email claim of the wrong type is
// ignored; a verified ID token still signs the person in.
func TestMalformedClaimsDoNotFailLogin(t *testing.T) {
	e := newEnv(t)
	e.link(t, e.anna, "sub-anna")
	e.idp.tamper = func(c map[string]any) map[string]any {
		c["email"], c["email_verified"] = 42, []any{"true"}
		return c
	}
	wantRedirect(t, e.run(t, "login", nil, nil, idClaims{Subject: "sub-anna"}), "/")
}

func TestProviderDownDoesNotBreakStartup(t *testing.T) {
	e := newServer(t, &oidc.Config{PublicURL: publicURL, Issuer: "http://127.0.0.1:1", ClientID: "rezepte"})
	e.passwordLogin(t)
	wantRedirect(t, start(e.h, url.Values{"intent": {"login"}}, publicURL), "/login?oidc=failed")
}

func TestUnlinkRefusedWithoutPassword(t *testing.T) {
	e := newEnv(t)
	e.link(t, e.anna, "sub-anna")
	sess, err := e.sessions.StartSession(context.Background(), e.anna)
	if err != nil {
		t.Fatal(err)
	}
	cookie := &http.Cookie{Name: auth.CookieName, Value: sess.Token}
	if rec := api(e.h, http.MethodDelete, "/api/v1/auth/me/identity", "", cookie); rec.Code != http.StatusConflict {
		t.Fatalf("unlink: %d %s", rec.Code, rec.Body.String())
	}
	if e.owner(t, "sub-anna") != "anna" {
		t.Fatal("identity gone")
	}
}

func TestUnlinkWithPassword(t *testing.T) {
	e := newEnv(t)
	e.link(t, e.sam, "sub-sam")
	other, err := e.sessions.StartSession(context.Background(), e.sam)
	if err != nil {
		t.Fatal(err)
	}
	session := e.passwordLogin(t)
	if rec := api(e.h, http.MethodGet, "/api/v1/auth/me/identity", "", session); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"linkedAt"`) {
		t.Fatalf("identity: %d %s", rec.Code, rec.Body.String())
	}
	if rec := api(e.h, http.MethodDelete, "/api/v1/auth/me/identity", "", session); rec.Code != http.StatusNoContent {
		t.Fatalf("unlink: %d %s", rec.Code, rec.Body.String())
	}
	// The caller's session stays (this answers 404, not 401); others end.
	if rec := api(e.h, http.MethodGet, "/api/v1/auth/me/identity", "", session); rec.Code != http.StatusNotFound {
		t.Fatalf("identity after unlink: %d %s", rec.Code, rec.Body.String())
	}
	if _, err := e.sessions.Authenticate(context.Background(), other.Token); !errors.Is(err, auth.ErrNoSession) {
		t.Fatalf("sam's other session survived the unlink: %v", err)
	}
}

// adminUnlink sends unlink-user-identity for target as session.
func (e *env) adminUnlink(target string, session *http.Cookie) *httptest.ResponseRecorder {
	return api(e.h, http.MethodDelete, "/api/v1/users/"+target+"/identity", "", session)
}

func (e *env) create(t *testing.T, name string, role user.Role) user.User {
	t.Helper()
	u, err := e.users.Create(context.Background(), user.CreateParams{Username: name, Password: "pw", Role: role})
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func (e *env) sessionOf(t *testing.T, u user.User) *http.Cookie {
	t.Helper()
	sess, err := e.sessions.StartSession(context.Background(), u)
	if err != nil {
		t.Fatal(err)
	}
	return &http.Cookie{Name: auth.CookieName, Value: sess.Token}
}

// TestAdminUnlinksMember: anna has no password, and the admin may still
// disconnect her - the admin then issues a setup link.
func TestAdminUnlinksMember(t *testing.T) {
	e := newEnv(t)
	e.link(t, e.anna, "sub-anna")
	annaSession := e.sessionOf(t, e.anna)
	if rec := e.adminUnlink(e.anna.ID, e.passwordLogin(t)); rec.Code != http.StatusNoContent {
		t.Fatalf("unlink: %d %s", rec.Code, rec.Body.String())
	}
	if e.owner(t, "sub-anna") != "" {
		t.Fatal("identity still linked")
	}
	if _, err := e.sessions.Authenticate(context.Background(), annaSession.Value); !errors.Is(err, auth.ErrNoSession) {
		t.Fatalf("anna's session survived: %v", err)
	}
}

func TestAdminUnlinkRankRule(t *testing.T) {
	e := newEnv(t)
	kim := e.create(t, "kim", user.RoleAdmin)
	owner := e.create(t, "owner", user.RoleSuperadmin)
	e.link(t, kim, "sub-kim")
	e.link(t, owner, "sub-owner")
	e.link(t, e.anna, "sub-anna")
	sam := e.passwordLogin(t)
	for _, tc := range []struct {
		name    string
		target  string
		session *http.Cookie
		want    int
	}{
		{"admin on admin", kim.ID, sam, http.StatusForbidden},
		{"admin on owner", owner.ID, sam, http.StatusConflict},
		{"owner on owner", owner.ID, e.sessionOf(t, owner), http.StatusConflict},
		{"member on member", e.anna.ID, e.sessionOf(t, e.anna), http.StatusForbidden},
		{"anonymous", e.anna.ID, nil, http.StatusUnauthorized},
	} {
		if rec := e.adminUnlink(tc.target, tc.session); rec.Code != tc.want {
			t.Errorf("%s: %d, want %d: %s", tc.name, rec.Code, tc.want, rec.Body.String())
		}
	}
	for _, sub := range []string{"sub-kim", "sub-owner", "sub-anna"} {
		if e.owner(t, sub) == "" {
			t.Errorf("%s unlinked by a refused call", sub)
		}
	}
}

func TestAdminUnlinkRejectsABearerToken(t *testing.T) {
	e := newEnv(t)
	e.link(t, e.anna, "sub-anna")
	raw, _, err := e.tokens.Create(context.Background(), e.sam.ID, "t", []string{auth.ScopeUsersRead, auth.ScopeUsersWrite}, nil)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/users/"+e.anna.ID+"/identity", nil)
	req.Header.Set("Origin", publicURL)
	req.Header.Set("Authorization", "Bearer "+raw)
	if rec := send(e.h, req); rec.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403: %s", rec.Code, rec.Body.String())
	}
	if e.owner(t, "sub-anna") != "anna" {
		t.Fatal("identity gone")
	}
}

func TestAdminUnlinkWithoutIdentity(t *testing.T) {
	e := newEnv(t)
	if rec := e.adminUnlink(e.anna.ID, e.passwordLogin(t)); rec.Code != http.StatusNotFound {
		t.Fatalf("no identity: %d %s", rec.Code, rec.Body.String())
	}
	off := newServer(t, nil)
	if rec := off.adminUnlink(off.anna.ID, off.passwordLogin(t)); rec.Code != http.StatusNotFound {
		t.Fatalf("OIDC off: %d %s", rec.Code, rec.Body.String())
	}
}

func TestGetOIDCConfig(t *testing.T) {
	for _, tc := range []struct {
		e    *env
		want string
	}{
		{newEnv(t), `{"enabled":true,"name":"Test IdP"}`},
		{newServer(t, nil), `{"enabled":false}`},
	} {
		rec := api(tc.e.h, http.MethodGet, "/api/v1/auth/oidc", "")
		var got, want map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &got)
		_ = json.Unmarshal([]byte(tc.want), &want)
		delete(got, "$schema")
		if rec.Code != http.StatusOK || len(got) != len(want) || got["enabled"] != want["enabled"] || got["name"] != want["name"] {
			t.Errorf("got %d %s, want %s", rec.Code, rec.Body.String(), tc.want)
		}
	}
	off := newServer(t, nil)
	if rec := api(off.h, http.MethodGet, "/api/v1/auth/me/identity", "", off.passwordLogin(t)); rec.Code != http.StatusNotFound {
		t.Errorf("identity with OIDC off: %d", rec.Code)
	}
}

func TestStartRejectsCrossOrigin(t *testing.T) {
	e := newEnv(t)
	rec := start(e.h, url.Values{"intent": {"login"}}, "https://evil.example")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403", rec.Code)
	}
}
