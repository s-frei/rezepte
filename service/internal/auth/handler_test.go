package auth_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/config"
	"github.com/s-frei/rezepte/service/internal/db/dbtest"
	"github.com/s-frei/rezepte/service/internal/httpserver"
	"github.com/s-frei/rezepte/service/internal/user"
)

// newHandlerWithSessions is like newHandler but also returns the auth.Service
// so tests can control its clock (e.g. for sliding-renewal assertions).
func newHandlerWithSessions(t *testing.T) (http.Handler, *auth.Service) {
	t.Helper()
	conn := dbtest.Open(t)
	users := user.NewService(conn)
	if _, err := users.Create(context.Background(), user.CreateParams{Username: "sam", Password: "pw", Role: user.RoleAdmin}); err != nil {
		t.Fatal(err)
	}
	cfg, _ := config.LoadFrom(map[string]string{})
	sessions := auth.NewService(conn, users)
	tokens := auth.NewTokenService(conn, users)
	srv := httpserver.New(cfg, slog.New(slog.DiscardHandler), fstest.MapFS{},
		httpserver.WithAPIMiddleware(auth.Middleware(sessions, tokens, false)),
		httpserver.WithSecuritySchemes(auth.SecuritySchemes()))
	auth.Register(srv.API(), sessions, false)
	return srv.Handler(), sessions
}

func newHandler(t *testing.T) http.Handler {
	t.Helper()
	h, _ := newHandlerWithSessions(t)
	return h
}

func do(h http.Handler, method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
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

func login(t *testing.T, h http.Handler) *http.Cookie {
	t.Helper()
	rec := do(h, http.MethodPost, "/api/v1/auth/login", `{"username":"sam","password":"pw"}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("login status %d: %s", rec.Code, rec.Body.String())
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.CookieName {
			if !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.Path != "/" {
				t.Fatalf("cookie flags wrong: %+v", c)
			}
			return c
		}
	}
	t.Fatal("no session cookie set")
	return nil
}

func TestLoginRejectsWrongPassword(t *testing.T) {
	rec := do(newHandler(t), http.MethodPost, "/api/v1/auth/login", `{"username":"sam","password":"x"}`, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/problem+json") {
		t.Fatalf("content type %q", ct)
	}
}

func TestLoginValidatesBody(t *testing.T) {
	rec := do(newHandler(t), http.MethodPost, "/api/v1/auth/login", `{"username":""}`, nil)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
}

func TestMeRequiresSession(t *testing.T) {
	rec := do(newHandler(t), http.MethodGet, "/api/v1/auth/me", "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestLoginMeLogout(t *testing.T) {
	h := newHandler(t)
	cookie := login(t, h)

	rec := do(h, http.MethodGet, "/api/v1/auth/me", "", cookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"username":"sam"`) {
		t.Fatalf("me: %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"role":"admin"`) {
		t.Fatalf("me lacks role: %s", rec.Body.String())
	}

	rec = do(h, http.MethodPost, "/api/v1/auth/logout", "", cookie)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("logout: %d %s", rec.Code, rec.Body.String())
	}
	cleared := false
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.CookieName && c.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Fatal("logout did not clear the cookie")
	}

	rec = do(h, http.MethodGet, "/api/v1/auth/me", "", cookie)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("me after logout: %d", rec.Code)
	}
}

// sessionCookieFrom returns the rezepte_session cookie set on rec, if any.
func sessionCookieFrom(rec *httptest.ResponseRecorder) (*http.Cookie, bool) {
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.CookieName {
			return c, true
		}
	}
	return nil, false
}

