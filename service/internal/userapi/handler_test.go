package userapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/avatar"
	"github.com/s-frei/rezepte/service/internal/config"
	"github.com/s-frei/rezepte/service/internal/db/dbtest"
	"github.com/s-frei/rezepte/service/internal/httpserver"
	"github.com/s-frei/rezepte/service/internal/image"
	"github.com/s-frei/rezepte/service/internal/mail"
	"github.com/s-frei/rezepte/service/internal/user"
	"github.com/s-frei/rezepte/service/internal/userapi"
)

// newHandler seeds the owner "owner", an admin "sam" and a member "kim" (all
// password "pw") and returns the full-stack handler, following
// internal/recipe/handler_test.go. Nothing is set in the environment, so the
// instance runs on config's own defaults.
func newHandler(t *testing.T) http.Handler {
	t.Helper()
	return newHandlerWithEnv(t, map[string]string{})
}

// newHandlerWithEnv is newHandler with an environment of its own, wired the
// way cmd/rezepte does it: the config carries REZEPTE_LOCALE into the user
// service, so a test can pin the instance language rather than rely on the
// package default behind it.
func newHandlerWithEnv(t *testing.T, environment map[string]string) http.Handler {
	t.Helper()
	h, _ := newStack(t, environment)
	return h
}

// newStack is newHandlerWithEnv that also returns the user service, for a
// test that needs state the API cannot create, such as a linked identity.
func newStack(t *testing.T, environment map[string]string) (http.Handler, *user.Service) {
	t.Helper()
	return newStackWithMailer(t, environment, &fakeMailer{})
}

// fakeMailer stands in for mail.Service: it records what would be sent and
// can be told to fail, so no SMTP server is needed.
type fakeMailer struct {
	enabled bool
	fail    bool
	err     error // returned as is when set
	sent    []mail.Invite
}

func (f *fakeMailer) SendInvite(_ context.Context, in mail.Invite) error {
	if !f.enabled {
		return mail.ErrDisabled
	}
	if f.err != nil {
		return f.err
	}
	if f.fail {
		return fmt.Errorf("%w: 535 nope", mail.ErrSend)
	}
	f.sent = append(f.sent, in)
	return nil
}

func newStackWithMailer(t *testing.T, environment map[string]string, mailer userapi.Mailer) (http.Handler, *user.Service) {
	t.Helper()
	cfg, err := config.LoadFrom(environment)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	conn := dbtest.Open(t)
	users := user.NewService(conn, user.Locale(cfg.Locale))
	for _, seed := range []struct {
		name string
		role user.Role
	}{{"owner", user.RoleSuperadmin}, {"sam", user.RoleAdmin}, {"kim", user.RoleUser}} {
		if _, err := users.Create(context.Background(), user.CreateParams{Username: seed.name, Password: "pw", Role: seed.role}); err != nil {
			t.Fatal(err)
		}
	}
	sessions := auth.NewService(conn, users)
	tokens := auth.NewTokenService(conn, users)
	srv := httpserver.New(cfg, slog.New(slog.DiscardHandler), fstest.MapFS{},
		httpserver.WithAPIMiddleware(auth.Middleware(sessions, tokens, false)))
	auth.Register(srv.API(), sessions, false)
	userapi.Register(srv.API(), users, sessions, avatar.NewService(conn, t.TempDir(), image.NewService(conn, t.TempDir())), cfg.OIDCIssuer, mailer)
	return srv.Handler(), users
}

// tokenEnv is newHandlerWithEnv's counterpart for bearer-token tests: a
// full-stack handler seeded the same way, plus an API token for "sam" (the
// admin) carrying the requested scopes. Follows internal/recipe/handler_test.go's
// tokenEnv.
type tokenEnv struct {
	h     http.Handler
	token string
}

func newTokenEnv(t *testing.T, scopes []string) *tokenEnv {
	t.Helper()
	cfg, err := config.LoadFrom(map[string]string{})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	conn := dbtest.Open(t)
	users := user.NewService(conn, user.Locale(cfg.Locale))
	var samID string
	for _, seed := range []struct {
		name string
		role user.Role
	}{{"owner", user.RoleSuperadmin}, {"sam", user.RoleAdmin}, {"kim", user.RoleUser}} {
		u, err := users.Create(context.Background(), user.CreateParams{Username: seed.name, Password: "pw", Role: seed.role})
		if err != nil {
			t.Fatal(err)
		}
		if seed.name == "sam" {
			samID = u.ID
		}
	}
	sessions := auth.NewService(conn, users)
	tokens := auth.NewTokenService(conn, users)
	srv := httpserver.New(cfg, slog.New(slog.DiscardHandler), fstest.MapFS{},
		httpserver.WithAPIMiddleware(auth.Middleware(sessions, tokens, false)))
	auth.Register(srv.API(), sessions, false)
	userapi.Register(srv.API(), users, sessions, avatar.NewService(conn, t.TempDir(), image.NewService(conn, t.TempDir())), "", &fakeMailer{})
	raw, _, err := tokens.Create(context.Background(), samID, "t", scopes, nil)
	if err != nil {
		t.Fatal(err)
	}
	return &tokenEnv{h: srv.Handler(), token: raw}
}

// do sends a bearer-authenticated request against env's handler.
func (e *tokenEnv) do(method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Host = "localhost:8060"
	req.Header.Set("Origin", "http://localhost:8060")
	req.Header.Set("Authorization", "Bearer "+e.token)
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)
	return rec
}

