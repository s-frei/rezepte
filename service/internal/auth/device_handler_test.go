package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/user"
)

// deviceEnv is the auth API with two people who share a device: anna (A,
// a member) and sam (B, the seeded admin).
type deviceEnv struct {
	h         http.Handler
	sessions  *auth.Service
	users     *user.Service
	anna, sam user.User
}

func newDeviceEnv(t *testing.T) *deviceEnv {
	t.Helper()
	h, sessions, users := newHandlerWithSessions(t)
	ctx := context.Background()
	anna, err := users.Create(ctx, user.CreateParams{Username: "anna", Password: "pw-anna", Role: user.RoleUser})
	if err != nil {
		t.Fatal(err)
	}
	sam, err := users.ByUsername(ctx, "sam")
	if err != nil {
		t.Fatal(err)
	}
	return &deviceEnv{h: h, sessions: sessions, users: users, anna: anna, sam: sam}
}

func cookieOf(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// accountIDs parses an accounts cookie the way the server does.
func accountIDs(c *http.Cookie) []string {
	if c == nil {
		return nil
	}
	var ids []string
	for _, e := range auth.NewDevice(deviceKey).Parse(c.Value) {
		ids = append(ids, e.UserID)
	}
	return ids
}

func (e *deviceEnv) login(t *testing.T, username, password string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	rec := do(e.h, http.MethodPost, "/api/v1/auth/login", `{"username":"`+username+`","password":"`+password+`"}`, cookies...)
	if rec.Code != http.StatusOK {
		t.Fatalf("login %s: %d %s", username, rec.Code, rec.Body.String())
	}
	return rec
}

func (e *deviceEnv) accounts(t *testing.T, cookies ...*http.Cookie) ([]auth.DeviceAccount, *httptest.ResponseRecorder) {
	t.Helper()
	rec := do(e.h, http.MethodGet, "/api/v1/auth/accounts", "", cookies...)
	if rec.Code != http.StatusOK {
		t.Fatalf("accounts: %d %s", rec.Code, rec.Body.String())
	}
	var list []auth.DeviceAccount
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	return list, rec
}

// onTop signs in anna, then sam on the same device, and returns sam's
// session, anna's session and the accounts cookie.
func (e *deviceEnv) onTop(t *testing.T) (samSession, annaSession, accounts *http.Cookie) {
	t.Helper()
	annaSession = cookieOf(e.login(t, "anna", "pw-anna"), auth.CookieName)
	rec := e.login(t, "sam", "pw", annaSession)
	return cookieOf(rec, auth.CookieName), annaSession, cookieOf(rec, auth.AccountsCookieName)
}

func TestLoginOnTopKeepsThePreviousPerson(t *testing.T) {
	e := newDeviceEnv(t)
	samSession, _, accounts := e.onTop(t)
	if got := accountIDs(accounts); !slices.Equal(got, []string{e.sam.ID, e.anna.ID}) {
		t.Fatalf("accounts cookie lists %v, want sam (active), anna", got)
	}
	list, _ := e.accounts(t, samSession, accounts)
	if len(list) != 1 || list[0].ID != e.anna.ID || list[0].Username != "anna" || list[0].Expired || list[0].LastSeenAt.IsZero() {
		t.Fatalf("accounts = %+v, want anna, not expired", list)
	}
}

func TestLoginAloneWritesOnlyThatPerson(t *testing.T) {
	e := newDeviceEnv(t)
	rec := e.login(t, "sam", "pw")
	if got := accountIDs(cookieOf(rec, auth.AccountsCookieName)); !slices.Equal(got, []string{e.sam.ID}) {
		t.Fatalf("accounts cookie lists %v, want sam", got)
	}
}

func TestBrowserDroppedSessionCookieStaysOnDevice(t *testing.T) {
	e := newDeviceEnv(t)
	samSession, _, accounts := e.onTop(t)
	// The browser dropped sam's session cookie; only the accounts cookie comes.
	list, _ := e.accounts(t, accounts)
	if len(list) != 2 || list[0].ID != e.sam.ID || list[0].Expired || list[1].ID != e.anna.ID {
		t.Fatalf("accounts = %+v, want sam (valid), anna", list)
	}
	// Once that session has ended too, sam is still there, expired.
	if err := e.sessions.Logout(context.Background(), samSession.Value); err != nil {
		t.Fatal(err)
	}
	list, _ = e.accounts(t, accounts)
	if len(list) != 2 || list[0].ID != e.sam.ID || !list[0].Expired {
		t.Fatalf("accounts = %+v, want sam expired first", list)
	}
	// And signing someone in on top of the dropped cookie keeps him.
	rec := e.login(t, "anna", "pw-anna", accounts)
	if got := accountIDs(cookieOf(rec, auth.AccountsCookieName)); !slices.Equal(got, []string{e.anna.ID, e.sam.ID}) {
		t.Fatalf("accounts cookie lists %v, want anna, sam", got)
	}
}

func TestSwitchSwapsSessions(t *testing.T) {
	e := newDeviceEnv(t)
	samSession, annaSession, accounts := e.onTop(t)
	rec := do(e.h, http.MethodPost, "/api/v1/auth/switch", `{"userId":"`+e.anna.ID+`"}`, samSession, accounts)
	if rec.Code != http.StatusOK {
		t.Fatalf("switch: %d %s", rec.Code, rec.Body.String())
	}
	var body auth.UserResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body.ID != e.anna.ID {
		t.Fatalf("switched to %+v, want anna", body)
	}
	if c := cookieOf(rec, auth.CookieName); c == nil || c.Value != annaSession.Value {
		t.Fatalf("session cookie %+v, want anna's token", c)
	}
	if cookieOf(rec, auth.LocaleCookieName) == nil {
		t.Fatal("no locale cookie")
	}
	if got := accountIDs(cookieOf(rec, auth.AccountsCookieName)); !slices.Equal(got, []string{e.anna.ID, e.sam.ID}) {
		t.Fatalf("accounts cookie lists %v, want anna (active), sam", got)
	}
}

func TestSwitchToExpiredAnswers409(t *testing.T) {
	e := newDeviceEnv(t)
	samSession, annaSession, accounts := e.onTop(t)
	if err := e.sessions.Logout(context.Background(), annaSession.Value); err != nil {
		t.Fatal(err)
	}
	rec := do(e.h, http.MethodPost, "/api/v1/auth/switch", `{"userId":"`+e.anna.ID+`"}`, samSession, accounts)
	if rec.Code != http.StatusConflict {
		t.Fatalf("switch: %d %s", rec.Code, rec.Body.String())
	}
	if c := cookieOf(rec, auth.AccountsCookieName); c != nil {
		t.Fatalf("a refused switch rewrote the accounts cookie: %+v", c)
	}
	list, _ := e.accounts(t, samSession, accounts)
	if len(list) != 1 || list[0].ID != e.anna.ID || !list[0].Expired {
		t.Fatalf("accounts = %+v, want anna, expired", list)
	}
}

func TestAccountsWithoutActiveSession(t *testing.T) {
	e := newDeviceEnv(t)
	samSession, _, accounts := e.onTop(t)
	// Sam signs out; anna is left on the device, and sam stays as expired.
	rec := do(e.h, http.MethodPost, "/api/v1/auth/logout", "", samSession, accounts)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("logout: %d", rec.Code)
	}
	accounts = cookieOf(rec, auth.AccountsCookieName)

	list, _ := e.accounts(t, accounts)
	if len(list) != 2 || list[0].ID != e.sam.ID || !list[0].Expired || list[1].ID != e.anna.ID || list[1].Expired {
		t.Fatalf("accounts = %+v, want sam expired, anna", list)
	}
	rec = do(e.h, http.MethodPost, "/api/v1/auth/switch", `{"userId":"`+e.anna.ID+`"}`, accounts)
	if rec.Code != http.StatusOK || cookieOf(rec, auth.CookieName) == nil {
		t.Fatalf("switch: %d %s", rec.Code, rec.Body.String())
	}
	if rec := do(e.h, http.MethodGet, "/api/v1/auth/me", "", cookieOf(rec, auth.CookieName)); rec.Code != http.StatusOK {
		t.Fatalf("me after switch: %d", rec.Code)
	}
	// Nobody was active: anna moves to the front, sam's dead entry stays.
	if got := accountIDs(cookieOf(rec, auth.AccountsCookieName)); !slices.Equal(got, []string{e.anna.ID, e.sam.ID}) {
		t.Fatalf("accounts cookie lists %v, want anna, sam", got)
	}
}