// TestSlidingRenewalReissuesCookie verifies that a sliding-window renewal
// (internal/auth.Service.Authenticate reporting Renewed) reaches the
// browser: the middleware must append a fresh Set-Cookie header, since
// extending the row in the database alone does nothing for a client holding
// the login-time cookie.
func TestSlidingRenewalReissuesCookie(t *testing.T) {
	h, sessions := newHandlerWithSessions(t)

	clock := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	sessions.SetClock(func() time.Time { return clock })

	cookie := login(t, h)

	// Day 0: immediately after login, no renewal is due yet, so /auth/me
	// must not set a new cookie.
	rec := do(h, http.MethodGet, "/api/v1/auth/me", "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("me day 0: %d %s", rec.Code, rec.Body.String())
	}
	if _, ok := sessionCookieFrom(rec); ok {
		t.Fatalf("day 0: unexpected Set-Cookie: %+v", rec.Result().Cookies())
	}

	// Day 10: sliding renewal kicks in, so /auth/me must set a fresh cookie
	// expiring ~30 days from now (day 40).
	clock = clock.Add(10 * 24 * time.Hour)
	rec = do(h, http.MethodGet, "/api/v1/auth/me", "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("me day 10: %d %s", rec.Code, rec.Body.String())
	}
	renewed, ok := sessionCookieFrom(rec)
	if !ok {
		t.Fatal("day 10: no Set-Cookie, sliding renewal was not sent to the browser")
	}
	wantExpiry := clock.Add(auth.SessionTTL)
	if diff := renewed.Expires.Sub(wantExpiry); diff < -time.Minute || diff > time.Minute {
		t.Fatalf("renewed cookie Expires = %v, want ~%v (day 40)", renewed.Expires, wantExpiry)
	}
	if !renewed.HttpOnly || renewed.SameSite != http.SameSiteLaxMode {
		t.Fatalf("renewed cookie flags wrong: %+v", renewed)
	}
}

func TestOpenAPIDeclaresSessionSecurity(t *testing.T) {
	rec := do(newHandler(t), http.MethodGet, "/api/v1/openapi.json", "", nil)
	body := rec.Body.String()
	if !strings.Contains(body, `"session"`) || !strings.Contains(body, `"in":"cookie"`) {
		t.Fatalf("security scheme missing: %s", body)
	}
}

// pathParam matches OpenAPI {param}-style path segments.
var pathParam = regexp.MustCompile(`\{[^}]+\}`)

// TestEveryProtectedOperationRejectsAnonymous guards against a future
// operation declaring Security: auth.SessionSecurity without actually being
// protected. httpserver.WithAPIMiddleware makes that structurally hard to
// get wrong (see httpserver.TestWithAPIMiddlewareAppliesBeforeRegistration),
// but this test still exercises every currently registered operation.
func TestEveryProtectedOperationRejectsAnonymous(t *testing.T) {
	conn := dbtest.Open(t)
	users := user.NewService(conn)
	cfg, _ := config.LoadFrom(map[string]string{})
	sessions := auth.NewService(conn, users)
	tokens := auth.NewTokenService(conn, users)
	srv := httpserver.New(cfg, slog.New(slog.DiscardHandler), fstest.MapFS{},
		httpserver.WithAPIMiddleware(auth.Middleware(sessions, tokens, false)))
	auth.Register(srv.API(), sessions, false)
	h := srv.Handler()

	tested := 0
	for path, item := range srv.API().OpenAPI().Paths {
		for method, op := range map[string]*huma.Operation{
			http.MethodGet:     item.Get,
			http.MethodPut:     item.Put,
			http.MethodPost:    item.Post,
			http.MethodDelete:  item.Delete,
			http.MethodPatch:   item.Patch,
			http.MethodOptions: item.Options,
			http.MethodHead:    item.Head,
			http.MethodTrace:   item.Trace,
		} {
			if op == nil || len(op.Security) == 0 {
				continue
			}
			tested++
			concretePath := pathParam.ReplaceAllString(path, "x")
			rec := do(h, method, concretePath, "", nil)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("%s %s: status %d, want 401", method, concretePath, rec.Code)
			}
			if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/problem+json") {
				t.Fatalf("%s %s: content type %q, want application/problem+json prefix", method, concretePath, ct)
			}
		}
	}
	if tested == 0 {
		t.Fatal("no protected operations found to exercise")
	}
}