func doReq(h http.Handler, method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Host = "localhost:8060"
	req.Header.Set("Origin", "http://localhost:8060")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func loginAs(t *testing.T, h http.Handler, username, password string) *http.Cookie {
	t.Helper()
	rec := doReq(h, http.MethodPost, "/api/v1/auth/login", `{"username":"`+username+`","password":"`+password+`"}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("login %s: status %d: %s", username, rec.Code, rec.Body.String())
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.CookieName {
			return c
		}
	}
	t.Fatal("no session cookie set")
	return nil
}

func listUsers(t *testing.T, h http.Handler, cookie *http.Cookie) []userapi.UserAccount {
	t.Helper()
	rec := doReq(h, http.MethodGet, "/api/v1/users", "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status %d: %s", rec.Code, rec.Body.String())
	}
	var list userapi.UserAccountList
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	return list.Items
}

func idOf(t *testing.T, items []userapi.UserAccount, username string) string {
	t.Helper()
	for _, u := range items {
		if u.Username == username {
			return u.ID
		}
	}
	t.Fatalf("no user %q in %+v", username, items)
	return ""
}

func TestUsersRequireAdmin(t *testing.T) {
	h := newHandler(t)
	if rec := doReq(h, http.MethodGet, "/api/v1/users", "", nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous: status %d", rec.Code)
	}
	kim := loginAs(t, h, "kim", "pw")
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/v1/users", ""},
		{http.MethodPost, "/api/v1/users", `{"username":"lea","password":"lea-password","role":"user"}`},
		{http.MethodPatch, "/api/v1/users/x", `{"role":"admin"}`},
		{http.MethodDelete, "/api/v1/users/x", ""},
	} {
		rec := doReq(h, tc.method, tc.path, tc.body, kim)
		if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "admin role required") {
			t.Fatalf("%s %s as member: status %d: %s", tc.method, tc.path, rec.Code, rec.Body.String())
		}
	}
}

func TestPeopleIsReadableByEveryAccount(t *testing.T) {
	h := newHandler(t)
	kim := loginAs(t, h, "kim", "pw")
	rec := doReq(h, http.MethodGet, "/api/v1/people", "", kim)
	if rec.Code != http.StatusOK {
		t.Fatalf("people as member: status %d: %s", rec.Code, rec.Body.String())
	}
	var list userapi.PersonList
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, p := range list.Items {
		got = append(got, p.Username+":"+p.Role)
	}
	want := "kim:user owner:superadmin sam:admin"
	if strings.Join(got, " ") != want {
		t.Fatalf("people = %v, want %s", got, want)
	}
}

// The list is what every account may know about the others, so its shape is
// pinned key by key: a field added to PersonEntry has to be added here on
// purpose, not ride along.
func TestPeopleHidesAccountDetails(t *testing.T) {
	h := newHandler(t)
	kim := loginAs(t, h, "kim", "pw")
	rec := doReq(h, http.MethodGet, "/api/v1/people", "", kim)
	var body struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Items) == 0 {
		t.Fatalf("no people in %s", rec.Body.String())
	}
	want := map[string]bool{"id": true, "username": true, "displayName": true, "color": true, "role": true, "avatarId": true}
	for _, item := range body.Items {
		if len(item) != len(want) {
			t.Fatalf("keys of %v, want exactly %v", item, want)
		}
		for key := range item {
			if !want[key] {
				t.Fatalf("unexpected key %q in %v", key, item)
			}
		}
	}
}

func TestCreateUserWithoutPasswordReturnsASetupLink(t *testing.T) {
	h := newHandler(t)
	c := loginAs(t, h, "sam", "pw")
	rec := doReq(h, http.MethodPost, "/api/v1/users", `{"username":"anna","role":"user"}`, c)
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"path":"/welcome#`) {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
}

func TestCreateUserStillRejectsAShortPassword(t *testing.T) {
	h := newHandler(t)
	c := loginAs(t, h, "sam", "pw")
	rec := doReq(h, http.MethodPost, "/api/v1/users", `{"username":"anna","role":"user","password":"short"}`, c)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "body.password") {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
}

func TestIssueSetupLinkIsSessionOnlyAndRanked(t *testing.T) {
	h := newHandler(t)
	c := loginAs(t, h, "sam", "pw")
	items := listUsers(t, h, c)
	kim := idOf(t, items, "kim")
	owner := idOf(t, items, "owner")
	if rec := doReq(h, http.MethodPost, "/api/v1/users/"+kim+"/setup-link", ``, c); rec.Code != http.StatusCreated {
		t.Fatalf("member: %d %s", rec.Code, rec.Body.String())
	}
	if rec := doReq(h, http.MethodPost, "/api/v1/users/"+owner+"/setup-link", ``, c); rec.Code != http.StatusConflict {
		t.Fatalf("owner: %d", rec.Code)
	}
}

// TestIssueAndRevokeSetupLinkRejectAnonymous pins the 401 an anonymous
// caller gets - no credential at all, which is a different case from a
// bearer token that authenticates but is refused for the operation (see
// TestIssueAndRevokeSetupLinkRejectABearerToken).
func TestIssueAndRevokeSetupLinkRejectAnonymous(t *testing.T) {
	h := newHandler(t)
	c := loginAs(t, h, "sam", "pw")
	kimID := idOf(t, listUsers(t, h, c), "kim")
	if rec := doReq(h, http.MethodPost, "/api/v1/users/"+kimID+"/setup-link", ``, nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous issue: status %d, want 401: %s", rec.Code, rec.Body.String())
	}
	if rec := doReq(h, http.MethodDelete, "/api/v1/users/"+kimID+"/setup-link", ``, nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous revoke: status %d, want 401: %s", rec.Code, rec.Body.String())
	}
}

// TestIssueAndRevokeSetupLinkRejectABearerToken pins that setup-link
// management is session-only like every other credential action: a real
// users:write (and users:read) token - one that can list and would
// otherwise manage users - still cannot issue or revoke a setup link.
// auth.SessionSecurity declares no token scheme at all, so
// auth.Middleware's bearer branch refuses it with 403 before the handler
// ever runs, the same "API tokens cannot use this operation" every other
// session-only write answers with.
func TestIssueAndRevokeSetupLinkRejectABearerToken(t *testing.T) {
	env := newTokenEnv(t, []string{auth.ScopeUsersRead, auth.ScopeUsersWrite})
	rec := env.do(http.MethodGet, "/api/v1/users", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list with token: %d %s", rec.Code, rec.Body.String())
	}
	var list userapi.UserAccountList
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	kimID := idOf(t, list.Items, "kim")

	if rec := env.do(http.MethodPost, "/api/v1/users/"+kimID+"/setup-link", ""); rec.Code != http.StatusForbidden {
		t.Fatalf("issue with token: status %d, want 403: %s", rec.Code, rec.Body.String())
	}
	if rec := env.do(http.MethodDelete, "/api/v1/users/"+kimID+"/setup-link", ""); rec.Code != http.StatusForbidden {
		t.Fatalf("revoke with token: status %d, want 403: %s", rec.Code, rec.Body.String())
	}
}

// TestCreateUserWithBearerTokenRequiresAPassword: an API token has no admin
// dialog to show a setup link through, so create-user refuses to hand one
// out to a bearer caller and asks for a typed password instead, the way the
// form always did.
func TestCreateUserWithBearerTokenRequiresAPassword(t *testing.T) {
	env := newTokenEnv(t, []string{auth.ScopeUsersRead, auth.ScopeUsersWrite})
	rec := env.do(http.MethodPost, "/api/v1/users", `{"username":"anna","role":"user"}`)
	if rec.Code != http.StatusUnprocessableEntity ||
		!strings.Contains(rec.Body.String(), `"location":"body.password"`) ||
		!strings.Contains(rec.Body.String(), "a password is required when creating an account with an API token") {
		t.Fatalf("status %d, body %s", rec.Code, rec.Body.String())
	}

	rec = env.do(http.MethodGet, "/api/v1/users", "")
	if strings.Contains(rec.Body.String(), `"username":"anna"`) {
		t.Fatalf("account created despite the refusal: %s", rec.Body.String())
	}

	// A token with a real password still works exactly like a session.
	rec = env.do(http.MethodPost, "/api/v1/users", `{"username":"anna","role":"user","password":"anna1234"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("with password: status %d, body %s", rec.Code, rec.Body.String())
	}
}

func TestRevokeSetupLinkOverHTTP(t *testing.T) {
	h := newHandler(t)
	c := loginAs(t, h, "sam", "pw")
	rec := doReq(h, http.MethodPost, "/api/v1/users", `{"username":"anna","role":"user"}`, c)
	var created struct {
		ID        string                `json:"id"`
		SetupLink struct{ Path string } `json:"setupLink"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	if !strings.Contains(doReq(h, http.MethodGet, "/api/v1/users", ``, c).Body.String(), `"setupLinkExpiresAt"`) {
		t.Fatal("open link not listed")
	}
	if rec := doReq(h, http.MethodDelete, "/api/v1/users/"+created.ID+"/setup-link", ``, c); rec.Code != http.StatusNoContent {
		t.Fatalf("revoke: %d %s", rec.Code, rec.Body.String())
	}
	token := strings.TrimPrefix(created.SetupLink.Path, "/welcome#")
	if rec := doReq(h, http.MethodPost, "/api/v1/auth/setup/inspect", `{"token":"`+token+`"}`, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("inspect after revoke: %d", rec.Code)
	}
	if strings.Contains(doReq(h, http.MethodGet, "/api/v1/users", ``, c).Body.String(), `"setupLinkExpiresAt"`) {
		t.Fatal("revoked link still listed")
	}
}

func TestPeopleRequiresSignIn(t *testing.T) {
	h := newHandler(t)
	if rec := doReq(h, http.MethodGet, "/api/v1/people", "", nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous: status %d: %s", rec.Code, rec.Body.String())
	}
}

func TestListAndCreate(t *testing.T) {
	h := newHandler(t)
	sam := loginAs(t, h, "sam", "pw")

	// Ordered by username, so the owner sits between kim and sam; it is
	// listed like any other account.
	items := listUsers(t, h, sam)
	if len(items) != 3 || items[0].Username != "kim" ||
		items[1].Username != "owner" || items[1].Role != "superadmin" ||
		items[2].Username != "sam" || items[2].Role != "admin" || items[2].CreatedAt.IsZero() {
		t.Fatalf("items = %+v", items)
	}

	rec := doReq(h, http.MethodPost, "/api/v1/users", `{"username":"KIM","password":"kim-password","role":"user"}`, sam)
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate: status %d: %s", rec.Code, rec.Body.String())
	}
	rec = doReq(h, http.MethodPost, "/api/v1/users", `{"username":"lea","password":"short","role":"user"}`, sam)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "body.password") {
		t.Fatalf("short password: status %d: %s", rec.Code, rec.Body.String())
	}
	rec = doReq(h, http.MethodPost, "/api/v1/users", `{"username":"   ","password":"lea-password","role":"user"}`, sam)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "body.username") {
		t.Fatalf("blank username: status %d: %s", rec.Code, rec.Body.String())
	}
	rec = doReq(h, http.MethodPost, "/api/v1/users", `{"username":"lea","password":"lea-password","role":"user"}`, sam)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: status %d: %s", rec.Code, rec.Body.String())
	}
	var created userapi.UserAccount
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.ID == "" || created.Username != "lea" || created.Role != "user" {
		t.Fatalf("created = %+v", created)
	}
	if len(listUsers(t, h, sam)) != 4 {
		t.Fatal("expected 4 users after create")
	}
	if loginAs(t, h, "lea", "lea-password") == nil {
		t.Fatal("new user cannot log in")
	}
}

