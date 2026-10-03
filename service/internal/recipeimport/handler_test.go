package recipeimport_test

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/config"
	"github.com/s-frei/rezepte/service/internal/db/dbtest"
	"github.com/s-frei/rezepte/service/internal/httpserver"
	"github.com/s-frei/rezepte/service/internal/recipe"
	"github.com/s-frei/rezepte/service/internal/recipeimport"
	"github.com/s-frei/rezepte/service/internal/user"
)

type harness struct {
	h       http.Handler
	tokens  *auth.TokenService
	users   *user.Service
	recipes *recipe.Service
	samID   string
	cookie  *http.Cookie
}

// newHandler wires the stack as main.go does, with a member session for sam.
func newHandler(t *testing.T, allow func(netip.Addr) bool) harness {
	t.Helper()
	ctx := context.Background()
	conn := dbtest.Open(t)
	users := user.NewService(conn, "")
	sam, err := users.Create(ctx, user.CreateParams{Username: "sam", Password: "pw", Role: user.RoleUser})
	if err != nil {
		t.Fatal(err)
	}
	cfg, _ := config.LoadFrom(map[string]string{})
	sessions := auth.NewService(conn, users)
	tokens := auth.NewTokenService(conn, users)
	srv := httpserver.New(cfg, slog.New(slog.DiscardHandler), fstest.MapFS{},
		httpserver.WithAPIMiddleware(auth.Middleware(sessions, tokens, false)))
	auth.Register(srv.API(), sessions, false)
	recipes := recipe.NewService(conn, t.TempDir())
	recipe.Register(srv.API(), recipes)
	recipeimport.Register(srv.API(), recipeimport.NewService(recipes, "test", allow))
	h := harness{h: srv.Handler(), tokens: tokens, users: users, recipes: recipes, samID: sam.ID}
	rec := h.do(http.MethodPost, "/api/v1/auth/login", `{"username":"sam","password":"pw"}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("login status %d: %s", rec.Code, rec.Body.String())
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.CookieName {
			h.cookie = c
		}
	}
	if h.cookie == nil {
		t.Fatal("no session cookie")
	}
	return h
}

func (h harness) do(method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Host = "localhost:8060"
	req.Header.Set("Origin", "http://localhost:8060")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	h.h.ServeHTTP(rec, req)
	return rec
}

func (h harness) post(t *testing.T, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	return h.do(http.MethodPost, "/api/v1/recipe-drafts", string(b), h.cookie)
}

type draftBody struct {
	Recipe struct {
		Title string `json:"title"`
	} `json:"recipe"`
	Review []struct {
		Line string `json:"line"`
	} `json:"review"`
	Photo *struct {
		Href string `json:"href"`
	} `json:"photo"`
	Duplicate *struct {
		Title string `json:"title"`
	} `json:"duplicate"`
}

type problem struct {
	Errors []struct {
		Location string `json:"location"`
		Message  string `json:"message"`
	} `json:"errors"`
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return v
}

func wantProblem(t *testing.T, rec *httptest.ResponseRecorder, location, message string) {
	t.Helper()
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	p := decode[problem](t, rec)
	if len(p.Errors) == 0 || p.Errors[0].Location != location || p.Errors[0].Message != message {
		t.Fatalf("errors = %+v, want %s %q", p.Errors, location, message)
	}
}

func pngBytes(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// site serves the soup page (canonical rewritten to the site itself), its
// photo and a page without a recipe.
func site(t *testing.T) (*httptest.Server, []byte) {
	t.Helper()
	soup, err := os.ReadFile("testdata/pages/graph-typelist.html")
	if err != nil {
		t.Fatal(err)
	}
	article, err := os.ReadFile("testdata/pages/no-recipe.html")
	if err != nil {
		t.Fatal(err)
	}
	img := pngBytes(t)
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/soup":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(strings.ReplaceAll(string(soup), "https://example.com", srv.URL)))
		case "/img/soup.jpg":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(img)
		case "/latin1":
			w.Header().Set("Content-Type", "text/html; charset=iso-8859-1")
			_, _ = w.Write([]byte("<html><head><script type=\"application/ld+json\">" +
				"{\"@type\":\"Recipe\",\"name\":\"K\xe4sesuppe\",\"recipeIngredient\":[\"200 g K\xe4se\"],\"recipeInstructions\":\"R\xfchren.\"}" +
				"</script></head></html>"))
		case "/article":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write(article)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, img
}

func TestDraftFromURL(t *testing.T) {
	h := newHandler(t, allowAll)
	srv, _ := site(t)
	rec := h.post(t, map[string]string{"url": srv.URL + "/soup"})
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	d := decode[draftBody](t, rec)
	if d.Recipe.Title != "Graph Soup" {
		t.Fatalf("title = %q", d.Recipe.Title)
	}
	if d.Photo == nil || !strings.HasPrefix(d.Photo.Href, "/api/v1/recipe-drafts/photo?src=") {
		t.Fatalf("photo = %+v", d.Photo)
	}
	if d.Duplicate != nil {
		t.Fatalf("duplicate = %+v", d.Duplicate)
	}
}

func TestDraftFromLatin1Page(t *testing.T) {
	h := newHandler(t, allowAll)
	srv, _ := site(t)
	rec := h.post(t, map[string]string{"url": srv.URL + "/latin1"})
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	d := decode[draftBody](t, rec)
	if d.Recipe.Title != "Käsesuppe" {
		t.Fatalf("title = %q", d.Recipe.Title)
	}
}

func TestDraftPhoto(t *testing.T) {
	h := newHandler(t, allowAll)
	srv, img := site(t)
	d := decode[draftBody](t, h.post(t, map[string]string{"url": srv.URL + "/soup"}))
	if d.Photo == nil {
		t.Fatal("no photo")
	}

	rec := h.do(http.MethodGet, d.Photo.Href, "", h.cookie)
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/png" || !bytes.Equal(rec.Body.Bytes(), img) {
		t.Fatalf("status %d, type %q, %d bytes", rec.Code, rec.Header().Get("Content-Type"), rec.Body.Len())
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q", got)
	}

	u, err := url.Parse(d.Photo.Href)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	// Flip the MAC's first character: its last one carries padding bits a
	// decoder may ignore.
	token := []byte(q.Get("token"))
	i := bytes.IndexByte(token, '.') + 1
	if token[i] == 'A' {
		token[i] = 'B'
	} else {
		token[i] = 'A'
	}
	bad := url.Values{"src": {q.Get("src")}, "token": {string(token)}}
	other := url.Values{"src": {srv.URL + "/other.jpg"}, "token": {q.Get("token")}}
	for name, v := range map[string]url.Values{"flipped token": bad, "other src": other} {
		rec := h.do(http.MethodGet, u.Path+"?"+v.Encode(), "", h.cookie)
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: status %d: %s", name, rec.Code, rec.Body.String())
		}
	}
}

func TestDraftPageWithoutRecipe(t *testing.T) {
	h := newHandler(t, allowAll)
	srv, _ := site(t)
	wantProblem(t, h.post(t, map[string]string{"url": srv.URL + "/article"}), "body.url", "no-recipe")
}

func TestDraftUnreachable(t *testing.T) {
	h := newHandler(t, recipeimport.PublicOnly)
	wantProblem(t, h.post(t, map[string]string{"url": "http://127.0.0.1:1/"}), "body.url", "unreachable")
}

func TestDraftFromText(t *testing.T) {
	h := newHandler(t, allowAll)
	rec := h.post(t, map[string]string{"text": "Pancakes\nIngredients\n2 eggs\nMethod\nFry."})
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	d := decode[draftBody](t, rec)
	if d.Recipe.Title != "Pancakes" || len(d.Review) != 0 || d.Photo != nil {
		t.Fatalf("draft = %+v", d)
	}
}

func TestDraftTextWithoutRecipe(t *testing.T) {
	h := newHandler(t, allowAll)
	wantProblem(t, h.post(t, map[string]string{"text": "hello"}), "body.text", "no-recipe-in-text")
}

func TestDraftNeedsExactlyOneInput(t *testing.T) {
	h := newHandler(t, allowAll)
	for _, body := range []map[string]string{{}, {"url": "https://example.com/", "text": "x"}} {
		wantProblem(t, h.post(t, body), "body", "give exactly one of url and text")
	}
}

func TestDraftFindsDuplicate(t *testing.T) {
	for name, stored := range map[string]string{
		"canonical":          "/graph-soup",
		"entered with utm_*": "/soup?utm_source=x",
	} {
		t.Run(name, func(t *testing.T) {
			h := newHandler(t, allowAll)
			srv, _ := site(t)
			in, err := recipe.Samples("de")
			if err != nil {
				t.Fatal(err)
			}
			src := srv.URL + stored
			in[0].Title = "Meine Suppe"
			in[0].SourceURL = &src
			if _, err := h.recipes.Create(context.Background(), h.samID, in[0]); err != nil {
				t.Fatal(err)
			}
			d := decode[draftBody](t, h.post(t, map[string]string{"url": srv.URL + "/soup"}))
			if d.Duplicate == nil || d.Duplicate.Title != "Meine Suppe" {
				t.Fatalf("duplicate = %+v", d.Duplicate)
			}
		})
	}
}

func TestDraftNeedsWriteScope(t *testing.T) {
	h := newHandler(t, allowAll)
	if rec := h.do(http.MethodPost, "/api/v1/recipe-drafts", `{"text":"x"}`, nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous status %d: %s", rec.Code, rec.Body.String())
	}
	// API tokens belong to admins; sam is a plain member.
	ada, err := h.users.Create(context.Background(), user.CreateParams{Username: "ada", Password: "pw", Role: user.RoleAdmin})
	if err != nil {
		t.Fatal(err)
	}
	raw, _, err := h.tokens.Create(context.Background(), ada.ID, "r", []string{auth.ScopeRecipesRead}, nil)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/recipe-drafts", strings.NewReader(`{"text":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Host = "localhost:8060"
	req.Header.Set("Origin", "http://localhost:8060")
	req.Header.Set("Authorization", "Bearer "+raw)
	rec := httptest.NewRecorder()
	h.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("read-only token status %d: %s", rec.Code, rec.Body.String())
	}
}

func TestDraftTextTooLong(t *testing.T) {
	h := newHandler(t, allowAll)
	rec := h.post(t, map[string]string{"text": strings.Repeat("a", 64*1024+1)})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status %d", rec.Code)
	}
}

