// Package oidc signs people in through one OpenID Connect provider. A login
// ends in an ordinary Rezepte session (auth.Service.StartSession); the
// provider is only asked who someone is, never what they may do.
package oidc

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/user"
)

// Config is the provider an instance signs in with. Identities are stored and
// looked up under Issuer, never under an ID token's iss: go-oidc accepts
// Google's scheme-less "accounts.google.com" for "https://accounts.google.com",
// and one person must not end up under two keys.
type Config struct {
	PublicURL, Issuer, ClientID, ClientSecret, Name string
}

// Login runs the authorization code flow against the configured provider.
type Login struct {
	cfg       Config
	sessions  *auth.Service
	users     *user.Service
	secure    bool
	cookieKey []byte // HMAC key for the flow cookie, per process
	client    *http.Client
	logger    *slog.Logger

	mu       sync.Mutex
	provider *gooidc.Provider // discovered lazily; nil until the first success
}

const (
	flowCookie = "rezepte_oidc"
	flowPath   = "/api/v1/auth/oidc/"
	flowTTL    = 10 * time.Minute
)

// The intents a flow can have: sign in, connect the signed-in account, or
// finish an account through its setup link.
const (
	intentLogin = "login"
	intentLink  = "link"
	intentSetup = "setup"
)

// flow is what the browser carries from start to callback.
type flow struct {
	State, Nonce, Verifier, Intent, Next string
	UserID                               string `json:",omitempty"` // intent link: who started it
	// Setup is the setup link's hash (auth.SetupLinkHash), enough to peek
	// and consume the link at the end; the raw token never enters the cookie.
	Setup   string `json:",omitempty"`
	Expires time.Time
}

// New returns a Login for cfg. It does not contact the provider: discovery
// runs on the first start, so an unreachable provider never stops Rezepte
// from starting, and password login keeps working without it.
func New(cfg Config, sessions *auth.Service, users *user.Service, secureCookies bool) *Login {
	key := make([]byte, 32)
	_, _ = rand.Read(key)
	return &Login{
		cfg: cfg, sessions: sessions, users: users, secure: secureCookies, cookieKey: key,
		client: &http.Client{Timeout: 10 * time.Second},
		logger: slog.Default(),
	}
}

// ctx carries the timeout client go-oidc and oauth2 read from the context.
func (l *Login) ctx(r *http.Request) context.Context {
	return gooidc.ClientContext(r.Context(), l.client)
}

// discover returns the cached provider or runs discovery. A failure is not
// cached, so the next request tries again.
func (l *Login) discover(ctx context.Context) (*gooidc.Provider, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.provider != nil {
		return l.provider, nil
	}
	// The provider keeps the client from this context for its key set, but
	// not the request's cancellation.
	p, err := gooidc.NewProvider(ctx, l.cfg.Issuer)
	if err != nil {
		return nil, fmt.Errorf("discover %s: %w", l.cfg.Issuer, err)
	}
	l.provider = p
	return p, nil
}

func (l *Login) oauth(p *gooidc.Provider) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     l.cfg.ClientID,
		ClientSecret: l.cfg.ClientSecret,
		Endpoint:     p.Endpoint(),
		RedirectURL:  l.cfg.PublicURL + "/api/v1/auth/oidc/callback",
		Scopes:       []string{gooidc.ScopeOpenID, "email"},
	}
}

// page is the SPA page a flow of intent returns to when it fails.
func page(intent string) string {
	switch intent {
	case intentLink:
		return "/settings"
	case intentSetup:
		return "/welcome"
	}
	return "/login"
}

func (l *Login) start(w http.ResponseWriter, r *http.Request) {
	ctx := l.ctx(r)
	f := flow{Intent: r.PostFormValue("intent"), Next: r.PostFormValue("next")}
	switch f.Intent {
	case intentLogin:
	case intentLink:
		c, err := r.Cookie(auth.CookieName)
		if err != nil {
			redirect(w, r, "/login?next=/settings")
			return
		}
		v, err := l.sessions.Authenticate(ctx, c.Value)
		if err != nil {
			redirect(w, r, "/login?next=/settings")
			return
		}
		f.UserID = v.User.ID
	case intentSetup:
		// PeekSetupLinkByHash refuses a reset link, so this flow only ever finishes a setup link.
		f.Setup = auth.SetupLinkHash(r.PostFormValue("setup"))
		if _, err := l.sessions.PeekSetupLinkByHash(ctx, f.Setup); err != nil {
			l.fail(w, r, intentSetup, "setup link not open", err)
			return
		}
	default:
		l.fail(w, r, intentLogin, "unknown intent", nil)
		return
	}
	p, err := l.discover(ctx)
	if err != nil {
		l.fail(w, r, f.Intent, "provider unavailable", err)
		return
	}
	f.State, f.Nonce, f.Verifier = rand.Text(), rand.Text(), oauth2.GenerateVerifier()
	f.Expires = time.Now().Add(flowTTL)
	authURL := l.oauth(p).AuthCodeURL(f.State, oauth2.S256ChallengeOption(f.Verifier), gooidc.Nonce(f.Nonce))
	c := l.flowCookie(l.seal(f), int(flowTTL.Seconds()))
	http.SetCookie(w, &c)
	redirect(w, r, authURL)
}