func TestUpdateRole(t *testing.T) {
	h := newHandler(t)
	sam := loginAs(t, h, "sam", "pw")
	kimID := idOf(t, listUsers(t, h, sam), "kim")

	// An admin only ever sets a member's role; handing out admin belongs to
	// the owner.
	rec := doReq(h, http.MethodPatch, "/api/v1/users/"+kimID, `{"role":"user"}`, sam)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"role":"user"`) {
		t.Fatalf("set kim's role: status %d: %s", rec.Code, rec.Body.String())
	}
	if rec := doReq(h, http.MethodPatch, "/api/v1/users/"+kimID, `{"role":"admin"}`, sam); rec.Code != http.StatusForbidden {
		t.Fatalf("admin promoting kim: status %d, want 403: %s", rec.Code, rec.Body.String())
	}
	owner := loginAs(t, h, "owner", "pw")
	rec = doReq(h, http.MethodPatch, "/api/v1/users/"+kimID, `{"role":"admin"}`, owner)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"role":"admin"`) {
		t.Fatalf("owner promoting kim: status %d: %s", rec.Code, rec.Body.String())
	}
	rec = doReq(h, http.MethodPatch, "/api/v1/users/"+kimID, `{}`, sam)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("empty body: status %d: %s", rec.Code, rec.Body.String())
	}
	rec = doReq(h, http.MethodPatch, "/api/v1/users/missing", `{"role":"user"}`, sam)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown id: status %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdatePasswordEndsSessions(t *testing.T) {
	h := newHandler(t)
	sam := loginAs(t, h, "sam", "pw")
	kimID := idOf(t, listUsers(t, h, sam), "kim")
	kim1 := loginAs(t, h, "kim", "pw")
	kim2 := loginAs(t, h, "kim", "pw")

	rec := doReq(h, http.MethodPatch, "/api/v1/users/"+kimID, `{"password":"reset-password"}`, sam)
	if rec.Code != http.StatusOK {
		t.Fatalf("reset: status %d: %s", rec.Code, rec.Body.String())
	}
	for i, c := range []*http.Cookie{kim1, kim2} {
		if rec := doReq(h, http.MethodGet, "/api/v1/auth/me", "", c); rec.Code != http.StatusUnauthorized {
			t.Fatalf("kim session %d survived the reset: status %d", i+1, rec.Code)
		}
	}
	if rec := doReq(h, http.MethodPost, "/api/v1/auth/login", `{"username":"kim","password":"pw"}`, nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("old password: status %d", rec.Code)
	}
	loginAs(t, h, "kim", "reset-password")
}