func TestChangeOwnPassword(t *testing.T) {
	h := newHandler(t)
	c1 := login(t, h)
	c2 := login(t, h)

	rec := do(h, http.MethodPatch, "/api/v1/auth/me", `{"currentPassword":"wrong","password":"brand-new-pw"}`, c1)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), `"location":"body.currentPassword"`) {
		t.Fatalf("wrong current: status %d: %s", rec.Code, rec.Body.String())
	}
	rec = do(h, http.MethodPatch, "/api/v1/auth/me", `{"currentPassword":"pw","password":"short"}`, c1)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "body.password") {
		t.Fatalf("short new password: status %d: %s", rec.Code, rec.Body.String())
	}
	rec = do(h, http.MethodPatch, "/api/v1/auth/me", `{"currentPassword":"pw","password":"brand-new-pw"}`, c1)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("change: status %d: %s", rec.Code, rec.Body.String())
	}
	if rec := do(h, http.MethodGet, "/api/v1/auth/me", "", c1); rec.Code != http.StatusOK {
		t.Fatalf("acting session must survive: status %d", rec.Code)
	}
	if rec := do(h, http.MethodGet, "/api/v1/auth/me", "", c2); rec.Code != http.StatusUnauthorized {
		t.Fatalf("other session must be gone: status %d", rec.Code)
	}
	if rec := do(h, http.MethodPost, "/api/v1/auth/login", `{"username":"sam","password":"brand-new-pw"}`, nil); rec.Code != http.StatusOK {
		t.Fatalf("login with new password: status %d: %s", rec.Code, rec.Body.String())
	}
	if rec := do(h, http.MethodPatch, "/api/v1/auth/me", `{"currentPassword":"pw","password":"brand-new-pw"}`, nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous: status %d", rec.Code)
	}
}

func TestUpdateOwnProfile(t *testing.T) {
	h := newHandler(t)
	cookie := login(t, h)

	rec := do(h, http.MethodPatch, "/api/v1/auth/me/profile",
		`{"displayName":"Sam der Koch","color":"teal"}`, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{`"displayName":"Sam der Koch"`, `"color":"teal"`, `"username":"sam"`} {
		if !strings.Contains(body, want) {
			t.Errorf("body %s; want it to contain %s", body, want)
		}
	}

	// The change is stored, not just echoed.
	rec = do(h, http.MethodGet, "/api/v1/auth/me", "", cookie)
	if !strings.Contains(rec.Body.String(), `"displayName":"Sam der Koch"`) {
		t.Errorf("me: %s", rec.Body.String())
	}
}

func TestUpdateOwnProfileSetsTheColorAlone(t *testing.T) {
	h := newHandler(t)
	cookie := login(t, h)

	rec := do(h, http.MethodPatch, "/api/v1/auth/me/profile", `{"color":"sage"}`, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"displayName":"sam"`) {
		t.Errorf("display name changed: %s", rec.Body.String())
	}
}

func TestUpdateOwnProfileRefusesAnEmptyBody(t *testing.T) {
	h := newHandler(t)
	cookie := login(t, h)

	rec := do(h, http.MethodPatch, "/api/v1/auth/me/profile", `{}`, cookie)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status %d, want 422: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateOwnProfileRefusesAnUnknownColor(t *testing.T) {
	h := newHandler(t)
	cookie := login(t, h)

	rec := do(h, http.MethodPatch, "/api/v1/auth/me/profile", `{"color":"chartreuse"}`, cookie)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status %d, want 422: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateOwnProfileRefusesATooLongName(t *testing.T) {
	h := newHandler(t)
	cookie := login(t, h)

	rec := do(h, http.MethodPatch, "/api/v1/auth/me/profile",
		`{"displayName":"`+strings.Repeat("x", 65)+`"}`, cookie)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status %d, want 422: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateOwnProfileRequiresSession(t *testing.T) {
	rec := do(newHandler(t), http.MethodPatch, "/api/v1/auth/me/profile", `{"displayName":"x"}`, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d, want 401", rec.Code)
	}
}

func TestListColorUsageReturnsThePaletteInOrder(t *testing.T) {
	h := newHandler(t)
	cookie := login(t, h)

	rec := do(h, http.MethodGet, "/api/v1/auth/me/colors", "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Items []user.ColorCount `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Items) != len(user.Colors) {
		t.Fatalf("len %d, want %d: %s", len(body.Items), len(user.Colors), rec.Body.String())
	}
	for i, c := range user.Colors {
		if body.Items[i].Color != c {
			t.Fatalf("items[%d] = %q, want %q", i, body.Items[i].Color, c)
		}
	}
	// The seeded "sam" holds the first color of the palette, nobody holds
	// the second.
	if body.Items[0].Count != 1 || body.Items[1].Count != 0 {
		t.Errorf("counts %+v; want 1 and 0", body.Items[:2])
	}
}

func TestListColorUsageRequiresSession(t *testing.T) {
	rec := do(newHandler(t), http.MethodGet, "/api/v1/auth/me/colors", "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d, want 401", rec.Code)
	}
}

