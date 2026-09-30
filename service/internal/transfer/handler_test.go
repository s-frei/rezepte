package transfer_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/config"
	"github.com/s-frei/rezepte/service/internal/db/dbtest"
	"github.com/s-frei/rezepte/service/internal/httpserver"
	"github.com/s-frei/rezepte/service/internal/image"
	"github.com/s-frei/rezepte/service/internal/recipe"
	"github.com/s-frei/rezepte/service/internal/transfer"
	"github.com/s-frei/rezepte/service/internal/user"
)

type harness struct {
	h       http.Handler
	tokens  *auth.TokenService
	ids     map[string]string
	dataDir string
}

// newHandler mirrors internal/image/handler_test.go's stack and adds a
// member, kim, and the transfer operations exactly as main.go wires them.
func newHandler(t *testing.T) harness {
	t.Helper()
	ctx := context.Background()
	conn := dbtest.Open(t)
	users := user.NewService(conn, "")
	sam, err := users.Create(ctx, user.CreateParams{Username: "sam", Password: "pw", Role: user.RoleAdmin})
	if err != nil {
		t.Fatal(err)
	}
	kim, err := users.Create(ctx, user.CreateParams{Username: "kim", Password: "pw", Role: user.RoleUser})
	if err != nil {
		t.Fatal(err)
	}
	cfg, _ := config.LoadFrom(map[string]string{})
	sessions := auth.NewService(conn, users)
	tokens := auth.NewTokenService(conn, users)
	srv := httpserver.New(cfg, slog.New(slog.DiscardHandler), fstest.MapFS{},
		httpserver.WithAPIMiddleware(auth.Middleware(sessions, tokens, false)))
	auth.Register(srv.API(), sessions, false)
	dataDir := t.TempDir()
	dir := filepath.Join(dataDir, "images")
	recipes := recipe.NewService(conn, dir)
	recipe.Register(srv.API(), recipes)
	images := image.NewService(conn, dir)
	image.Register(srv.API(), images)
	transfer.Register(srv.API(), transfer.NewService(recipes, images, dataDir, transfer.InputValidator(srv.API())))
	return harness{h: srv.Handler(), tokens: tokens, ids: map[string]string{"sam": sam.ID, "kim": kim.ID}, dataDir: dataDir}
}

// doReq sends a POST, the only method these operations take.
func doReq(h http.Handler, path string, body []byte, contentType string, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Host = "localhost:8060"
	req.Header.Set("Origin", "http://localhost:8060")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func doToken(h http.Handler, path string, body []byte, contentType, raw string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "Bearer "+raw)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func loginCookie(t *testing.T, h http.Handler, username string) *http.Cookie {
	t.Helper()
	rec := doReq(h, "/api/v1/auth/login", []byte(`{"username":"`+username+`","password":"pw"}`), "application/json", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("login status %d: %s", rec.Code, rec.Body.String())
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.CookieName {
			return c
		}
	}
	t.Fatal("no session cookie")
	return nil
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func zipWith(t *testing.T, name string, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// assertNoTempFiles fails when an export or import left its temp zip in
// the data directory.
func assertNoTempFiles(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".zip") {
			t.Errorf("temp file left behind: %s", e.Name())
		}
	}
}

func TestExportImportOverHTTP(t *testing.T) {
	env := newHandler(t)
	h := env.h
	c := loginCookie(t, h, "sam")
	rec := doReq(h, "/api/v1/recipes", mustJSON(t, sampleInput("Focaccia")), "application/json", c)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	var r struct{ ID string }
	if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil {
		t.Fatal(err)
	}

	rec = doReq(h, "/api/v1/export", []byte(`{"recipeIds":["`+r.ID+`"]}`), "application/json", c)
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/zip" {
		t.Fatalf("export: %d %s", rec.Code, rec.Header())
	}
	if rec.Header().Get("Content-Length") != strconv.Itoa(rec.Body.Len()) {
		t.Fatalf("content-length %q for %d bytes", rec.Header().Get("Content-Length"), rec.Body.Len())
	}
	if !strings.HasPrefix(rec.Header().Get("Content-Disposition"), `attachment; filename="rezepte-`) {
		t.Fatalf("disposition = %q", rec.Header().Get("Content-Disposition"))
	}
	assertNoTempFiles(t, env.dataDir)

	rec = doReq(h, "/api/v1/import", rec.Body.Bytes(), "application/zip", c)
	if rec.Code != http.StatusCreated {
		t.Fatalf("import: %d %s", rec.Code, rec.Body)
	}
	var out struct{ Created []transfer.Created }
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Created) != 1 || out.Created[0].Slug != "focaccia-2" {
		t.Fatalf("created = %+v", out.Created)
	}
	assertNoTempFiles(t, env.dataDir)
}