func TestAccountsDropsDeletedUsers(t *testing.T) {
	e := newDeviceEnv(t)
	samSession, _, accounts := e.onTop(t)
	if err := e.users.Delete(context.Background(), e.sam, e.anna.ID); err != nil {
		t.Fatal(err)
	}
	list, rec := e.accounts(t, samSession, accounts)
	if len(list) != 0 {
		t.Fatalf("accounts = %+v, want none", list)
	}
	if got := accountIDs(cookieOf(rec, auth.AccountsCookieName)); !slices.Equal(got, []string{e.sam.ID}) {
		t.Fatalf("accounts cookie rewritten to %v, want sam alone", got)
	}
	if rec := do(e.h, http.MethodPost, "/api/v1/auth/switch", `{"userId":"`+e.anna.ID+`"}`, samSession, accounts); rec.Code != http.StatusNotFound {
		t.Fatalf("switch to a deleted user: %d", rec.Code)
	}
}

func TestAccountsLeavesAnUnchangedCookieAlone(t *testing.T) {
	e := newDeviceEnv(t)
	samSession, _, accounts := e.onTop(t)
	if _, rec := e.accounts(t, samSession, accounts); cookieOf(rec, auth.AccountsCookieName) != nil {
		t.Fatal("a list with nothing dropped rewrote the cookie")
	}
}