func TestDelete(t *testing.T) {
	h := newHandler(t)
	sam := loginAs(t, h, "sam", "pw")
	items := listUsers(t, h, sam)
	samID, kimID := idOf(t, items, "sam"), idOf(t, items, "kim")
	kim := loginAs(t, h, "kim", "pw")

	if rec := doReq(h, http.MethodDelete, "/api/v1/users/"+samID, "", sam); rec.Code != http.StatusConflict {
		t.Fatalf("self delete: status %d: %s", rec.Code, rec.Body.String())
	}
	if rec := doReq(h, http.MethodDelete, "/api/v1/users/"+kimID, "", sam); rec.Code != http.StatusNoContent {
		t.Fatalf("delete kim: status %d: %s", rec.Code, rec.Body.String())
	}
	if rec := doReq(h, http.MethodGet, "/api/v1/auth/me", "", kim); rec.Code != http.StatusUnauthorized {
		t.Fatalf("kim's session survived the delete: status %d", rec.Code)
	}
	if rec := doReq(h, http.MethodDelete, "/api/v1/users/"+kimID, "", sam); rec.Code != http.StatusNotFound {
		t.Fatalf("delete twice: status %d: %s", rec.Code, rec.Body.String())
	}
	if len(listUsers(t, h, sam)) != 2 {
		t.Fatal("expected the owner and sam to remain")
	}
}

func TestOwnerIsAnAdminForEveryOperation(t *testing.T) {
	h := newHandler(t)
	cookie := loginAs(t, h, "owner", "pw")
	if rec := doReq(h, http.MethodGet, "/api/v1/users", "", cookie); rec.Code != http.StatusOK {
		t.Fatalf("owner listing users: status %d: %s", rec.Code, rec.Body.String())
	}
	create := `{"username":"nia","password":"password1","role":"user"}`
	if rec := doReq(h, http.MethodPost, "/api/v1/users", create, cookie); rec.Code != http.StatusCreated {
		t.Fatalf("owner creating a user: status %d, want 201: %s", rec.Code, rec.Body.String())
	}
	nia := userNamed(t, h, cookie, "nia")
	if rec := doReq(h, http.MethodPatch, "/api/v1/users/"+nia.ID, `{"role":"admin"}`, cookie); rec.Code != http.StatusOK {
		t.Fatalf("owner updating a user: status %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if rec := doReq(h, http.MethodDelete, "/api/v1/users/"+nia.ID, "", cookie); rec.Code != http.StatusNoContent {
		t.Fatalf("owner deleting a user: status %d, want 204: %s", rec.Code, rec.Body.String())
	}
}

func TestOnlyTheOwnerHandsOutAdmin(t *testing.T) {
	h := newHandler(t)
	admin := loginAs(t, h, "sam", "pw")
	body := `{"username":"new","password":"password1","role":"admin"}`
	rec := doReq(h, http.MethodPost, "/api/v1/users", body, admin)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("admin creating an admin: status %d, want 403: %s", rec.Code, rec.Body.String())
	}

	owner := loginAs(t, h, "owner", "pw")
	if rec := doReq(h, http.MethodPost, "/api/v1/users", body, owner); rec.Code != http.StatusCreated {
		t.Fatalf("owner creating an admin: status %d, want 201: %s", rec.Code, rec.Body.String())
	}
}