// claims are the ID token claims Rezepte reads besides iss and sub.
type claims struct {
	Email         string   `json:"email"`
	EmailVerified verified `json:"email_verified"`
}

// verified reads email_verified leniently: JSON true or the string "true" in
// any case (some providers send a string) mean verified; anything else -
// false, "false", a number, an object - means unverified, never an error.
type verified bool

func (v *verified) UnmarshalJSON(b []byte) error {
	var x any
	_ = json.Unmarshal(b, &x)
	switch x := x.(type) {
	case bool:
		*v = verified(x)
	case string:
		*v = verified(strings.EqualFold(x, "true"))
	default:
		*v = false
	}
	return nil
}

func (l *Login) callback(w http.ResponseWriter, r *http.Request) {
	// Whatever happens next, this flow is over: its cookie goes first.
	gone := l.flowCookie("", -1)
	http.SetCookie(w, &gone)

	ctx := l.ctx(r)
	c, err := r.Cookie(flowCookie)
	if err != nil {
		l.fail(w, r, intentLogin, "no flow cookie", nil)
		return
	}
	f, ok := l.open(c.Value)
	if !ok {
		l.fail(w, r, intentLogin, "flow cookie invalid or expired", nil)
		return
	}
	q := r.URL.Query()
	if e := q.Get("error"); e != "" {
		l.fail(w, r, f.Intent, "provider answered with an error", errors.New(e))
		return
	}
	if subtle.ConstantTimeCompare([]byte(q.Get("state")), []byte(f.State)) != 1 {
		l.fail(w, r, f.Intent, "state mismatch", nil)
		return
	}
	p, err := l.discover(ctx)
	if err != nil {
		l.fail(w, r, f.Intent, "provider unavailable", err)
		return
	}
	tok, err := l.oauth(p).Exchange(ctx, q.Get("code"), oauth2.VerifierOption(f.Verifier))
	if err != nil {
		l.fail(w, r, f.Intent, "code exchange failed", err)
		return
	}
	raw, _ := tok.Extra("id_token").(string)
	idt, err := p.Verifier(&gooidc.Config{ClientID: l.cfg.ClientID}).Verify(ctx, raw)
	if err != nil {
		l.fail(w, r, f.Intent, "ID token rejected", err)
		return
	}
	if subtle.ConstantTimeCompare([]byte(idt.Nonce), []byte(f.Nonce)) != 1 {
		l.fail(w, r, f.Intent, "nonce mismatch", nil)
		return
	}
	// The email claims only fill an empty profile; claims that do not
	// decode leave it empty rather than fail a verified sign-in.
	var cl claims
	if err := idt.Claims(&cl); err != nil {
		l.logger.Info("oidc email claims unreadable", "intent", f.Intent, "err", err)
		cl = claims{}
	}
	// Only from here on is anything written: the ID token is verified.
	switch f.Intent {
	case intentLogin:
		l.login(w, r, f, idt)
	case intentLink:
		l.link(w, r, f, idt, cl)
	case intentSetup:
		l.setup(w, r, f, idt, cl)
	}
}

func (l *Login) login(w http.ResponseWriter, r *http.Request, f flow, idt *gooidc.IDToken) {
	ctx := r.Context()
	u, err := l.users.ByIdentity(ctx, l.cfg.Issuer, idt.Subject)
	if errors.Is(err, user.ErrNotFound) {
		l.logger.Info("oidc login with an unlinked identity")
		redirect(w, r, "/login?oidc=unlinked")
		return
	}
	if err != nil {
		l.fail(w, r, f.Intent, "look up identity", err)
		return
	}
	if !l.signIn(w, r, f.Intent, u) {
		return
	}
	redirect(w, r, safeNext(f.Next))
}

func (l *Login) link(w http.ResponseWriter, r *http.Request, f flow, idt *gooidc.IDToken, cl claims) {
	ctx := r.Context()
	// The account that started the flow must still be the one signed in.
	c, err := r.Cookie(auth.CookieName)
	if err != nil {
		l.fail(w, r, f.Intent, "no session at callback", nil)
		return
	}
	v, err := l.sessions.Authenticate(ctx, c.Value)
	if err != nil || v.User.ID != f.UserID {
		l.fail(w, r, f.Intent, "session changed during the flow", nil)
		return
	}
	err = l.users.LinkIdentity(ctx, f.UserID, l.cfg.Issuer, idt.Subject)
	if errors.Is(err, user.ErrIdentityTaken) {
		l.logger.Info("oidc link refused: identity taken", "user", f.UserID)
		redirect(w, r, "/settings?oidc=taken")
		return
	}
	if err != nil {
		l.fail(w, r, f.Intent, "link identity", err)
		return
	}
	if err := l.users.SetEmailIfEmpty(ctx, f.UserID, cl.Email, bool(cl.EmailVerified)); err != nil {
		l.logger.Warn("oidc link: store email", "user", f.UserID, "err", err)
	}
	redirect(w, r, "/settings?oidc=linked")
}

