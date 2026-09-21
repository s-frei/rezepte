package recipe_test

import (
	"context"
	"database/sql"
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
	h, _ := newRecipeHandlerWithConn(t)
	return h
}

// newRecipeHandlerWithConn is newRecipeHandler, but also returns the
// database connection so a test can create a second user directly (via
// user.NewService, the same way setup does in service_test.go) to exercise
// favourite isolation between users through the actual HTTP endpoints.
func newRecipeHandlerWithConn(t *testing.T) (http.Handler, *sql.DB) {
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
		httpserver.WithAPIMiddleware(auth.Middleware(sessions, tokens, false)))
	auth.Register(srv.API(), sessions, false)
	recipe.Register(srv.API(), recipe.NewService(conn))
	return srv.Handler(), conn
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
	return loginAs(t, h, "sam", "pw")
}

// loginAs logs in as username/password and returns the session cookie.
// loginCookie is the "sam" (the handler's default admin) special case of
// this, used to log in as a second user for favourite-isolation tests.
func loginAs(t *testing.T, h http.Handler, username, password string) *http.Cookie {
	t.Helper()
	rec := doReq(h, http.MethodPost, "/api/v1/auth/login", mustMarshal(t, map[string]string{"username": username, "password": password}), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("login as %s status %d: %s", username, rec.Code, rec.Body.String())
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
	fx := loadFixtures(t)
	for _, i := range []int{0, 1} { // Königsberger Klopse (klassiker, fleisch), Rinderrouladen (klassiker, fleisch, schmoren)
		rec := doReq(h, http.MethodPost, "/api/v1/recipes", mustMarshal(t, fx[i]), cookie)
		if rec.Code != http.StatusCreated {
			t.Fatalf("create %d status %d: %s", i, rec.Code, rec.Body.String())
		}
	}

	rec := doReq(h, http.MethodGet, "/api/v1/recipes?q=klop", "", cookie)
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
	// lower) each tag query parameter before calling List.
	rec = doReq(h, http.MethodGet, "/api/v1/recipes?tags="+url.QueryEscape(" FLEISCH "), "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("tags status %d: %s", rec.Code, rec.Body.String())
	}
	page = recipe.Page{}
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if page.Total != 2 {
		t.Fatalf("tags=FLEISCH total = %d, body = %s", page.Total, rec.Body.String())
	}

	// A single comma-separated query parameter must reach the handler as
	// two elements (huma disables `explode` by default for query params,
	// splitting on ",") and combine with AND, narrowing from two matches
	// to the one recipe carrying both tags.
	rec = doReq(h, http.MethodGet, "/api/v1/recipes?tags="+url.QueryEscape("fleisch,schmoren"), "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("tags=fleisch,schmoren status %d: %s", rec.Code, rec.Body.String())
	}
	page = recipe.Page{}
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || page.Items[0].Slug != "rinderrouladen" {
		t.Fatalf("tags=fleisch,schmoren total = %d, body = %s", page.Total, rec.Body.String())
	}
}

func TestFavouriteEndpoints(t *testing.T) {
	h := newRecipeHandler(t)
	cookie := loginCookie(t, h)
	fx := loadFixtures(t)[0]

	rec := doReq(h, http.MethodPost, "/api/v1/recipes", mustMarshal(t, fx), cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status %d: %s", rec.Code, rec.Body.String())
	}
	var created recipe.Recipe
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	if resp := doReq(h, http.MethodPut, "/api/v1/recipes/"+created.ID+"/favourite", "", cookie); resp.Code != http.StatusNoContent {
		t.Fatalf("put = %d, body %s", resp.Code, resp.Body)
	}
	rec = doReq(h, http.MethodGet, "/api/v1/recipes/"+created.ID, "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("get status %d: %s", rec.Code, rec.Body.String())
	}
	var got recipe.Recipe
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !got.Favourite {
		t.Fatal("recipe must report the star")
	}

	// The list must report it too - Card.Favourite reads from the same
	// per-user state as Recipe.Favourite.
	rec = doReq(h, http.MethodGet, "/api/v1/recipes", "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status %d: %s", rec.Code, rec.Body.String())
	}
	var page recipe.Page
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || !page.Items[0].Favourite {
		t.Fatalf("card must report the star: %+v", page.Items)
	}

	if resp := doReq(h, http.MethodDelete, "/api/v1/recipes/"+created.ID+"/favourite", "", cookie); resp.Code != http.StatusNoContent {
		t.Fatalf("delete = %d, body %s", resp.Code, resp.Body)
	}
	rec = doReq(h, http.MethodGet, "/api/v1/recipes/"+created.ID, "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("get status %d: %s", rec.Code, rec.Body.String())
	}
	got = recipe.Recipe{}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Favourite {
		t.Fatal("star must be gone")
	}
}

