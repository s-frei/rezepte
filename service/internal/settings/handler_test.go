package settings_test

import (
	"context"
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
	"github.com/s-frei/rezepte/service/internal/settings"
	"github.com/s-frei/rezepte/service/internal/user"
)

func newHandler(t *testing.T) http.Handler {
	t.Helper()
	conn := dbtest.Open(t)
	users := user.NewService(conn, "")
	for name, role := range map[string]user.Role{"olga": user.RoleSuperadmin, "adam": user.RoleAdmin, "mia": user.RoleUser} {
		if _, err := users.Create(context.Background(), user.CreateParams{Username: name, Password: "pw", Role: role}); err != nil {
			t.Fatal(err)
		}
	}
	cfg, _ := config.LoadFrom(map[string]string{})
	sessions := auth.NewService(conn, users)
	tokens := auth.NewTokenService(conn, users)
	srv := httpserver.New(cfg, slog.New(slog.DiscardHandler), fstest.MapFS{},
		httpserver.WithAPIMiddleware(auth.Middleware(sessions, tokens, false)))
	auth.Register(srv.API(), sessions, false)
	settings.Register(srv.API(), settings.NewService(conn), func(context.Context) bool { return false })
	return srv.Handler()
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

func login(t *testing.T, h http.Handler, username string) *http.Cookie {
	t.Helper()
	rec := do(h, http.MethodPost, "/api/v1/auth/login", `{"username":"`+username+`","password":"pw"}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("login %s: %d %s", username, rec.Code, rec.Body.String())
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.CookieName {
			return c
		}
	}
	t.Fatal("no session cookie")
	return nil
}

func TestSettingsEndpoints(t *testing.T) {
	h := newHandler(t)

	rec := do(h, http.MethodGet, "/api/v1/settings", "", login(t, h, "mia"))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"recipesLockedByDefault":false`) || !strings.Contains(rec.Body.String(), `"mailEnabled":false`) {
		t.Errorf("get as member: %d %s", rec.Code, rec.Body.String())
	}

	body := `{"recipesLockedByDefault":true}`
	for _, name := range []string{"mia", "adam"} {
		rec = do(h, http.MethodPatch, "/api/v1/settings", body, login(t, h, name))
		if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "superadmin role required") {
			t.Errorf("patch as %s: %d %s", name, rec.Code, rec.Body.String())
		}
	}

	rec = do(h, http.MethodPatch, "/api/v1/settings", body, login(t, h, "olga"))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"recipesLockedByDefault":true`) {
		t.Errorf("patch as owner: %d %s", rec.Code, rec.Body.String())
	}

	rec = do(h, http.MethodPatch, "/api/v1/settings", `{"linkPreviews":true}`, login(t, h, "olga"))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"linkPreviews":true`) ||
		!strings.Contains(rec.Body.String(), `"recipesLockedByDefault":true`) {
		t.Errorf("patch one field as owner: %d %s", rec.Code, rec.Body.String())
	}

	rec = do(h, http.MethodPatch, "/api/v1/settings", `{"linkPreviewMinutes":30}`, login(t, h, "olga"))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("patch with an unknown lifetime: %d %s", rec.Code, rec.Body.String())
	}

	rec = do(h, http.MethodGet, "/api/v1/settings", "", login(t, h, "olga"))
	if strings.Contains(strings.ToLower(rec.Body.String()), "key") {
		t.Errorf("settings expose the signing key: %s", rec.Body.String())
	}

	rec = do(h, http.MethodPatch, "/api/v1/settings", body, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("patch without session: %d", rec.Code)
	}
}

func TestPublicShareSettingsEndpoint(t *testing.T) {
	h := newHandler(t)
	owner := login(t, h, "olga")

	rec := do(h, http.MethodPatch, "/api/v1/settings", `{"publicShares":true,"publicShareMaxDays":30}`, owner)
	if rec.Code != http.StatusOK ||
		!strings.Contains(rec.Body.String(), `"publicShares":true`) ||
		!strings.Contains(rec.Body.String(), `"publicShareDefaultDays":30`) ||
		!strings.Contains(rec.Body.String(), `"publicShareMaxDays":30`) {
		t.Errorf("patch public share settings: %d %s", rec.Code, rec.Body.String())
	}

	rec = do(h, http.MethodPatch, "/api/v1/settings", `{"publicShareAttribution":false}`, owner)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"publicShareAttribution":false`) {
		t.Errorf("patch share attribution: %d %s", rec.Code, rec.Body.String())
	}

	rec = do(h, http.MethodPatch, "/api/v1/settings", `{"publicShareMaxDays":2}`, owner)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("patch with an unknown share lifetime: %d %s", rec.Code, rec.Body.String())
	}

	// An explicit null on one lifetime field makes it permanent again and
	// leaves the other untouched - the whole point of nullableDay: a plain
	// *int could never tell "send null" apart from "field absent" once
	// publicShareMaxDays had already been set to 30 above.
	rec = do(h, http.MethodPatch, "/api/v1/settings", `{"publicShareMaxDays":null}`, owner)
	if rec.Code != http.StatusOK ||
		!strings.Contains(rec.Body.String(), `"publicShareDefaultDays":30`) ||
		!strings.Contains(rec.Body.String(), `"publicShareMaxDays":null`) {
		t.Errorf("null the maximum: %d %s", rec.Code, rec.Body.String())
	}

	// The maximum is null now, so nulling the default too needs no clamping:
	// both end up permanent.
	rec = do(h, http.MethodPatch, "/api/v1/settings", `{"publicShareDefaultDays":null}`, owner)
	if rec.Code != http.StatusOK ||
		!strings.Contains(rec.Body.String(), `"publicShareDefaultDays":null`) ||
		!strings.Contains(rec.Body.String(), `"publicShareMaxDays":null`) {
		t.Errorf("null the default while the maximum is already null: %d %s", rec.Code, rec.Body.String())
	}

	// Set the maximum back to 30, then null the default while a maximum is
	// in force: SetShareLifetimes clamps a nil default up to the maximum
	// rather than refusing the request, so the default comes back as 30,
	// not null.
	rec = do(h, http.MethodPatch, "/api/v1/settings", `{"publicShareMaxDays":30}`, owner)
	if rec.Code != http.StatusOK {
		t.Fatalf("set the maximum back to 30: %d %s", rec.Code, rec.Body.String())
	}
	rec = do(h, http.MethodPatch, "/api/v1/settings", `{"publicShareDefaultDays":null}`, owner)
	if rec.Code != http.StatusOK ||
		!strings.Contains(rec.Body.String(), `"publicShareDefaultDays":30`) ||
		!strings.Contains(rec.Body.String(), `"publicShareMaxDays":30`) {
		t.Errorf("null the default while the maximum is 30 (clamped, not permanent): %d %s", rec.Code, rec.Body.String())
	}

	// An empty body names neither field, so both lifetimes stay exactly as
	// they are - the "absent" half of the three-way distinction.
	rec = do(h, http.MethodPatch, "/api/v1/settings", `{}`, owner)
	if rec.Code != http.StatusOK ||
		!strings.Contains(rec.Body.String(), `"publicShareDefaultDays":30`) ||
		!strings.Contains(rec.Body.String(), `"publicShareMaxDays":30`) {
		t.Errorf("empty body: %d %s", rec.Code, rec.Body.String())
	}
}