func TestSuperadminIsNotAnAssignableRole(t *testing.T) {
	h := newHandler(t)
	owner := loginAs(t, h, "owner", "pw")
	create := `{"username":"usurper","password":"password1","role":"superadmin"}`
	if rec := doReq(h, http.MethodPost, "/api/v1/users", create, owner); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("creating a superadmin: status %d, want 422: %s", rec.Code, rec.Body.String())
	}
	kim := userNamed(t, h, owner, "kim")
	patch := `{"role":"superadmin"}`
	if rec := doReq(h, http.MethodPatch, "/api/v1/users/"+kim.ID, patch, owner); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("promoting to superadmin: status %d, want 422: %s", rec.Code, rec.Body.String())
	}
}

func TestTheOwnerRowIsRefusedAsATarget(t *testing.T) {
	h := newHandler(t)
	owner := loginAs(t, h, "owner", "pw")
	admin := loginAs(t, h, "sam", "pw")
	row := userNamed(t, h, owner, "owner")

	for _, tc := range []struct {
		name, method, path, body string
		cookie                   *http.Cookie
	}{
		{"admin demotes the owner", http.MethodPatch, "/api/v1/users/" + row.ID, `{"role":"user"}`, admin},
		{"admin resets the owner", http.MethodPatch, "/api/v1/users/" + row.ID, `{"password":"password1"}`, admin},
		{"admin deletes the owner", http.MethodDelete, "/api/v1/users/" + row.ID, "", admin},
		{"owner resets the owner", http.MethodPatch, "/api/v1/users/" + row.ID, `{"password":"password1"}`, owner},
	} {
		rec := doReq(h, tc.method, tc.path, tc.body, tc.cookie)
		if rec.Code != http.StatusConflict {
			t.Errorf("%s: status %d, want 409: %s", tc.name, rec.Code, rec.Body.String())
		}
	}
}

// userNamed finds a seeded user by name through the list endpoint.
func userNamed(t *testing.T, h http.Handler, cookie *http.Cookie, name string) userapi.UserAccount {
	t.Helper()
	for _, u := range listUsers(t, h, cookie) {
		if u.Username == name {
			return u
		}
	}
	t.Fatalf("no user %q in the list", name)
	return userapi.UserAccount{}
}

func TestCreateUserAcceptsAProfile(t *testing.T) {
	h := newHandler(t)
	owner := loginAs(t, h, "owner", "pw")

	rec := doReq(h, http.MethodPost, "/api/v1/users",
		`{"username":"ida","password":"ida-password","role":"user","displayName":"Ida die Bäckerin","color":"teal"}`, owner)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{`"displayName":"Ida die Bäckerin"`, `"color":"teal"`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("body %s; want it to contain %s", rec.Body.String(), want)
		}
	}
}

func TestCreateUserDefaultsTheProfile(t *testing.T) {
	h := newHandler(t)
	owner := loginAs(t, h, "owner", "pw")

	rec := doReq(h, http.MethodPost, "/api/v1/users",
		`{"username":"ida","password":"ida-password","role":"user"}`, owner)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var created userapi.UserAccount
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if created.DisplayName != "ida" {
		t.Errorf("DisplayName = %q; want the login name", created.DisplayName)
	}
	// Three accounts are seeded, so the fourth color of the palette is the
	// least used one.
	if created.Color != string(user.Colors[3]) {
		t.Errorf("Color = %q; want %q", created.Color, user.Colors[3])
	}
}

func TestOnlyTheOwnerWritesAnotherProfile(t *testing.T) {
	h := newHandler(t)
	sam := loginAs(t, h, "sam", "pw")
	owner := loginAs(t, h, "owner", "pw")
	kimID := idOf(t, listUsers(t, h, sam), "kim")

	rec := doReq(h, http.MethodPatch, "/api/v1/users/"+kimID, `{"displayName":"Umbenannt"}`, sam)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("admin renaming kim: status %d, want 403: %s", rec.Code, rec.Body.String())
	}

	rec = doReq(h, http.MethodPatch, "/api/v1/users/"+kimID, `{"displayName":"Umbenannt","color":"plum"}`, owner)
	if rec.Code != http.StatusOK {
		t.Fatalf("owner renaming kim: status %d: %s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{`"displayName":"Umbenannt"`, `"color":"plum"`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("body %s; want it to contain %s", rec.Body.String(), want)
		}
	}
}

func TestARefusedProfileChangesNothing(t *testing.T) {
	h := newHandler(t)
	sam := loginAs(t, h, "sam", "pw")
	kimID := idOf(t, listUsers(t, h, sam), "kim")

	// The password alone would be allowed; the display name in the same body
	// is not, and the refusal comes before any write.
	rec := doReq(h, http.MethodPatch, "/api/v1/users/"+kimID,
		`{"password":"a-brand-new-password","displayName":"Umbenannt"}`, sam)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403: %s", rec.Code, rec.Body.String())
	}
	if name := idOf(t, listUsers(t, h, sam), "kim"); name == "" {
		t.Fatal("kim is gone")
	}
	for _, u := range listUsers(t, h, sam) {
		if u.Username == "kim" && u.DisplayName != "kim" {
			t.Errorf("DisplayName = %q; want it unchanged", u.DisplayName)
		}
	}
	// The old password still works, so the password half did not land either.
	loginAs(t, h, "kim", "pw")
}

