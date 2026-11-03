package shopping_test

import (
	"bytes"
	"context"
	"database/sql"
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
	"github.com/s-frei/rezepte/service/internal/recipe"
	"github.com/s-frei/rezepte/service/internal/shopping"
	"github.com/s-frei/rezepte/service/internal/user"
)

type api struct {
	h      http.Handler
	conn   *sql.DB
	tokens *auth.TokenService
	users  *user.Service
}

func newAPI(t *testing.T) api {
	t.Helper()
	conn := dbtest.Open(t)
	users := user.NewService(conn, "")
	for name, role := range map[string]user.Role{"anna": user.RoleSuperadmin, "ben": user.RoleUser} {
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
	shopping.Register(srv.API(), shopping.NewService(conn))
	return api{h: srv.Handler(), conn: conn, tokens: tokens, users: users}
}

func (a api) do(method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Host = "localhost:8060"
	req.Header.Set("Origin", "http://localhost:8060")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	a.h.ServeHTTP(rec, req)
	return rec
}

func (a api) login(t *testing.T, name string) *http.Cookie {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"username": name, "password": "pw"})
	rec := a.do(http.MethodPost, "/api/v1/auth/login", string(body), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("login %s: %d %s", name, rec.Code, rec.Body.String())
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.CookieName {
			return c
		}
	}
	t.Fatal("no session cookie")
	return nil
}

// call sends a JSON body, checks the status and decodes a List when asked.
func (a api) call(t *testing.T, c *http.Cookie, method, path string, body any, want int) shopping.List {
	t.Helper()
	raw := ""
	if body != nil {
		b, _ := json.Marshal(body)
		raw = string(b)
	}
	rec := a.do(method, path, raw, c)
	if rec.Code != want {
		t.Fatalf("%s %s: status %d, want %d: %s", method, path, rec.Code, want, rec.Body.String())
	}
	var l shopping.List
	if rec.Body.Len() > 0 && want < 300 {
		_ = json.Unmarshal(rec.Body.Bytes(), &l)
	}
	return l
}