// TestSetFavouriteOnUnknownRecipeReturns404 pins the asymmetry between the
// two favourite endpoints: starring a recipe that doesn't exist (an
// invented id, or one deleted in another tab) must 404, since
// favourites.recipe_id has a foreign key to recipes(id) and there is
// nothing sensible to create a favourite row against. Unstarring the same
// id must still succeed with 204 - removing a favourite that was never
// there, or whose recipe is already gone, is not an error.
func TestSetFavouriteOnUnknownRecipeReturns404(t *testing.T) {
	h := newRecipeHandler(t)
	cookie := loginCookie(t, h)
	fx := loadFixtures(t)[0]

	rec := doReq(h, http.MethodPost, "/api/v1/recipes", mustMarshal(t, fx), cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status %d: %s", rec.Code, rec.Body.String())
	}
	var created recipe.Recipe
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if resp := doReq(h, http.MethodDelete, "/api/v1/recipes/"+created.ID, "", cookie); resp.Code != http.StatusNoContent {
		t.Fatalf("delete recipe status %d: %s", resp.Code, resp.Body)
	}

	if resp := doReq(h, http.MethodPut, "/api/v1/recipes/"+created.ID+"/favourite", "", cookie); resp.Code != http.StatusNotFound {
		t.Fatalf("put on deleted recipe = %d, body %s", resp.Code, resp.Body)
	}
	if resp := doReq(h, http.MethodPut, "/api/v1/recipes/invented-id/favourite", "", cookie); resp.Code != http.StatusNotFound {
		t.Fatalf("put on invented id = %d, body %s", resp.Code, resp.Body)
	}
	if resp := doReq(h, http.MethodDelete, "/api/v1/recipes/"+created.ID+"/favourite", "", cookie); resp.Code != http.StatusNoContent {
		t.Fatalf("delete favourite on deleted recipe = %d, body %s", resp.Code, resp.Body)
	}
}

func TestFavouriteEndpointsRequireSession(t *testing.T) {
	h := newRecipeHandler(t)
	cookie := loginCookie(t, h)
	fx := loadFixtures(t)[0]
	rec := doReq(h, http.MethodPost, "/api/v1/recipes", mustMarshal(t, fx), cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status %d: %s", rec.Code, rec.Body.String())
	}
	var created recipe.Recipe
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	if resp := doReq(h, http.MethodPut, "/api/v1/recipes/"+created.ID+"/favourite", "", nil); resp.Code != http.StatusUnauthorized {
		t.Fatalf("put without session = %d, body %s", resp.Code, resp.Body)
	}
	if resp := doReq(h, http.MethodDelete, "/api/v1/recipes/"+created.ID+"/favourite", "", nil); resp.Code != http.StatusUnauthorized {
		t.Fatalf("delete without session = %d, body %s", resp.Code, resp.Body)
	}
}

// TestFavouritesAreIsolatedBetweenUsers is the HTTP-level counterpart of
// recipe.TestFavouritesAreNotSharedBetweenUsers and
// recipe.TestDeletingFavouriteOnlyAffectsCaller: it exercises the same
// isolation property through the actual endpoints, including that neither
// endpoint accepts a user id from the request - the only way to change
// whose favourite is affected is to log in as that user.
func TestFavouritesAreIsolatedBetweenUsers(t *testing.T) {
	h, conn := newRecipeHandlerWithConn(t)
	cookieA := loginCookie(t, h)
	fx := loadFixtures(t)[0]

	rec := doReq(h, http.MethodPost, "/api/v1/recipes", mustMarshal(t, fx), cookieA)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status %d: %s", rec.Code, rec.Body.String())
	}
	var created recipe.Recipe
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	if resp := doReq(h, http.MethodPut, "/api/v1/recipes/"+created.ID+"/favourite", "", cookieA); resp.Code != http.StatusNoContent {
		t.Fatalf("user A put = %d, body %s", resp.Code, resp.Body)
	}

	if _, err := user.NewService(conn).Create(context.Background(), user.CreateParams{Username: "zweite", Password: "pw", Role: user.RoleAdmin}); err != nil {
		t.Fatal(err)
	}
	cookieB := loginAs(t, h, "zweite", "pw")

	rec = doReq(h, http.MethodGet, "/api/v1/recipes/"+created.ID, "", cookieB)
	if rec.Code != http.StatusOK {
		t.Fatalf("user B get status %d: %s", rec.Code, rec.Body.String())
	}
	var gotB recipe.Recipe
	if err := json.Unmarshal(rec.Body.Bytes(), &gotB); err != nil {
		t.Fatal(err)
	}
	if gotB.Favourite {
		t.Fatal("user B must not see user A's star")
	}

	// User B "deleting" a favourite it never set must not clear user A's.
	if resp := doReq(h, http.MethodDelete, "/api/v1/recipes/"+created.ID+"/favourite", "", cookieB); resp.Code != http.StatusNoContent {
		t.Fatalf("user B delete = %d, body %s", resp.Code, resp.Body)
	}

	rec = doReq(h, http.MethodGet, "/api/v1/recipes/"+created.ID, "", cookieA)
	if rec.Code != http.StatusOK {
		t.Fatalf("user A get status %d: %s", rec.Code, rec.Body.String())
	}
	var gotA recipe.Recipe
	if err := json.Unmarshal(rec.Body.Bytes(), &gotA); err != nil {
		t.Fatal(err)
	}
	if !gotA.Favourite {
		t.Fatal("user B's delete must not clear user A's star")
	}
}

func TestFavouritesOnlyQueryParam(t *testing.T) {
	h := newRecipeHandler(t)
	cookie := loginCookie(t, h)
	fx := loadFixtures(t)

	var created []recipe.Recipe
	for _, i := range []int{0, 1} {
		rec := doReq(h, http.MethodPost, "/api/v1/recipes", mustMarshal(t, fx[i]), cookie)
		if rec.Code != http.StatusCreated {
			t.Fatalf("create %d status %d: %s", i, rec.Code, rec.Body.String())
		}
		var r recipe.Recipe
		if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil {
			t.Fatal(err)
		}
		created = append(created, r)
	}

	if resp := doReq(h, http.MethodPut, "/api/v1/recipes/"+created[0].ID+"/favourite", "", cookie); resp.Code != http.StatusNoContent {
		t.Fatalf("put = %d, body %s", resp.Code, resp.Body)
	}

	rec := doReq(h, http.MethodGet, "/api/v1/recipes?favourites=true", "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var page recipe.Page
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || page.Items[0].ID != created[0].ID {
		t.Fatalf("favourites=true total = %d, body = %s", page.Total, rec.Body.String())
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
