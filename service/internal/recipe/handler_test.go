package recipe_test

import (
	"bytes"
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
// favorite isolation between users through the actual HTTP endpoints.
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
// this, used to log in as a second user for favorite-isolation tests.
func loginAs(t *testing.T, h http.Handler, username, password string) *http.Cookie { //nolint:unparam // every fixture user happens to share the password "pw"
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

// tokenEnv is newRecipeHandlerWithConn's counterpart for scope checks: a
// full-stack handler authenticated with a bearer token instead of a session
// cookie.
type tokenEnv struct {
	h     http.Handler
	token string
}

// newTokenEnv seeds an admin ("sam") and issues a token carrying scopes
// against the same handler stack newRecipeHandlerWithConn builds.
func newTokenEnv(t *testing.T, scopes []string) *tokenEnv {
	t.Helper()
	conn := dbtest.Open(t)
	users := user.NewService(conn)
	sam, err := users.Create(context.Background(), user.CreateParams{Username: "sam", Password: "pw", Role: user.RoleAdmin})
	if err != nil {
		t.Fatal(err)
	}
	cfg, _ := config.LoadFrom(map[string]string{})
	sessions := auth.NewService(conn, users)
	tokens := auth.NewTokenService(conn, users)
	srv := httpserver.New(cfg, slog.New(slog.DiscardHandler), fstest.MapFS{},
		httpserver.WithAPIMiddleware(auth.Middleware(sessions, tokens, false)))
	auth.Register(srv.API(), sessions, false)
	recipe.Register(srv.API(), recipe.NewService(conn))
	raw, _, err := tokens.Create(context.Background(), sam.ID, "t", scopes, nil)
	if err != nil {
		t.Fatal(err)
	}
	return &tokenEnv{h: srv.Handler(), token: raw}
}

// do sends a bearer-authenticated request against env's handler.
func (e *tokenEnv) do(t *testing.T, method, path string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Host = "localhost:8060"
	req.Header.Set("Origin", "http://localhost:8060")
	req.Header.Set("Authorization", "Bearer "+e.token)
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)
	return rec
}

// createRecipe creates a fixture recipe through env's handler and returns its
// id.
func (e *tokenEnv) createRecipe(t *testing.T) string {
	t.Helper()
	fx := loadFixtures(t)[0]
	rec := e.do(t, http.MethodPost, "/api/v1/recipes", []byte(mustMarshal(t, fx)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create recipe status %d: %s", rec.Code, rec.Body.String())
	}
	var created recipe.Recipe
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	return created.ID
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
	if created.Slug != "koenigsberger-klopse" || created.ID == "" || created.CreatedBy.ID == "" {
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

// TestDeleteRecipeNeedsDeleteScope covers that recipes:write alone is not
// enough to delete: a token can edit a collection without being able to
// empty it, so deleting requires the separate recipes:delete scope.
func TestDeleteRecipeNeedsDeleteScope(t *testing.T) {
	cases := []struct {
		name   string
		scopes []string
		want   int
	}{
		{"write only", []string{auth.ScopeRecipesRead, auth.ScopeRecipesWrite}, http.StatusForbidden},
		{"full", []string{auth.ScopeRecipesRead, auth.ScopeRecipesWrite, auth.ScopeRecipesDelete}, http.StatusNoContent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newTokenEnv(t, tc.scopes)
			id := env.createRecipe(t)
			resp := env.do(t, http.MethodDelete, "/api/v1/recipes/"+id, nil)
			if resp.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", resp.Code, tc.want, resp.Body)
			}
		})
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

// unresolvableRef points at an ingredient ("Einhorn") that does not exist
// in the "Klopse" group of the Königsberger Klopse fixture, while its word
// ("Zwiebel") does occur in the fixture's first step - so resolution fails
// specifically on the ingredientName field, not on word or groupName.
func unresolvableRef() recipe.IngredientRef {
	group := "Klopse"
	return recipe.IngredientRef{Word: "Zwiebel", GroupName: &group, IngredientName: "Einhorn"}
}

// problemErrors is the subset of huma's RFC 9457 error body this package's
// tests read: the JSON pointer of each validation failure.
type problemErrors struct {
	Errors []struct {
		Location string `json:"location"`
	} `json:"errors"`
}

func TestCreateRejectsUnresolvableReference(t *testing.T) {
	h := newRecipeHandler(t)
	cookie := loginCookie(t, h)
	fx := loadFixtures(t)[0] // Königsberger Klopse
	fx.Steps[0].References = []recipe.IngredientRef{unresolvableRef()}

	rec := doReq(h, http.MethodPost, "/api/v1/recipes", mustMarshal(t, fx), cookie)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var problem problemErrors
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatal(err)
	}
	if len(problem.Errors) == 0 || problem.Errors[0].Location != "body.steps[0].references[0].ingredientName" {
		t.Fatalf("errors = %+v, body = %s", problem.Errors, rec.Body.String())
	}
}

// TestCreateAcceptsAReferenceAsLongAsAnIngredientName pins the word limit to
// the name limit: the editor's @ picker inserts an ingredient's own name as
// the word, so any name the API takes has to be a word it takes too.
func TestCreateAcceptsAReferenceAsLongAsAnIngredientName(t *testing.T) {
	h := newRecipeHandler(t)
	cookie := loginCookie(t, h)
	fx := loadFixtures(t)[0] // Königsberger Klopse
	name := strings.Repeat("ä", 120)
	fx.IngredientGroups[0].Ingredients[0].Name = name
	fx.Steps[0].Text = name + " vorbereiten."
	fx.Steps[0].References = []recipe.IngredientRef{
		{Word: name, GroupName: fx.IngredientGroups[0].Name, IngredientName: name},
	}

	rec := doReq(h, http.MethodPost, "/api/v1/recipes", mustMarshal(t, fx), cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
}

// TestUpdateRejectsUnresolvableReference is the update-side counterpart to
// TestCreateRejectsUnresolvableReference, against a recipe that already
// exists: the reference is resolved before Update's own 404 lookup
// (Service.Update calls resolveRefs before opening its transaction), so an
// existing recipe with a bad reference must report 422, not fall through to
// a successful save.
func TestUpdateRejectsUnresolvableReference(t *testing.T) {
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

	update := created.Input
	update.Steps[0].References = []recipe.IngredientRef{unresolvableRef()}
	rec = doReq(h, http.MethodPut, "/api/v1/recipes/"+created.ID, mustMarshal(t, update), cookie)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("update status %d: %s", rec.Code, rec.Body.String())
	}
	var problem problemErrors
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatal(err)
	}
	if len(problem.Errors) == 0 || problem.Errors[0].Location != "body.steps[0].references[0].ingredientName" {
		t.Fatalf("errors = %+v, body = %s", problem.Errors, rec.Body.String())
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

	// Uppercase and padded on purpose: the handler must normalize (trim +
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

func TestFavoriteEndpoints(t *testing.T) {
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

	if resp := doReq(h, http.MethodPut, "/api/v1/recipes/"+created.ID+"/favorite", "", cookie); resp.Code != http.StatusNoContent {
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
	if !got.Favorite {
		t.Fatal("recipe must report the star")
	}

	// The list must report it too - Card.Favorite reads from the same
	// per-user state as Recipe.Favorite.
	rec = doReq(h, http.MethodGet, "/api/v1/recipes", "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status %d: %s", rec.Code, rec.Body.String())
	}
	var page recipe.Page
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || !page.Items[0].Favorite {
		t.Fatalf("card must report the star: %+v", page.Items)
	}

	if resp := doReq(h, http.MethodDelete, "/api/v1/recipes/"+created.ID+"/favorite", "", cookie); resp.Code != http.StatusNoContent {
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
	if got.Favorite {
		t.Fatal("star must be gone")
	}
}

// TestSetFavoriteOnUnknownRecipeReturns404 pins the asymmetry between the
// two favorite endpoints: starring a recipe that doesn't exist (an
// invented id, or one deleted in another tab) must 404, since
// favorites.recipe_id has a foreign key to recipes(id) and there is
// nothing sensible to create a favorite row against. Unstarring the same
// id must still succeed with 204 - removing a favorite that was never
// there, or whose recipe is already gone, is not an error.
func TestSetFavoriteOnUnknownRecipeReturns404(t *testing.T) {
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

	if resp := doReq(h, http.MethodPut, "/api/v1/recipes/"+created.ID+"/favorite", "", cookie); resp.Code != http.StatusNotFound {
		t.Fatalf("put on deleted recipe = %d, body %s", resp.Code, resp.Body)
	}
	if resp := doReq(h, http.MethodPut, "/api/v1/recipes/invented-id/favorite", "", cookie); resp.Code != http.StatusNotFound {
		t.Fatalf("put on invented id = %d, body %s", resp.Code, resp.Body)
	}
	if resp := doReq(h, http.MethodDelete, "/api/v1/recipes/"+created.ID+"/favorite", "", cookie); resp.Code != http.StatusNoContent {
		t.Fatalf("delete favorite on deleted recipe = %d, body %s", resp.Code, resp.Body)
	}
}

func TestFavoriteEndpointsRequireSession(t *testing.T) {
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

	if resp := doReq(h, http.MethodPut, "/api/v1/recipes/"+created.ID+"/favorite", "", nil); resp.Code != http.StatusUnauthorized {
		t.Fatalf("put without session = %d, body %s", resp.Code, resp.Body)
	}
	if resp := doReq(h, http.MethodDelete, "/api/v1/recipes/"+created.ID+"/favorite", "", nil); resp.Code != http.StatusUnauthorized {
		t.Fatalf("delete without session = %d, body %s", resp.Code, resp.Body)
	}
}

// TestFavoritesAreIsolatedBetweenUsers is the HTTP-level counterpart of
// recipe.TestFavoritesAreNotSharedBetweenUsers and
// recipe.TestDeletingFavoriteOnlyAffectsCaller: it exercises the same
// isolation property through the actual endpoints, including that neither
// endpoint accepts a user id from the request - the only way to change
// whose favorite is affected is to log in as that user.
func TestFavoritesAreIsolatedBetweenUsers(t *testing.T) {
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

	if resp := doReq(h, http.MethodPut, "/api/v1/recipes/"+created.ID+"/favorite", "", cookieA); resp.Code != http.StatusNoContent {
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
	if gotB.Favorite {
		t.Fatal("user B must not see user A's star")
	}

	// User B "deleting" a favorite it never set must not clear user A's.
	if resp := doReq(h, http.MethodDelete, "/api/v1/recipes/"+created.ID+"/favorite", "", cookieB); resp.Code != http.StatusNoContent {
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
	if !gotA.Favorite {
		t.Fatal("user B's delete must not clear user A's star")
	}
}

func TestFavoritesOnlyQueryParam(t *testing.T) {
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

	if resp := doReq(h, http.MethodPut, "/api/v1/recipes/"+created[0].ID+"/favorite", "", cookie); resp.Code != http.StatusNoContent {
		t.Fatalf("put = %d, body %s", resp.Code, resp.Body)
	}

	rec := doReq(h, http.MethodGet, "/api/v1/recipes?favorites=true", "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var page recipe.Page
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || page.Items[0].ID != created[0].ID {
		t.Fatalf("favorites=true total = %d, body = %s", page.Total, rec.Body.String())
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

// TestNullListsAreRejected pins that a recipe's lists are arrays, never
// null: a null passed minItems and replaced the stored steps, ingredients
// or tags with nothing.
func TestNullListsAreRejected(t *testing.T) {
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

	cases := map[string]func(m map[string]any){
		"steps":            func(m map[string]any) { m["steps"] = nil },
		"ingredientGroups": func(m map[string]any) { m["ingredientGroups"] = nil },
		"tags":             func(m map[string]any) { m["tags"] = nil },
		"ingredients": func(m map[string]any) {
			m["ingredientGroups"].([]any)[0].(map[string]any)["ingredients"] = nil
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			var body map[string]any
			if err := json.Unmarshal([]byte(mustMarshal(t, fx)), &body); err != nil {
				t.Fatal(err)
			}
			mutate(body)
			for _, req := range []struct{ method, path string }{
				{http.MethodPost, "/api/v1/recipes"},
				{http.MethodPut, "/api/v1/recipes/" + created.ID},
			} {
				rec := doReq(h, req.method, req.path, mustMarshal(t, body), cookie)
				if rec.Code != http.StatusUnprocessableEntity {
					t.Errorf("%s %s: status %d, want 422", req.method, req.path, rec.Code)
				}
			}
		})
	}
}

func TestHandlerLockedRecipeAnswers403AndFlags(t *testing.T) {
	h, conn := newRecipeHandlerWithConn(t)
	users := user.NewService(conn)
	for _, name := range []string{"anna", "ben"} {
		if _, err := users.Create(context.Background(), user.CreateParams{Username: name, Password: "pw", Role: user.RoleUser}); err != nil {
			t.Fatal(err)
		}
	}
	anna := loginAs(t, h, "anna", "pw")
	ben := loginAs(t, h, "ben", "pw")

	fx := loadFixtures(t)[0]
	fx.EditPolicy = recipe.PolicyLocked
	rec := doReq(h, http.MethodPost, "/api/v1/recipes", mustMarshal(t, fx), anna)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status %d: %s", rec.Code, rec.Body.String())
	}
	var created recipe.Recipe
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	path := "/api/v1/recipes/" + created.ID

	rec = doReq(h, http.MethodGet, path, "", ben)
	if rec.Code != http.StatusOK {
		t.Fatalf("get as ben status %d", rec.Code)
	}
	var asBen recipe.Recipe
	if err := json.Unmarshal(rec.Body.Bytes(), &asBen); err != nil {
		t.Fatal(err)
	}
	if !asBen.Locked || asBen.CanEdit || asBen.CanDelete || asBen.CanChangePolicy {
		t.Errorf("ben sees locked=%v edit=%v delete=%v policy=%v, want true/false/false/false",
			asBen.Locked, asBen.CanEdit, asBen.CanDelete, asBen.CanChangePolicy)
	}

	rec = doReq(h, http.MethodPut, path, mustMarshal(t, fx), ben)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "recipe edit locked") {
		t.Errorf("put as ben: %d %s", rec.Code, rec.Body.String())
	}
	rec = doReq(h, http.MethodDelete, path, "", ben)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "recipe delete not allowed") {
		t.Errorf("delete as ben: %d %s", rec.Code, rec.Body.String())
	}

	rec = doReq(h, http.MethodGet, path, "", anna)
	var asAnna recipe.Recipe
	if err := json.Unmarshal(rec.Body.Bytes(), &asAnna); err != nil {
		t.Fatal(err)
	}
	if !asAnna.CanEdit || !asAnna.CanDelete || !asAnna.CanChangePolicy {
		t.Errorf("anna sees edit=%v delete=%v policy=%v, want all true",
			asAnna.CanEdit, asAnna.CanDelete, asAnna.CanChangePolicy)
	}
}