func TestTransferAccess(t *testing.T) {
	env := newHandler(t)
	h := env.h
	kim := loginCookie(t, h, "kim")
	for _, path := range []string{"/api/v1/export", "/api/v1/import"} {
		if rec := doReq(h, path, []byte(`{"recipeIds":["x"]}`), "application/json", kim); rec.Code != http.StatusForbidden {
			t.Fatalf("%s as member: %d", path, rec.Code)
		}
	}
	// The member is refused before the body is looked at: a declared length
	// over the cap would otherwise answer 413.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/import", strings.NewReader("x"))
	req.ContentLength = 1<<30 + 1
	req.Header.Set("Content-Type", "application/zip")
	req.Host = "localhost:8060"
	req.Header.Set("Origin", "http://localhost:8060")
	req.AddCookie(kim)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("oversized import as member: %d", rr.Code)
	}
	ctx := context.Background()
	memberToken, _, err := env.tokens.Create(ctx, env.ids["kim"], "t", []string{auth.ScopeRecipesRead, auth.ScopeRecipesWrite}, nil)
	if err != nil {
		t.Fatal(err)
	}
	// A member's token is inert (auth.TokenService.Authenticate refuses a
	// non-admin owner), so it never reaches spoolBody: 401, not 403.
	if rec := doToken(h, "/api/v1/import", []byte("PK"), "application/zip", memberToken); rec.Code != http.StatusUnauthorized {
		t.Fatalf("import with member token: %d", rec.Code)
	}
	readOnly, _, err := env.tokens.Create(ctx, env.ids["sam"], "t", []string{auth.ScopeRecipesRead}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if rec := doToken(h, "/api/v1/import", []byte("PK"), "application/zip", readOnly); rec.Code != http.StatusForbidden {
		t.Fatalf("import with read-only admin token: %d", rec.Code)
	}
	assertNoTempFiles(t, env.dataDir)
}

func TestExportRejectsBadIDs(t *testing.T) {
	env := newHandler(t)
	h := env.h
	c := loginCookie(t, h, "sam")
	if rec := doReq(h, "/api/v1/export", []byte(`{"recipeIds":["0190aaaa-0000-7000-8000-000000000000"]}`), "application/json", c); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown id: %d", rec.Code)
	}
	if rec := doReq(h, "/api/v1/export", []byte(`{"recipeIds":["a","a"]}`), "application/json", c); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("duplicate id: %d", rec.Code)
	}
	if rec := doReq(h, "/api/v1/export", []byte(`{"recipeIds":[]}`), "application/json", c); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("empty: %d", rec.Code)
	}
	assertNoTempFiles(t, env.dataDir)
}

func TestImportRejectsWithLocationAndTooLarge(t *testing.T) {
	env := newHandler(t)
	h := env.h
	c := loginCookie(t, h, "sam")
	rec := doReq(h, "/api/v1/import", zipWith(t, "a/notes.txt", []byte("x")), "application/zip", c)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), `"location":"a/notes.txt"`) {
		t.Fatalf("bad entry: %d %s", rec.Code, rec.Body)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/import", strings.NewReader("x"))
	req.ContentLength = 1<<30 + 1
	req.Header.Set("Content-Type", "application/zip")
	req.Host = "localhost:8060"
	req.Header.Set("Origin", "http://localhost:8060")
	req.AddCookie(c)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("too large: %d", rr.Code)
	}
	assertNoTempFiles(t, env.dataDir)
}

func TestImportValidatesAgainstTheInputSchema(t *testing.T) {
	env := newHandler(t)
	h := env.h
	c := loginCookie(t, h, "sam")
	raw := []byte(`{"format":"rezepte.recipe","version":1,"title":"","description":"","servings":2,"prepMinutes":null,"cookMinutes":null,"sourceUrl":null,"tags":[],"steps":[],"ingredientGroups":[{"name":null,"ingredients":[{"quantity":null,"unit":null,"name":"Mehl","note":null}]}],"images":[],"cover":null}`)
	rec := doReq(h, "/api/v1/import", zipWith(t, "a/recipe.json", raw), "application/zip", c)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "a/recipe.json: body.title") {
		t.Fatalf("schema: %d %s", rec.Code, rec.Body)
	}
}