func TestRemoveDeviceAccountEndsItsSession(t *testing.T) {
	e := newDeviceEnv(t)
	samSession, annaSession, accounts := e.onTop(t)
	rec := do(e.h, http.MethodDelete, "/api/v1/auth/accounts/"+e.anna.ID, "", samSession, accounts)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("remove: %d %s", rec.Code, rec.Body.String())
	}
	if _, err := e.sessions.Authenticate(context.Background(), annaSession.Value); !errors.Is(err, auth.ErrNoSession) {
		t.Fatalf("anna's token still authenticates: %v", err)
	}
	if got := accountIDs(cookieOf(rec, auth.AccountsCookieName)); !slices.Equal(got, []string{e.sam.ID}) {
		t.Fatalf("accounts cookie lists %v, want sam alone", got)
	}
	if rec := do(e.h, http.MethodDelete, "/api/v1/auth/accounts/"+e.anna.ID, "", samSession); rec.Code != http.StatusNotFound {
		t.Fatalf("remove without an entry: %d", rec.Code)
	}
	// The active person signs out instead.
	if rec := do(e.h, http.MethodDelete, "/api/v1/auth/accounts/"+e.sam.ID, "", samSession, accounts); rec.Code != http.StatusConflict {
		t.Fatalf("remove the active person: %d", rec.Code)
	}
}

func TestLogoutKeepsPersonWhenOthersRemain(t *testing.T) {
	e := newDeviceEnv(t)
	samSession, _, accounts := e.onTop(t)
	rec := do(e.h, http.MethodPost, "/api/v1/auth/logout", "", samSession, accounts)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("logout: %d", rec.Code)
	}
	if got := accountIDs(cookieOf(rec, auth.AccountsCookieName)); !slices.Equal(got, []string{e.sam.ID, e.anna.ID}) {
		t.Fatalf("accounts cookie lists %v, want sam, anna", got)
	}
	if c := cookieOf(rec, auth.CookieName); c == nil || c.MaxAge >= 0 {
		t.Fatalf("session cookie %+v, want it expired", c)
	}
	if _, err := e.sessions.Authenticate(context.Background(), samSession.Value); !errors.Is(err, auth.ErrNoSession) {
		t.Fatalf("sam's session survived logout: %v", err)
	}
}

func TestLogoutClearsCookieWhenAlone(t *testing.T) {
	e := newDeviceEnv(t)
	login := e.login(t, "sam", "pw")
	rec := do(e.h, http.MethodPost, "/api/v1/auth/logout", "", cookieOf(login, auth.CookieName), cookieOf(login, auth.AccountsCookieName))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("logout: %d", rec.Code)
	}
	if c := cookieOf(rec, auth.AccountsCookieName); c == nil || c.MaxAge >= 0 {
		t.Fatalf("accounts cookie %+v, want it expired", c)
	}
}