// PeekSetupLinkByHash refuses a reset link, so this flow only ever finishes a
// setup link.
func (l *Login) setup(w http.ResponseWriter, r *http.Request, f flow, idt *gooidc.IDToken, cl claims) {
	ctx := r.Context()
	u, err := l.sessions.PeekSetupLinkByHash(ctx, f.Setup)
	if err != nil {
		l.fail(w, r, f.Intent, "setup link no longer open", err)
		return
	}
	err = l.users.LinkIdentity(ctx, u.ID, l.cfg.Issuer, idt.Subject)
	if errors.Is(err, user.ErrIdentityTaken) {
		l.logger.Info("oidc setup refused: identity taken", "user", u.ID)
		redirect(w, r, "/welcome?oidc=taken")
		return
	}
	if err != nil {
		l.fail(w, r, f.Intent, "link identity", err)
		return
	}
	if _, err := l.sessions.ConsumeSetupLinkByHash(ctx, f.Setup); err != nil {
		l.fail(w, r, f.Intent, "setup link used meanwhile", err)
		return
	}
	if err := l.users.SetEmailIfEmpty(ctx, u.ID, cl.Email, bool(cl.EmailVerified)); err != nil {
		l.logger.Warn("oidc setup: store email", "user", u.ID, "err", err)
	}
	if err := l.sessions.DeleteUserSessionsExcept(ctx, u.ID, ""); err != nil {
		l.fail(w, r, f.Intent, "end earlier sessions", err)
		return
	}
	if !l.signIn(w, r, f.Intent, u) {
		return
	}
	redirect(w, r, "/")
}

// signIn opens a session for u and sets the cookies exactly as password
// login does. It reports false after answering with a failure.
func (l *Login) signIn(w http.ResponseWriter, r *http.Request, intent string, u user.User) bool {
	sess, err := l.sessions.StartSession(r.Context(), u)
	if err != nil {
		l.fail(w, r, intent, "start session", err)
		return false
	}
	sc := auth.SessionCookie(sess.Token, sess.ExpiresAt, l.secure)
	lc := auth.LocaleCookie(u.Locale, l.secure)
	http.SetCookie(w, &sc)
	http.SetCookie(w, &lc)
	return true
}

// fail logs why a flow ended and sends the browser back to the page it
// started from. reason and err never carry a token, code or cookie value.
func (l *Login) fail(w http.ResponseWriter, r *http.Request, intent, reason string, err error) {
	l.logger.Info("oidc flow failed", "intent", intent, "reason", reason, "err", err)
	redirect(w, r, page(intent)+"?oidc=failed")
}

func redirect(w http.ResponseWriter, r *http.Request, to string) {
	http.Redirect(w, r, to, http.StatusSeeOther)
}

func (l *Login) flowCookie(value string, maxAge int) http.Cookie {
	return http.Cookie{ //nolint:gosec // G124: Secure always on an https public URL, else the secureCookies flag (false only for local http dev); HttpOnly and SameSite are always set.
		Name:     flowCookie,
		Value:    value,
		Path:     flowPath,
		MaxAge:   maxAge,
		HttpOnly: true,
		// An https public URL means the browser reaches Rezepte over TLS,
		// whatever REZEPTE_SECURE_COOKIES says.
		Secure: l.secure || strings.HasPrefix(l.cfg.PublicURL, "https://"),
		// Lax, not Strict: the callback is a top-level GET navigation coming
		// from the provider's site, which Lax lets the cookie ride along.
		SameSite: http.SameSiteLaxMode,
	}
}

// seal encodes f as payload.mac, both base64url.
func (l *Login) seal(f flow) string {
	payload, _ := json.Marshal(f)
	return base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(l.mac(payload))
}

// open verifies and decodes a sealed flow; a bad MAC or a passed expiry fail.
func (l *Login) open(value string) (flow, bool) {
	p, m, ok := strings.Cut(value, ".")
	if !ok {
		return flow{}, false
	}
	payload, err1 := base64.RawURLEncoding.DecodeString(p)
	mac, err2 := base64.RawURLEncoding.DecodeString(m)
	if err1 != nil || err2 != nil || !hmac.Equal(mac, l.mac(payload)) {
		return flow{}, false
	}
	var f flow
	if err := json.Unmarshal(payload, &f); err != nil || !time.Now().Before(f.Expires) {
		return flow{}, false
	}
	return f, true
}

func (l *Login) mac(payload []byte) []byte {
	h := hmac.New(sha256.New, l.cookieKey)
	h.Write(payload)
	return h.Sum(nil)
}

// safeNext keeps a post-login target only when it is a path on this origin,
// and never the login page itself. It mirrors safeNext in
// frontend/src/lib/api/auth.ts; the two must agree.
func safeNext(next string) string {
	if !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") || strings.HasPrefix(next, `/\`) {
		return "/"
	}
	u, err := url.Parse(next)
	if err != nil || u.Scheme != "" || u.Host != "" || strings.HasPrefix(u.Path, "/login") {
		return "/"
	}
	return u.RequestURI()
}