func TestUpdateUserNeedsSomethingToChange(t *testing.T) {
	h := newHandler(t)
	owner := loginAs(t, h, "owner", "pw")
	kimID := idOf(t, listUsers(t, h, owner), "kim")

	rec := doReq(h, http.MethodPatch, "/api/v1/users/"+kimID, `{}`, owner)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status %d, want 422: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateUserAcceptsALocale(t *testing.T) {
	h := newHandler(t)
	owner := loginAs(t, h, "owner", "pw")

	rec := doReq(h, http.MethodPost, "/api/v1/users",
		`{"username":"gina","password":"gina1234","role":"user","locale":"de"}`, owner)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d, body %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"locale":"de"`) {
		t.Errorf("body = %s, want locale de", rec.Body.String())
	}
}

func TestCreateUserWithoutLocaleUsesTheInstanceDefault(t *testing.T) {
	// German, so the assertion cannot be satisfied by user.BaseLocale - the
	// fallback an unconfigured instance would land on anyway.
	h := newHandlerWithEnv(t, map[string]string{"REZEPTE_LOCALE": "de"})
	owner := loginAs(t, h, "owner", "pw")

	rec := doReq(h, http.MethodPost, "/api/v1/users",
		`{"username":"hugo","password":"hugo1234","role":"user"}`, owner)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d, body %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"locale":"de"`) {
		t.Errorf("body = %s, want locale de", rec.Body.String())
	}
}

// The admin path takes no locale on purpose: the interface language is the
// account holder's own choice, changed through PATCH /auth/me/profile and
// nowhere else. Admins reset passwords and set roles; they do not pick
// somebody's language. The field is simply absent from updateInput, and huma
// rejects an unknown property, so the refusal is the schema's rather than a
// rule anyone has to remember.
func TestUpdateUserRefusesALocaleBecauseLanguageIsTheAccountHoldersAlone(t *testing.T) {
	h := newHandler(t)
	owner := loginAs(t, h, "owner", "pw")
	kimID := idOf(t, listUsers(t, h, owner), "kim")

	for _, body := range []string{`{"locale":"de"}`, `{"role":"admin","locale":"de"}`} {
		rec := doReq(h, http.MethodPatch, "/api/v1/users/"+kimID, body, owner)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("PATCH %s: status %d, want 422: %s", body, rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), `"location":"body.locale"`) {
			t.Errorf("PATCH %s: body = %s, want an error on body.locale", body, rec.Body.String())
		}
	}

	// And nothing was written on the way to that refusal.
	kim := loginAs(t, h, "kim", "pw")
	rec := doReq(h, http.MethodGet, "/api/v1/auth/me", "", kim)
	if !strings.Contains(rec.Body.String(), `"locale":"en"`) {
		t.Errorf("kim = %s, want the instance default locale en", rec.Body.String())
	}
}