func TestRedeemSetupLinkKeepsThePreviousPerson(t *testing.T) {
	e := newDeviceEnv(t)
	ctx := context.Background()
	mila, err := e.users.Create(ctx, user.CreateParams{Username: "mila", Role: user.RoleUser})
	if err != nil {
		t.Fatal(err)
	}
	link, err := e.sessions.IssueSetupLink(ctx, e.sam, mila.ID)
	if err != nil {
		t.Fatal(err)
	}
	annaSession := cookieOf(e.login(t, "anna", "pw-anna"), auth.CookieName)
	rec := do(e.h, http.MethodPost, "/api/v1/auth/setup/password",
		`{"token":"`+link.Token+`","password":"mila-new-password"}`, annaSession)
	if rec.Code != http.StatusOK {
		t.Fatalf("redeem: %d %s", rec.Code, rec.Body.String())
	}
	if got := accountIDs(cookieOf(rec, auth.AccountsCookieName)); !slices.Equal(got, []string{mila.ID, e.anna.ID}) {
		t.Fatalf("accounts cookie lists %v, want mila (active), anna", got)
	}
}

// TestEntryBoundToItsSessionsOwner: an entry this server signed but whose
// token belongs to someone else - which only a leaked device key could make
// - neither switches nor lists.
func TestEntryBoundToItsSessionsOwner(t *testing.T) {
	e := newDeviceEnv(t)
	samSession := cookieOf(e.login(t, "sam", "pw"), auth.CookieName)
	forged := auth.NewDevice(deviceKey).Cookie([]auth.DeviceEntry{{UserID: e.anna.ID, Token: samSession.Value}}, false)
	if rec := do(e.h, http.MethodPost, "/api/v1/auth/switch", `{"userId":"`+e.anna.ID+`"}`, &forged); rec.Code != http.StatusNotFound {
		t.Fatalf("switch with sam's token under anna: %d %s", rec.Code, rec.Body.String())
	}
	list, rec := e.accounts(t, &forged)
	if len(list) != 0 {
		t.Fatalf("accounts = %+v, want the forged entry dropped", list)
	}
	if c := cookieOf(rec, auth.AccountsCookieName); c == nil || c.MaxAge >= 0 {
		t.Fatalf("accounts cookie %+v, want it rewritten without the entry", c)
	}
}

// sessionsFor starts n sessions of u and returns them as entries named
// "a", "b", ...: SignedIn takes the names as given.
func (e *deviceEnv) sessionsFor(t *testing.T, u user.User, n int) []auth.DeviceEntry {
	t.Helper()
	var out []auth.DeviceEntry
	for i := range n {
		s, err := e.sessions.StartSession(context.Background(), u)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, auth.DeviceEntry{UserID: string(rune('a' + i)), Token: s.Token})
	}
	return out
}

func (e *deviceEnv) authenticates(t *testing.T, token string) bool {
	t.Helper()
	_, err := e.sessions.Authenticate(context.Background(), token)
	if err != nil && !errors.Is(err, auth.ErrNoSession) {
		t.Fatal(err)
	}
	return err == nil
}

// TestSignedInEndsWhatTheCapDrops: the active person first, then the ten
// most recent others; the eleventh drops, and its session ends with it,
// since no device holds it anymore.
func TestSignedInEndsWhatTheCapDrops(t *testing.T) {
	e := newDeviceEnv(t)
	others := e.sessionsFor(t, e.anna, 11)
	c, err := auth.NewDevice(deviceKey).SignedIn(context.Background(), e.sessions, "", others, auth.DeviceEntry{UserID: e.sam.ID, Token: "ts"}, false)
	if err != nil {
		t.Fatal(err)
	}
	got := accountIDs(&c)
	if len(got) != 11 || got[0] != e.sam.ID || got[1] != "a" || got[10] != "j" {
		t.Fatalf("accounts cookie lists %v, want sam then a..j", got)
	}
	if e.authenticates(t, others[10].Token) {
		t.Fatal("the dropped eleventh session still authenticates")
	}
	for _, o := range others[:10] {
		if !e.authenticates(t, o.Token) {
			t.Fatalf("kept entry %s lost its session", o.UserID)
		}
	}
}