// TestDraftSchemaSaysNullable: photo and duplicate are objects or null on the
// wire, and the OpenAPI document must say so (a oneOf with a null branch);
// review and suggestedTags are always arrays.
func TestDraftSchemaSaysNullable(t *testing.T) {
	h := newHandler(t, allowAll)
	rec := h.do(http.MethodGet, "/api/v1/openapi.json", "", h.cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var doc struct {
		Components struct {
			Schemas map[string]struct {
				Properties map[string]struct {
					Type  json.RawMessage `json:"type"`
					OneOf []struct {
						Ref  string `json:"$ref"`
						Type string `json:"type"`
					} `json:"oneOf"`
				} `json:"properties"`
			} `json:"schemas"`
		} `json:"components"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	out, ok := doc.Components.Schemas["DraftOutputBody"]
	if !ok {
		t.Fatalf("no DraftOutputBody schema in %v", doc.Components.Schemas)
	}
	for _, name := range []string{"photo", "duplicate"} {
		var obj, null bool
		for _, b := range out.Properties[name].OneOf {
			obj = obj || b.Ref != ""
			null = null || b.Type == "null"
		}
		if !obj || !null {
			t.Errorf("%s: oneOf = %+v, want an object ref and null", name, out.Properties[name].OneOf)
		}
	}
	for _, name := range []string{"review", "suggestedTags"} {
		if got := string(out.Properties[name].Type); got != `"array"` {
			t.Errorf("%s: type = %s, want \"array\" (never null on the wire)", name, got)
		}
	}
	if _, ok := doc.Components.Schemas["DraftReview"]; !ok {
		t.Error("no DraftReview schema")
	}
}
