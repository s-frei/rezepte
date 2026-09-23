package userapi_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/config"
	"github.com/s-frei/rezepte/service/internal/db/dbtest"
	"github.com/s-frei/rezepte/service/internal/httpserver"
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
	cfg, err := config.LoadFrom(environment)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	conn := dbtest.Open(t)
	users := user.NewService(conn, user.WithDefaultLocale(user.Locale(cfg.Locale)))
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
	userapi.Register(srv.API(), users, sessions)
	return srv.Handler()
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
	// Three accounts are seeded, so the fourth colour of the palette is the
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
