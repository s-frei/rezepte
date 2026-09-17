package recipe_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/config"
	"github.com/s-frei/rezepte/service/internal/db/dbtest"
	"github.com/s-frei/rezepte/service/internal/httpserver"
	"github.com/s-frei/rezepte/service/internal/recipe"
	"github.com/s-frei/rezepte/service/internal/user"
)

// newRecipeHandler builds a full-stack handler (auth + recipe operations)
// backed by a fresh, migrated database, following the pattern in
// internal/auth/handler_test.go.
func newRecipeHandler(t *testing.T) http.Handler {
	t.Helper()
	conn := dbtest.Open(t)
	users := user.NewService(conn)
	if _, err := users.Create(context.Background(), "sam", "pw", user.RoleAdmin); err != nil {
		t.Fatal(err)
	}
	cfg, _ := config.LoadFrom(map[string]string{})
	sessions := auth.NewService(conn, users)
	srv := httpserver.New(cfg, slog.New(slog.DiscardHandler), fstest.MapFS{},
		httpserver.WithAPIMiddleware(auth.Middleware(sessions, false)))
	auth.Register(srv.API(), sessions, false)
	recipe.Register(srv.API(), recipe.NewService(conn))
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

func loginCookie(t *testing.T, h http.Handler) *http.Cookie {
	t.Helper()
	rec := doReq(h, http.MethodPost, "/api/v1/auth/login", `{"username":"sam","password":"pw"}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("login status %d: %s", rec.Code, rec.Body.String())
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.CookieName {
			return c
		}
	}
	t.Fatal("no session cookie set")
	return nil
}

func mustMarshal(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestRecipesRequireSession(t *testing.T) {
	h := newRecipeHandler(t)
	rec := doReq(h, http.MethodGet, "/api/v1/recipes", "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/problem+json") {
		t.Fatalf("content type %q", ct)
	}
}

func TestCreateListGetUpdateDelete(t *testing.T) {
	h := newRecipeHandler(t)
	cookie := loginCookie(t, h)
	fx := loadFixtures(t)[0] // Königsberger Klopse

	rec := doReq(h, http.MethodPost, "/api/v1/recipes", mustMarshal(t, fx), cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status %d: %s", rec.Code, rec.Body.String())
	}
	var created recipe.Recipe
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Slug != "koenigsberger-klopse" || created.ID == "" || created.CreatedBy == "" {
		t.Fatalf("created = %+v", created)
	}
	if created.Images == nil {
		t.Fatal("images must be an empty array, not null")
	}

	rec = doReq(h, http.MethodGet, "/api/v1/recipes", "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status %d: %s", rec.Code, rec.Body.String())
	}
	var page recipe.Page
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(page.Items) != 1 {
		t.Fatalf("page = %+v", page)
	}
	card := page.Items[0]
	if card.Slug != created.Slug || card.Title != fx.Title || card.TotalMinutes == nil || *card.TotalMinutes != 60 {
		t.Fatalf("card = %+v", card)
	}

	rec = doReq(h, http.MethodGet, "/api/v1/recipes/by-slug/koenigsberger-klopse", "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("by-slug status %d: %s", rec.Code, rec.Body.String())
	}
	var bySlug recipe.Recipe
	if err := json.Unmarshal(rec.Body.Bytes(), &bySlug); err != nil {
		t.Fatal(err)
	}
	if len(bySlug.IngredientGroups) != 2 || bySlug.ID != created.ID {
		t.Fatalf("bySlug = %+v", bySlug)
	}

	rec = doReq(h, http.MethodGet, "/api/v1/recipes/"+created.ID, "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("get status %d: %s", rec.Code, rec.Body.String())
	}

	updateIn := fx
	updateIn.Title = "Königsberger Klopse deluxe"
	rec = doReq(h, http.MethodPut, "/api/v1/recipes/"+created.ID, mustMarshal(t, updateIn), cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("update status %d: %s", rec.Code, rec.Body.String())
	}
	var updated recipe.Recipe
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatal(err)
	}
	if updated.Title != "Königsberger Klopse deluxe" || updated.Slug != created.Slug {
		t.Fatalf("updated = %+v", updated)
	}

	rec = doReq(h, http.MethodDelete, "/api/v1/recipes/"+created.ID, "", cookie)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete status %d: %s", rec.Code, rec.Body.String())
	}

	rec = doReq(h, http.MethodGet, "/api/v1/recipes/"+created.ID, "", cookie)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("get after delete status %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateValidation(t *testing.T) {
	h := newRecipeHandler(t)
	cookie := loginCookie(t, h)
	fx := loadFixtures(t)[0]

	emptyTitle := fx
	emptyTitle.Title = ""
	rec := doReq(h, http.MethodPost, "/api/v1/recipes", mustMarshal(t, emptyTitle), cookie)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("empty title status %d: %s", rec.Code, rec.Body.String())
	}
	var problem struct {
		Errors []struct {
			Location string `json:"location"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatal(err)
	}
	if len(problem.Errors) == 0 || !strings.Contains(problem.Errors[0].Location, "body.title") {
		t.Fatalf("errors = %+v, body = %s", problem.Errors, rec.Body.String())
	}

	zeroServings := fx
	zeroServings.Servings = 0
	rec = doReq(h, http.MethodPost, "/api/v1/recipes", mustMarshal(t, zeroServings), cookie)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("zero servings status %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTags(t *testing.T) {
	h := newRecipeHandler(t)
	cookie := loginCookie(t, h)
	fx := loadFixtures(t)

	for _, i := range []int{0, 1} { // Königsberger Klopse, Rinderrouladen
		rec := doReq(h, http.MethodPost, "/api/v1/recipes", mustMarshal(t, fx[i]), cookie)
		if rec.Code != http.StatusCreated {
			t.Fatalf("create %d status %d: %s", i, rec.Code, rec.Body.String())
		}
	}

	rec := doReq(h, http.MethodGet, "/api/v1/tags", "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("tags status %d: %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Items []recipe.TagCount `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	counts := make(map[string]int, len(out.Items))
	for _, tc := range out.Items {
		counts[tc.Name] = tc.Count
	}
	if counts["fleisch"] != 2 || counts["klassiker"] != 2 || counts["schmoren"] != 1 {
		t.Fatalf("counts = %+v", out.Items)
	}
}

func TestSearchAndTagQuery(t *testing.T) {
	h := newRecipeHandler(t)
	cookie := loginCookie(t, h)
	fx := loadFixtures(t)[0] // tags: klassiker, fleisch

	rec := doReq(h, http.MethodPost, "/api/v1/recipes", mustMarshal(t, fx), cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status %d: %s", rec.Code, rec.Body.String())
	}

	rec = doReq(h, http.MethodGet, "/api/v1/recipes?q=klop", "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("search status %d: %s", rec.Code, rec.Body.String())
	}
	var page recipe.Page
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 {
		t.Fatalf("q=klop total = %d, body = %s", page.Total, rec.Body.String())
	}

	// Uppercase and padded on purpose: the handler must normalise (trim +
	// lower) the tag query before calling List.
	rec = doReq(h, http.MethodGet, "/api/v1/recipes?tag="+url.QueryEscape(" FLEISCH "), "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("tag status %d: %s", rec.Code, rec.Body.String())
	}
	page = recipe.Page{}
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 {
		t.Fatalf("tag=FLEISCH total = %d, body = %s", page.Total, rec.Body.String())
	}
}

func TestSourceURLMustBeHTTP(t *testing.T) {
	h := newRecipeHandler(t)
	cookie := loginCookie(t, h)
	fx := loadFixtures(t)[0]

	// format:"uri" alone only asks for some scheme, so without the pattern a
	// script URL is stored and the detail page renders it as a link.
	script := "javascript:alert(1)"
	blocked := fx
	blocked.SourceURL = &script
	rec := doReq(h, http.MethodPost, "/api/v1/recipes", mustMarshal(t, blocked), cookie)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("javascript: status %d: %s", rec.Code, rec.Body.String())
	}
	var problem struct {
		Errors []struct {
			Location string `json:"location"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatal(err)
	}
	located := false
	for _, e := range problem.Errors {
		if e.Location == "body.sourceUrl" {
			located = true
		}
	}
	if !located {
		t.Fatalf("errors = %+v, body = %s", problem.Errors, rec.Body.String())
	}

	source := "https://example.test/rezept"
	allowed := fx
	allowed.SourceURL = &source
	rec = doReq(h, http.MethodPost, "/api/v1/recipes", mustMarshal(t, allowed), cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("https status %d: %s", rec.Code, rec.Body.String())
	}
	var created recipe.Recipe
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.SourceURL == nil || *created.SourceURL != source {
		t.Fatalf("sourceUrl = %v", created.SourceURL)
	}
}