// TestSignInAgainEndsTheOlderSession: sam signs in on top of his own
// session; the token his entry carried until now is on no device anymore.
func TestSignInAgainEndsTheOlderSession(t *testing.T) {
	e := newDeviceEnv(t)
	samSession, annaSession, accounts := e.onTop(t)
	rec := e.login(t, "sam", "pw", samSession, accounts)
	if got := accountIDs(cookieOf(rec, auth.AccountsCookieName)); !slices.Equal(got, []string{e.sam.ID, e.anna.ID}) {
		t.Fatalf("accounts cookie lists %v, want sam, anna", got)
	}
	if e.authenticates(t, samSession.Value) {
		t.Fatal("sam's older session still authenticates")
	}
	if !e.authenticates(t, cookieOf(rec, auth.CookieName).Value) || !e.authenticates(t, annaSession.Value) {
		t.Fatal("a kept session ended")
	}
}

// TestLogoutEndsWhatTheCapDrops: on a full device the leaver goes first and
// the longest-unused other drops, its session with it.
func TestLogoutEndsWhatTheCapDrops(t *testing.T) {
	e := newDeviceEnv(t)
	d := auth.NewDevice(deviceKey)
	samSession := cookieOf(e.login(t, "sam", "pw"), auth.CookieName)
	others := e.sessionsFor(t, e.anna, 10)
	full := d.Cookie(append([]auth.DeviceEntry{{UserID: e.sam.ID, Token: samSession.Value}}, others...), false)
	rec := do(e.h, http.MethodPost, "/api/v1/auth/logout", "", samSession, &full)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("logout: %d", rec.Code)
	}
	if got := accountIDs(cookieOf(rec, auth.AccountsCookieName)); len(got) != 10 || got[0] != e.sam.ID || got[9] != "i" {
		t.Fatalf("accounts cookie lists %v, want sam then a..i", got)
	}
	if e.authenticates(t, others[9].Token) {
		t.Fatal("the dropped session still authenticates")
	}
	if !e.authenticates(t, others[8].Token) {
		t.Fatal("a kept session ended")
	}
}

func TestRemoveDeviceAccountWithNobodyActive(t *testing.T) {
	e := newDeviceEnv(t)
	samSession, annaSession, accounts := e.onTop(t)
	rec := do(e.h, http.MethodPost, "/api/v1/auth/logout", "", samSession, accounts)
	accounts = cookieOf(rec, auth.AccountsCookieName)
	rec = do(e.h, http.MethodDelete, "/api/v1/auth/accounts/"+e.anna.ID, "", accounts)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("remove: %d %s", rec.Code, rec.Body.String())
	}
	if _, err := e.sessions.Authenticate(context.Background(), annaSession.Value); !errors.Is(err, auth.ErrNoSession) {
		t.Fatalf("anna's token still authenticates: %v", err)
	}
	if got := accountIDs(cookieOf(rec, auth.AccountsCookieName)); !slices.Equal(got, []string{e.sam.ID}) {
		t.Fatalf("accounts cookie lists %v, want sam's dead entry alone", got)
	}
}

// TestForeignKeyCookieIsNoDevice: an accounts cookie another instance
// signed, with a real token of this one, is no entry at all here.
func TestForeignKeyCookieIsNoDevice(t *testing.T) {
	e := newDeviceEnv(t)
	samSession, annaSession, _ := e.onTop(t)
	foreign := auth.NewDevice(bytes.Repeat([]byte{2}, 32)).Cookie([]auth.DeviceEntry{{UserID: e.anna.ID, Token: annaSession.Value}}, false)
	if rec := do(e.h, http.MethodPost, "/api/v1/auth/switch", `{"userId":"`+e.anna.ID+`"}`, samSession, &foreign); rec.Code != http.StatusNotFound {
		t.Fatalf("switch: %d %s", rec.Code, rec.Body.String())
	}
	if rec := do(e.h, http.MethodDelete, "/api/v1/auth/accounts/"+e.anna.ID, "", samSession, &foreign); rec.Code != http.StatusNotFound {
		t.Fatalf("remove: %d %s", rec.Code, rec.Body.String())
	}
	// [] decodes to an empty slice, null to nil.
	if list, rec := e.accounts(t, samSession, &foreign); list == nil || len(list) != 0 {
		t.Fatalf("accounts = %q, want []", rec.Body.String())
	}
	if !e.authenticates(t, annaSession.Value) {
		t.Fatal("a foreign cookie ended anna's session")
	}
}