// TestOpenAPIDeclaresRetryAfter checks that the writes that hash a password
// say in the document how long a client turned away with 503 should wait.
func TestOpenAPIDeclaresRetryAfter(t *testing.T) {
	rec := doReq(newHandler(t), http.MethodGet, "/api/v1/openapi.json", "", nil)
	var doc struct {
		Paths map[string]map[string]struct {
			Responses map[string]struct {
				Headers map[string]json.RawMessage `json:"headers"`
			} `json:"responses"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	for _, want := range []struct{ path, method string }{
		{"/api/v1/users", "post"},
		{"/api/v1/users/{id}", "patch"},
	} {
		if _, ok := doc.Paths[want.path][want.method].Responses["503"].Headers["Retry-After"]; !ok {
			t.Errorf("%s %s 503: no Retry-After header declared", want.method, want.path)
		}
	}
}

// Two operations list accounts, so the document says which one a reader
// wants: list-people for who takes part, list-users for managing accounts.
func TestOpenAPISaysWhyThereAreTwoLists(t *testing.T) {
	rec := doReq(newHandler(t), http.MethodGet, "/api/v1/openapi.json", "", nil)
	var doc struct {
		Paths map[string]map[string]struct {
			Description string `json:"description"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if got := doc.Paths["/api/v1/people"]["get"].Description; !strings.Contains(got, "/api/v1/users") {
		t.Fatalf("list-people description %q does not point at /api/v1/users", got)
	}
}

// TestAdminsWithdrawPublicSharing covers canSharePublicly on PATCH: an admin
// turns it off for a member and the account shows it, a member may not touch
// it, and the owner's own right cannot be withdrawn - the owner governs public
// sharing for the whole instance.
func TestAdminsWithdrawPublicSharing(t *testing.T) {
	h := newHandler(t)
	owner := loginAs(t, h, "owner", "pw")
	admin := loginAs(t, h, "sam", "pw")
	member := loginAs(t, h, "kim", "pw")
	kim := userNamed(t, h, owner, "kim")
	if !kim.CanSharePublicly {
		t.Fatalf("a new member may not share publicly: %+v", kim)
	}

	rec := doReq(h, http.MethodPatch, "/api/v1/users/"+kim.ID, `{"canSharePublicly":false}`, admin)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"canSharePublicly":false`) {
		t.Errorf("admin withdraws: status %d: %s", rec.Code, rec.Body.String())
	}
	if got := userNamed(t, h, owner, "kim"); got.CanSharePublicly {
		t.Errorf("the list still says kim may share publicly")
	}

	rec = doReq(h, http.MethodPatch, "/api/v1/users/"+kim.ID, `{"canSharePublicly":true}`, member)
	if rec.Code != http.StatusForbidden {
		t.Errorf("member restores their own right: status %d, want 403: %s", rec.Code, rec.Body.String())
	}

	row := userNamed(t, h, owner, "owner")
	rec = doReq(h, http.MethodPatch, "/api/v1/users/"+row.ID, `{"canSharePublicly":false}`, admin)
	if rec.Code != http.StatusConflict {
		t.Errorf("admin withdraws from the owner: status %d, want 409: %s", rec.Code, rec.Body.String())
	}
}

// TestOnlyTheOwnerSwitchesAnAdminsSharing: canSharePublicly follows the rank
// rule of every other write on an account, so an admin reaches neither
// another admin's row nor their own, and the owner reaches both.
func TestOnlyTheOwnerSwitchesAnAdminsSharing(t *testing.T) {
	h := newHandler(t)
	owner := loginAs(t, h, "owner", "pw")
	sam := loginAs(t, h, "sam", "pw")
	rec := doReq(h, http.MethodPost, "/api/v1/users", `{"username":"ren","password":"ren-password","role":"admin"}`, owner)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create second admin: status %d: %s", rec.Code, rec.Body.String())
	}
	ren := loginAs(t, h, "ren", "ren-password")
	renAccount := userNamed(t, h, owner, "ren")

	for what, caller := range map[string]*http.Cookie{"ren on their own row": ren, "sam on ren's row": sam} {
		rec = doReq(h, http.MethodPatch, "/api/v1/users/"+renAccount.ID, `{"canSharePublicly":false}`, caller)
		if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "superadmin role required") {
			t.Errorf("%s: status %d, want 403: %s", what, rec.Code, rec.Body.String())
		}
	}

	rec = doReq(h, http.MethodPatch, "/api/v1/users/"+renAccount.ID, `{"canSharePublicly":false}`, owner)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"canSharePublicly":false`) {
		t.Errorf("owner withdraws ren's right: status %d: %s", rec.Code, rec.Body.String())
	}
}

// TestRefusedPatchWritesNothing: a body that mixes canSharePublicly with a
// password reset is refused as a whole when the target outranks the caller -
// the right is not withdrawn behind a 403.
func TestRefusedPatchWritesNothing(t *testing.T) {
	h := newHandler(t)
	owner := loginAs(t, h, "owner", "pw")
	sam := loginAs(t, h, "sam", "pw")
	rec := doReq(h, http.MethodPost, "/api/v1/users", `{"username":"ren","password":"ren-password","role":"admin"}`, owner)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create second admin: status %d: %s", rec.Code, rec.Body.String())
	}
	renAccount := userNamed(t, h, owner, "ren")

	rec = doReq(h, http.MethodPatch, "/api/v1/users/"+renAccount.ID, `{"canSharePublicly":false,"password":"new-password-1"}`, sam)
	if rec.Code != http.StatusForbidden {
		t.Errorf("sam withdraws and resets ren: status %d, want 403: %s", rec.Code, rec.Body.String())
	}
	if got := userNamed(t, h, owner, "ren"); !got.CanSharePublicly {
		t.Error("ren lost the right to share although the request was refused")
	}
}

// TestListUsersReportsLinkedIdentities: hasIdentity means an identity at the
// configured issuer - the one sign-in and disconnect use. One left behind at
// an earlier provider does not count, and nothing counts with OIDC off.
func TestListUsersReportsLinkedIdentities(t *testing.T) {
	oidcEnv := map[string]string{
		"REZEPTE_OIDC_ISSUER": "https://id.example", "REZEPTE_OIDC_CLIENT_ID": "rezepte",
		"REZEPTE_PUBLIC_URL": "http://localhost:8060",
	}
	for _, tc := range []struct {
		name        string
		environment map[string]string
		want        map[string]bool
	}{
		{"configured issuer", oidcEnv, map[string]bool{"kim": true}},
		{"OIDC off", map[string]string{}, map[string]bool{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, users := newStack(t, tc.environment)
			cookie := loginAs(t, h, "sam", "pw")
			list := listUsers(t, h, cookie)
			ctx := context.Background()
			if err := users.LinkIdentity(ctx, idOf(t, list, "kim"), "https://id.example", "sub-kim"); err != nil {
				t.Fatal(err)
			}
			if err := users.LinkIdentity(ctx, idOf(t, list, "owner"), "https://old.example", "sub-owner"); err != nil {
				t.Fatal(err)
			}
			for _, u := range listUsers(t, h, cookie) {
				if u.HasIdentity != tc.want[u.Username] {
					t.Errorf("%s: hasIdentity = %v", u.Username, u.HasIdentity)
				}
			}
		})
	}
}

func createMember(t *testing.T, h http.Handler, c *http.Cookie, name, email string) string {
	t.Helper()
	rec := doReq(h, http.MethodPost, "/api/v1/users", `{"username":"`+name+`","role":"user","email":"`+email+`"}`, c)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create %s = %d %s", name, rec.Code, rec.Body)
	}
	var out userapi.CreatedUserAccount
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out.ID
}

func TestCreateUserMailsLink(t *testing.T) {
	m := &fakeMailer{enabled: true}
	h, _ := newStackWithMailer(t, map[string]string{}, m)
	c := loginAs(t, h, "sam", "pw")
	rec := doReq(h, "POST", "/api/v1/users", `{"username":"lena","role":"user","email":"lena@example.org"}`, c)
	if rec.Code != 201 || !strings.Contains(rec.Body.String(), `"mailedTo":"lena@example.org"`) {
		t.Fatalf("create = %d %s", rec.Code, rec.Body)
	}
	if len(m.sent) != 1 || !strings.HasPrefix(m.sent[0].Path, "/welcome#") || m.sent[0].Inviter == "" {
		t.Fatalf("sent = %+v", m.sent)
	}
}