func TestMeReturnsTheLocale(t *testing.T) {
	h := newHandler(t)
	cookie := login(t, h)

	rec := do(h, http.MethodGet, "/api/v1/auth/me", "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, body %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"locale":"en"`) {
		t.Errorf("body = %s, want locale en", rec.Body.String())
	}
}

func TestUpdateOwnProfileChangesTheLocale(t *testing.T) {
	h := newHandler(t)
	cookie := login(t, h)

	rec := do(h, http.MethodPatch, "/api/v1/auth/me/profile", `{"locale":"de"}`, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, body %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"locale":"de"`) {
		t.Errorf("body = %s, want locale de", rec.Body.String())
	}
}

func TestUpdateOwnProfileRejectsAnUnknownLocale(t *testing.T) {
	h := newHandler(t)
	cookie := login(t, h)

	rec := do(h, http.MethodPatch, "/api/v1/auth/me/profile", `{"locale":"xx"}`, cookie)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status %d, want 422; body %s", rec.Code, rec.Body.String())
	}
	// 422 alone would also be huma's answer to a property the schema does not
	// know at all, so it cannot tell a working enum from a misspelled field.
	// Naming the location is what pins the enum.
	var problem struct {
		Errors []struct {
			Location string `json:"location"`
			Message  string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatalf("decode body %s: %v", rec.Body.String(), err)
	}
	var found bool
	for _, e := range problem.Errors {
		if e.Location == "body.locale" {
			found = true
			if strings.Contains(e.Message, "unexpected property") {
				t.Errorf("body.locale is not a known property: %s", e.Message)
			}
		}
	}
	if !found {
		t.Errorf("no error on body.locale; body %s", rec.Body.String())
	}
}

func TestLoginSetsTheLocaleCookie(t *testing.T) {
	h := newHandler(t)

	rec := do(h, http.MethodPost, "/api/v1/auth/login", `{"username":"sam","password":"pw"}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}

	var got *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.LocaleCookieName {
			got = c
		}
	}
	if got == nil {
		t.Fatalf("no %s cookie in %v", auth.LocaleCookieName, rec.Header())
	}
	if got.Value != "en" {
		t.Errorf("value = %q, want en", got.Value)
	}
	if got.HttpOnly {
		t.Error("cookie is HttpOnly; Paraglide reads it from JavaScript")
	}
	if got.Path != "/" {
		t.Errorf("path = %q, want /", got.Path)
	}
}

func TestChangingTheLocaleRewritesTheCookie(t *testing.T) {
	h := newHandler(t)
	cookie := login(t, h)

	rec := do(h, http.MethodPatch, "/api/v1/auth/me/profile", `{"locale":"de"}`, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.LocaleCookieName {
			if c.Value != "de" {
				t.Errorf("value = %q, want de", c.Value)
			}
			return
		}
	}
	t.Fatalf("no %s cookie in %v", auth.LocaleCookieName, rec.Header())
}

func TestLogoutClearsTheLocaleCookie(t *testing.T) {
	h := newHandler(t)
	cookie := login(t, h)

	rec := do(h, http.MethodPost, "/api/v1/auth/logout", "", cookie)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("logout: status %d, body %s", rec.Code, rec.Body.String())
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.LocaleCookieName {
			if c.MaxAge >= 0 {
				t.Errorf("MaxAge = %d, want negative (cleared)", c.MaxAge)
			}
			return
		}
	}
	t.Fatalf("no %s cookie in %v", auth.LocaleCookieName, rec.Header())
}