func (a api) recipeID(t *testing.T) string {
	t.Helper()
	rs := recipe.NewService(a.conn, t.TempDir())
	anna, err := a.users.ByUsername(context.Background(), "anna")
	if err != nil {
		t.Fatal(err)
	}
	q, g := 500.0, "g"
	r, err := rs.Create(context.Background(), anna.ID, recipe.Input{
		Title: "Hack", Servings: 4, Tags: []string{},
		IngredientGroups: []recipe.IngredientGroup{{Ingredients: []recipe.Ingredient{
			{Quantity: &q, Unit: &g, Name: "Rinderhackfleisch"}, {Name: "Salz"}}}},
		Steps: []recipe.Step{{Text: "Kochen."}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return r.ID
}

const base = "/api/v1/shopping-lists"

func TestShoppingListLifecycle(t *testing.T) {
	a := newAPI(t)
	c := a.login(t, "anna")
	rid := a.recipeID(t)

	l := a.call(t, c, http.MethodPost, base, map[string]string{"name": "Woche"}, http.StatusCreated)
	p := base + "/" + l.ID
	l = a.call(t, c, http.MethodPatch, p, map[string]string{"name": "Wochenende"}, http.StatusOK)
	if l.Name != "Wochenende" {
		t.Fatalf("name %q", l.Name)
	}
	l = a.call(t, c, http.MethodPost, p+"/recipes", map[string]any{"recipeId": rid, "servings": 2,
		"ingredients": []map[string]int{{"group": 0, "index": 0}, {"group": 0, "index": 1}}}, http.StatusOK)
	if len(l.Entries) != 1 || len(l.Items) != 2 || l.OpenCount != 2 {
		t.Fatalf("after add recipe: %+v", l)
	}
	l = a.call(t, c, http.MethodPatch, p+"/recipes/"+l.Entries[0].ID, map[string]int{"servings": 4}, http.StatusOK)
	if l.Entries[0].Servings != 4 {
		t.Fatalf("servings %d", l.Entries[0].Servings)
	}
	l = a.call(t, c, http.MethodPost, p+"/items", map[string]string{"text": "2 l Milch"}, http.StatusOK)
	if len(l.Items) != 3 {
		t.Fatalf("items %d", len(l.Items))
	}
	ids := []string{l.Items[0].ID, l.Items[1].ID}
	l = a.call(t, c, http.MethodPost, p+"/items/check", map[string]any{"itemIds": ids, "checked": true}, http.StatusOK)
	if l.DoneCount != 2 || l.OpenCount != 1 {
		t.Fatalf("counts %d/%d", l.OpenCount, l.DoneCount)
	}
	l = a.call(t, c, http.MethodPost, p+"/clear-checked", nil, http.StatusOK)
	if len(l.Items) != 1 || l.DoneCount != 0 {
		t.Fatalf("after clear: %+v", l)
	}
	l = a.call(t, c, http.MethodPost, p+"/items/remove", map[string]any{"itemIds": []string{l.Items[0].ID}}, http.StatusOK)
	if len(l.Items) != 0 {
		t.Fatalf("after remove: %+v", l)
	}
	l = a.call(t, c, http.MethodDelete, p+"/recipes/"+l.Entries[0].ID, nil, http.StatusOK)
	if len(l.Entries) != 0 {
		t.Fatalf("entries %d", len(l.Entries))
	}
	a.call(t, c, http.MethodPut, "/api/v1/shopping-departments", map[string]string{"name": "Milch", "department": "drinks"}, http.StatusNoContent)
	a.call(t, c, http.MethodPut, "/api/v1/shopping-departments", map[string]string{"name": "Milch", "department": "nope"}, http.StatusUnprocessableEntity)

	rec := a.do(http.MethodGet, base, "", c)
	var sums struct{ Items []shopping.Summary }
	if err := json.Unmarshal(rec.Body.Bytes(), &sums); err != nil || rec.Code != 200 || len(sums.Items) != 1 {
		t.Fatalf("overview %d %s", rec.Code, rec.Body.String())
	}
	a.call(t, c, http.MethodDelete, p, nil, http.StatusNoContent)
	a.call(t, c, http.MethodGet, p, nil, http.StatusNotFound)
}

func TestForeignShoppingListAnswers404(t *testing.T) {
	a := newAPI(t)
	ca, cb := a.login(t, "anna"), a.login(t, "ben")
	rid := a.recipeID(t)
	l := a.call(t, ca, http.MethodPost, base, map[string]string{"name": "Woche"}, http.StatusCreated)
	l = a.call(t, ca, http.MethodPost, base+"/"+l.ID+"/items", map[string]string{"text": "Milch"}, http.StatusOK)
	p := base + "/" + l.ID
	item := map[string]any{"itemIds": []string{l.Items[0].ID}}
	calls := []struct {
		method, path string
		body         any
	}{
		{http.MethodGet, p, nil},
		{http.MethodPatch, p, map[string]string{"name": "Neu"}},
		{http.MethodDelete, p, nil},
		{http.MethodPost, p + "/recipes", map[string]any{"recipeId": rid, "servings": 2, "ingredients": []map[string]int{{"group": 0, "index": 0}}}},
		{http.MethodPatch, p + "/recipes/x", map[string]int{"servings": 2}},
		{http.MethodDelete, p + "/recipes/x", nil},
		{http.MethodPost, p + "/items", map[string]string{"text": "Brot"}},
		{http.MethodPost, p + "/items/check", map[string]any{"itemIds": item["itemIds"], "checked": true}},
		{http.MethodPost, p + "/items/remove", item},
		{http.MethodPost, p + "/clear-checked", nil},
	}
	for _, c := range calls {
		a.call(t, cb, c.method, c.path, c.body, http.StatusNotFound)
	}
	after := a.call(t, ca, http.MethodGet, p, nil, http.StatusOK)
	if after.Name != "Woche" || len(after.Items) != 1 || after.Items[0].Checked || !after.UpdatedAt.Equal(l.UpdatedAt) {
		t.Fatalf("list changed: %+v", after)
	}
	rec := a.do(http.MethodGet, base, "", cb)
	if !strings.Contains(rec.Body.String(), `"items":[]`) {
		t.Fatalf("B sees lists: %s", rec.Body.String())
	}
}

func TestUnknownRecipeAndEmptyIngredients(t *testing.T) {
	a := newAPI(t)
	c := a.login(t, "anna")
	rid := a.recipeID(t)
	l := a.call(t, c, http.MethodPost, base, map[string]string{"name": "Woche"}, http.StatusCreated)
	p := base + "/" + l.ID + "/recipes"
	a.call(t, c, http.MethodPost, p, map[string]any{"recipeId": "nope", "servings": 2, "ingredients": []map[string]int{{"group": 0, "index": 0}}}, http.StatusNotFound)
	a.call(t, c, http.MethodPost, p, map[string]any{"recipeId": rid, "servings": 2, "ingredients": []map[string]int{}}, http.StatusUnprocessableEntity)
	a.call(t, c, http.MethodPost, p, map[string]any{"recipeId": rid, "servings": 2, "ingredients": []map[string]int{{"group": 0, "index": 9}}}, http.StatusUnprocessableEntity)
}

func TestBlankNameAndTextAre422(t *testing.T) {
	a := newAPI(t)
	c := a.login(t, "anna")
	for _, name := range []string{"", "   "} {
		a.call(t, c, http.MethodPost, base, map[string]string{"name": name}, http.StatusUnprocessableEntity)
	}
	l := a.call(t, c, http.MethodPost, base, map[string]string{"name": "Woche"}, http.StatusCreated)
	p := base + "/" + l.ID
	for _, name := range []string{"", "   "} {
		a.call(t, c, http.MethodPatch, p, map[string]string{"name": name}, http.StatusUnprocessableEntity)
	}
	for _, text := range []string{"", "   "} {
		a.call(t, c, http.MethodPost, p+"/items", map[string]string{"text": text}, http.StatusUnprocessableEntity)
	}
	l = a.call(t, c, http.MethodGet, p, nil, http.StatusOK)
	if l.Name != "Woche" || len(l.Items) != 0 {
		t.Fatalf("stored something: %+v", l)
	}
	rec := a.do(http.MethodPost, base, `{"name":"  "}`, c)
	if !strings.Contains(rec.Body.String(), "body.name") {
		t.Fatalf("no body.name location: %s", rec.Body.String())
	}
}

func TestShoppingRequiresSession(t *testing.T) {
	a := newAPI(t)
	if rec := a.do(http.MethodGet, base, "", nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("no cookie: %d", rec.Code)
	}
	anna, err := a.users.ByUsername(context.Background(), "anna")
	if err != nil {
		t.Fatal(err)
	}
	raw, _, err := a.tokens.Create(context.Background(), anna.ID, "t", []string{auth.ScopeRecipesRead}, nil)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, base, bytes.NewReader(nil))
	req.Host = "localhost:8060"
	req.Header.Set("Origin", "http://localhost:8060")
	req.Header.Set("Authorization", "Bearer "+raw)
	rec := httptest.NewRecorder()
	a.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "API tokens cannot use this operation") {
		t.Fatalf("token: %d %s", rec.Code, rec.Body.String())
	}
}