func TestCreateUserMailFailureKeepsAccount(t *testing.T) {
	m := &fakeMailer{enabled: true, fail: true}
	h, _ := newStackWithMailer(t, map[string]string{}, m)
	c := loginAs(t, h, "sam", "pw")
	rec := doReq(h, "POST", "/api/v1/users", `{"username":"lena","role":"user","email":"lena@example.org"}`, c)
	body := rec.Body.String()
	// mailedTo names the address the failed send was for.
	if rec.Code != 201 || !strings.Contains(body, `"mailError":"send_failed"`) || !strings.Contains(body, `"mailedTo":"lena@example.org"`) ||
		!strings.Contains(body, `"setupLink"`) || strings.Contains(body, "535") {
		t.Fatalf("create = %d %s", rec.Code, body)
	}
}

func TestCreateUserInternalMailErrorReportsNothing(t *testing.T) {
	m := &fakeMailer{enabled: true, err: errors.New("render: boom")}
	h, _ := newStackWithMailer(t, map[string]string{}, m)
	c := loginAs(t, h, "sam", "pw")
	rec := doReq(h, "POST", "/api/v1/users", `{"username":"lena","role":"user","email":"lena@example.org"}`, c)
	if rec.Code != 201 || strings.Contains(rec.Body.String(), "mailedTo") || strings.Contains(rec.Body.String(), "mailError") {
		t.Fatalf("create = %d %s", rec.Code, rec.Body)
	}
}

func TestCreateUserNoMailWhenDisabled(t *testing.T) {
	m := &fakeMailer{enabled: false}
	h, _ := newStackWithMailer(t, map[string]string{}, m)
	c := loginAs(t, h, "sam", "pw")
	rec := doReq(h, "POST", "/api/v1/users", `{"username":"lena","role":"user","email":"lena@example.org"}`, c)
	if rec.Code != 201 || strings.Contains(rec.Body.String(), "mailedTo") || strings.Contains(rec.Body.String(), "mailError") || len(m.sent) != 0 {
		t.Fatalf("create = %d %s sent=%d", rec.Code, rec.Body, len(m.sent))
	}
	if !strings.Contains(rec.Body.String(), `"email":"lena@example.org"`) {
		t.Fatal("address not stored")
	}
}

func TestIssueSetupLinkMailFalse(t *testing.T) {
	m := &fakeMailer{enabled: true}
	h, _ := newStackWithMailer(t, map[string]string{}, m)
	c := loginAs(t, h, "sam", "pw")
	id := createMember(t, h, c, "lena", "lena@example.org")
	m.sent = nil
	rec := doReq(h, "POST", "/api/v1/users/"+id+"/setup-link", `{"mail":false}`, c)
	if rec.Code != 201 || len(m.sent) != 0 {
		t.Fatalf("issue = %d sent=%d", rec.Code, len(m.sent))
	}
	rec = doReq(h, "POST", "/api/v1/users/"+id+"/setup-link", ``, c)
	if rec.Code != 201 || len(m.sent) != 1 || !strings.Contains(rec.Body.String(), `"mailedTo"`) {
		t.Fatalf("issue default = %d sent=%d %s", rec.Code, len(m.sent), rec.Body)
	}
}

func TestUpdateUserEmailRankRule(t *testing.T) {
	h := newHandler(t)
	ids := map[string]string{}
	owner := loginAs(t, h, "owner", "pw")
	for _, u := range listUsers(t, h, owner) {
		ids[u.Username] = u.ID
	}
	for _, tc := range []struct {
		caller, target string
		want           int
	}{
		{"sam", "kim", 200},
		{"sam", "sam", 403},
		{"sam", "owner", 409},
		{"owner", "sam", 200},
		{"owner", "owner", 409},
		{"owner", "kim", 200},
		{"kim", "kim", 403},
	} {
		rec := doReq(h, "PATCH", "/api/v1/users/"+ids[tc.target], `{"email":"x@example.org"}`, loginAs(t, h, tc.caller, "pw"))
		if rec.Code != tc.want {
			t.Errorf("%s -> %s = %d, want %d: %s", tc.caller, tc.target, rec.Code, tc.want, rec.Body)
		}
	}
	if got := userNamed(t, h, owner, "owner"); got.Email != "" {
		t.Errorf("refused PATCH left owner address %q", got.Email)
	}
}

func TestUpdateUserEmailClearsVerified(t *testing.T) {
	h, users := newStack(t, map[string]string{})
	c := loginAs(t, h, "sam", "pw")
	kim := idOf(t, listUsers(t, h, c), "kim")
	if err := users.SetEmailIfEmpty(context.Background(), kim, "kim@example.org", true); err != nil {
		t.Fatal(err)
	}
	if got := userNamed(t, h, c, "kim"); !got.EmailVerified {
		t.Fatalf("precondition: %+v", got)
	}
	if rec := doReq(h, "PATCH", "/api/v1/users/"+kim, `{"email":"other@example.org"}`, c); rec.Code != 200 {
		t.Fatalf("patch = %d %s", rec.Code, rec.Body)
	}
	got := userNamed(t, h, c, "kim")
	if got.EmailVerified || got.Email != "other@example.org" {
		t.Fatalf("after change: %+v", got)
	}
}

func TestUpdateUserEmailIsSessionOnly(t *testing.T) {
	env := newTokenEnv(t, []string{"users:write"})
	rec := env.do("PATCH", "/api/v1/users/anything", `{"email":"x@example.org"}`)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("token PATCH email = %d %s", rec.Code, rec.Body)
	}
}
