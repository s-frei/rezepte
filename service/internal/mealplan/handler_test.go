package mealplan_test

import (
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
	"github.com/s-frei/rezepte/service/internal/mealplan"
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
	srv := httpserver.New(cfg, slog.New(slog.DiscardHandler), fstest.MapFS{}, httpserver.Options{
		APIMiddleware: auth.Middleware(sessions, tokens, false),
	})
	auth.Register(srv.API(), sessions, auth.NewDevice(make([]byte, 32)), false)
	lists := shopping.NewService(conn)
	shopping.Register(srv.API(), lists)
	plans := mealplan.NewService(conn, lists)
	mealplan.Register(srv.API(), plans)
	mealplan.RegisterFeed(srv.API(), mealplan.NewFeeds(conn, "http://localhost:8060"))
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

func (a api) user(t *testing.T, name string) user.User {
	t.Helper()
	u, err := a.users.ByUsername(context.Background(), name)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

// send sends a JSON body, checks the status and decodes the answer into out
// when given.
func (a api) send(t *testing.T, c *http.Cookie, method, path string, body any, want int, out any) {
	t.Helper()
	raw := ""
	if s, ok := body.(string); ok {
		raw = s
	} else if body != nil {
		b, _ := json.Marshal(body)
		raw = string(b)
	}
	rec := a.do(method, path, raw, c)
	if rec.Code != want {
		t.Fatalf("%s %s: status %d, want %d: %s", method, path, rec.Code, want, rec.Body.String())
	}
	if out != nil {
		if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
			t.Fatalf("%s %s: %v", method, path, err)
		}
	}
}

// call is send for the operations answering a Plan.
func (a api) call(t *testing.T, c *http.Cookie, method, path string, body any, want int) mealplan.Plan {
	t.Helper()
	var p mealplan.Plan
	var out any
	if want < 300 && want != http.StatusNoContent {
		out = &p
	}
	a.send(t, c, method, path, body, want, out)
	return p
}

// dal makes anna's recipe for 4: 200 g Linsen, 1 Zwiebel.
func (a api) dal(t *testing.T) recipe.Recipe {
	t.Helper()
	g, l, z := "g", 200.0, 1.0
	r, err := recipe.NewService(a.conn, t.TempDir()).Create(context.Background(), a.user(t, "anna").ID, recipe.Input{
		Title: "Dal", Servings: 4, Tags: []string{},
		IngredientGroups: []recipe.IngredientGroup{{Ingredients: []recipe.Ingredient{
			{Quantity: &l, Unit: &g, Name: "Linsen"}, {Quantity: &z, Name: "Zwiebel"}}}},
		Steps: []recipe.Step{{Text: "Kochen."}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// sharedPlan makes a plan of anna's that ben has accepted.
func (a api) sharedPlan(t *testing.T, owner, member *http.Cookie) string {
	t.Helper()
	p := a.call(t, owner, http.MethodPost, plans, map[string]string{"name": "Familie"}, http.StatusCreated)
	a.call(t, owner, http.MethodPost, plans+"/"+p.ID+"/invitations", map[string]string{"userId": a.user(t, "ben").ID}, http.StatusOK)
	a.call(t, member, http.MethodPost, plans+"/"+p.ID+"/membership", nil, http.StatusOK)
	return p.ID
}

const (
	plans = "/api/v1/meal-plans"
	week  = "?from=" + mon + "&to=" + sun
)

func entryByTitle(t *testing.T, p mealplan.Plan, title string) mealplan.PlanEntry {
	t.Helper()
	for _, e := range p.Entries {
		if e.Title == title {
			return e
		}
	}
	t.Fatalf("no entry %q in %+v", title, p.Entries)
	return mealplan.PlanEntry{}
}

func TestPlanRoundTrip(t *testing.T) {
	a := newAPI(t)
	c := a.login(t, "anna")
	r := a.dal(t)
	p := a.call(t, c, http.MethodPost, plans, map[string]string{"name": "Familie"}, http.StatusCreated)
	if p.Name == nil || *p.Name != "Familie" || p.Personal {
		t.Fatalf("created %+v", p)
	}
	path := plans + "/" + p.ID
	p = a.call(t, c, http.MethodPatch, path+week, map[string]string{"name": "Wir"}, http.StatusOK)
	if *p.Name != "Wir" || p.From != mon || p.To != sun {
		t.Fatalf("renamed %+v", p)
	}
	p = a.call(t, c, http.MethodPost, path+"/entries"+week, map[string]any{"day": mon, "recipeId": r.ID, "servings": 2}, http.StatusOK)
	p = a.call(t, c, http.MethodPost, path+"/entries"+week, map[string]any{"title": "Reste"}, http.StatusOK)
	dal := entryByTitle(t, p, "Dal")
	if *dal.Day != mon || *dal.Servings != 2 || entryByTitle(t, p, "Reste").Day != nil {
		t.Fatalf("added %+v", p.Entries)
	}
	ep := path + "/entries/" + dal.ID
	p = a.call(t, c, http.MethodPatch, ep+week, map[string]any{"day": sun, "position": 0}, http.StatusOK)
	if d := entryByTitle(t, p, "Dal").Day; *d != sun {
		t.Fatalf("moved to %v", *d)
	}
	p = a.call(t, c, http.MethodPatch, ep+week, map[string]any{"servings": 3}, http.StatusOK)
	if s := entryByTitle(t, p, "Dal").Servings; *s != 3 || *entryByTitle(t, p, "Dal").Day != sun {
		t.Fatalf("servings %v", *s)
	}
	a.call(t, c, http.MethodPatch, ep+week, map[string]any{"servings": 100}, http.StatusUnprocessableEntity)
	p = a.call(t, c, http.MethodDelete, ep+week, nil, http.StatusOK)
	if len(p.Entries) != 1 {
		t.Fatalf("after remove %+v", p.Entries)
	}
	p = a.call(t, c, http.MethodPost, ep+"/restore"+week, nil, http.StatusOK)
	if len(p.Entries) != 2 || *entryByTitle(t, p, "Dal").Day != sun {
		t.Fatalf("after restore %+v", p.Entries)
	}
	p = a.call(t, c, http.MethodGet, path+week, nil, http.StatusOK)
	if len(p.Entries) != 2 {
		t.Fatalf("get %+v", p.Entries)
	}

	var overview struct {
		Plans       []mealplan.PlanSummary
		Invitations []mealplan.PlanInvitation
	}
	a.send(t, c, http.MethodGet, plans, nil, http.StatusOK, &overview)
	if len(overview.Plans) != 2 || !overview.Plans[0].Personal || overview.Invitations == nil {
		t.Fatalf("overview %+v", overview)
	}
	a.call(t, c, http.MethodDelete, plans+"/"+overview.Plans[0].ID, nil, http.StatusUnprocessableEntity)
	a.call(t, c, http.MethodGet, path+"?from=2026-11-31&to="+sun, nil, http.StatusUnprocessableEntity)
	a.call(t, c, http.MethodDelete, path, nil, http.StatusNoContent)
	a.call(t, c, http.MethodGet, path+week, nil, http.StatusNotFound)
}

func TestTrayMoveWithNullDay(t *testing.T) {
	a := newAPI(t)
	c := a.login(t, "anna")
	p := a.call(t, c, http.MethodPost, plans, map[string]string{"name": "Familie"}, http.StatusCreated)
	path := plans + "/" + p.ID
	a.call(t, c, http.MethodPost, path+"/entries"+week, map[string]any{"day": mon, "title": "Erste"}, http.StatusOK)
	p = a.call(t, c, http.MethodPost, path+"/entries"+week, map[string]any{"day": mon, "title": "Zweite"}, http.StatusOK)
	ep := path + "/entries/" + entryByTitle(t, p, "Zweite").ID + week
	// Without a day key the entry stays on its day and only moves up.
	p = a.call(t, c, http.MethodPatch, ep, `{"position":0}`, http.StatusOK)
	if got := titles(t, p, ptr(mon)); !equal(got, []string{"Zweite", "Erste"}) {
		t.Fatalf("monday = %v", got)
	}
	p = a.call(t, c, http.MethodPatch, ep, `{"day":null}`, http.StatusOK)
	if e := entryByTitle(t, p, "Zweite"); e.Day != nil {
		t.Fatalf("day = %v, want the tray", *e.Day)
	}
	a.call(t, c, http.MethodPatch, ep, `{"day":"montag"}`, http.StatusUnprocessableEntity)
}

// bearer sends a request authenticated by a token of anna's carrying scopes.
func (a api) bearer(t *testing.T, scopes []string, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	raw, _, err := a.tokens.Create(context.Background(), a.user(t, "anna").ID, "t", scopes, nil)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Host = "localhost:8060"
	req.Header.Set("Origin", "http://localhost:8060")
	req.Header.Set("Authorization", "Bearer "+raw)
	rec := httptest.NewRecorder()
	a.h.ServeHTTP(rec, req)
	return rec
}

func TestPlanNeedsPlanScopes(t *testing.T) {
	a := newAPI(t)
	lists := []string{auth.ScopeListsRead, auth.ScopeListsWrite}
	rec := a.bearer(t, lists, http.MethodGet, plans, "")
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "plan:read") {
		t.Fatalf("lists token reads plans: %d %s", rec.Code, rec.Body.String())
	}
	read := []string{auth.ScopePlanRead}
	if rec := a.bearer(t, read, http.MethodGet, plans, ""); rec.Code != http.StatusOK {
		t.Fatalf("read: %d %s", rec.Code, rec.Body.String())
	}
	rec = a.bearer(t, read, http.MethodPost, plans, `{"name":"Familie"}`)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "plan:write") {
		t.Fatalf("read token writes: %d %s", rec.Code, rec.Body.String())
	}
	if rec := a.bearer(t, []string{auth.ScopePlanRead, auth.ScopePlanWrite}, http.MethodPost, plans, `{"name":"Familie"}`); rec.Code != http.StatusCreated {
		t.Fatalf("write: %d %s", rec.Code, rec.Body.String())
	}
	// Putting a plan on a list writes a list: plan scopes alone do not reach it.
	rec = a.bearer(t, []string{auth.ScopePlanRead, auth.ScopePlanWrite}, http.MethodPost, plans+"/x/shopping", `{"newListName":"x","picks":[{"entryId":"x","ingredients":[{"group":0,"index":0}]}]}`)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "lists:write") {
		t.Fatalf("plan token writes a list: %d %s", rec.Code, rec.Body.String())
	}
}

func TestPatchChecksServingsBeforeMoving(t *testing.T) {
	a := newAPI(t)
	c := a.login(t, "anna")
	r := a.dal(t)
	p := a.call(t, c, http.MethodPost, plans, map[string]string{"name": "Familie"}, http.StatusCreated)
	path := plans + "/" + p.ID
	a.call(t, c, http.MethodPost, path+"/entries"+week, map[string]any{"day": mon, "recipeId": r.ID}, http.StatusOK)
	p = a.call(t, c, http.MethodPost, path+"/entries"+week, map[string]any{"day": mon, "title": "Reste"}, http.StatusOK)
	dal := path + "/entries/" + entryByTitle(t, p, "Dal").ID + week
	rest := path + "/entries/" + entryByTitle(t, p, "Reste").ID + week
	a.call(t, c, http.MethodPatch, dal, map[string]any{"day": sun, "servings": 100}, http.StatusUnprocessableEntity)
	a.call(t, c, http.MethodPatch, rest, map[string]any{"day": sun, "servings": 2}, http.StatusNotFound)
	// A bad day is refused before the servings change.
	a.call(t, c, http.MethodPatch, dal, map[string]any{"day": "montag", "servings": 3}, http.StatusUnprocessableEntity)
	p = a.call(t, c, http.MethodGet, path+week, nil, http.StatusOK)
	if d := entryByTitle(t, p, "Dal"); *d.Day != mon || *d.Servings != 4 {
		t.Fatalf("dal moved or changed: day %v servings %v", *d.Day, *d.Servings)
	}
	if d := entryByTitle(t, p, "Reste").Day; *d != mon {
		t.Fatalf("free text moved to %v", *d)
	}
}

func TestValidationWithoutFieldHasNoLocation(t *testing.T) {
	a := newAPI(t)
	c := a.login(t, "anna")
	var overview struct{ Plans []mealplan.PlanSummary }
	a.send(t, c, http.MethodGet, plans, nil, http.StatusOK, &overview)
	rec := a.do(http.MethodDelete, plans+"/"+overview.Plans[0].ID, "", c)
	if rec.Code != http.StatusUnprocessableEntity || strings.Contains(rec.Body.String(), `"location"`) {
		t.Fatalf("delete personal: %d %s", rec.Code, rec.Body.String())
	}
}

type preview struct {
	Entries []struct {
		EntryID       string
		Day           *string
		Title         string
		Servings      int
		AlreadyListed bool
		Ingredients   []struct {
			Group, Index int
			Name         string
			Quantity     *float64
			Unit         *string
			Staple       bool
		}
	}
}

func TestShoppingPreviewMarksStaplesAndListed(t *testing.T) {
	a := newAPI(t)
	c := a.login(t, "anna")
	r := a.dal(t)
	p := a.call(t, c, http.MethodPost, plans, map[string]string{"name": "Familie"}, http.StatusCreated)
	path := plans + "/" + p.ID
	p = a.call(t, c, http.MethodPost, path+"/entries"+week, map[string]any{"day": mon, "recipeId": r.ID, "servings": 2}, http.StatusOK)
	listed := p.Entries[0].ID
	a.call(t, c, http.MethodPost, path+"/entries"+week, map[string]any{"recipeId": r.ID}, http.StatusOK)
	a.call(t, c, http.MethodPost, path+"/entries"+week, map[string]any{"day": mon, "title": "Reste"}, http.StatusOK)
	a.send(t, c, http.MethodPut, path+"/staples", map[string]any{"name": "Zwiebeln", "staple": true}, http.StatusOK, nil)
	a.send(t, c, http.MethodPost, path+"/shopping"+week, map[string]any{"newListName": "Einkauf",
		"picks": []map[string]any{{"entryId": listed, "ingredients": []map[string]int{{"group": 0, "index": 0}}}}}, http.StatusOK, nil)

	var pv preview
	a.send(t, c, http.MethodGet, path+"/shopping-preview"+week+"&tray=false", nil, http.StatusOK, &pv)
	if len(pv.Entries) != 1 {
		t.Fatalf("without tray: %+v", pv)
	}
	e := pv.Entries[0]
	if e.EntryID != listed || e.Title != "Dal" || e.Servings != 2 || !e.AlreadyListed || len(e.Ingredients) != 2 || e.Day == nil || *e.Day != mon {
		t.Fatalf("entry %+v", e)
	}
	if lin, zw := e.Ingredients[0], e.Ingredients[1]; lin.Name != "Linsen" || *lin.Quantity != 100 || *lin.Unit != "g" || lin.Staple ||
		zw.Name != "Zwiebel" || zw.Index != 1 || *zw.Quantity != 0.5 || !zw.Staple {
		t.Fatalf("ingredients %+v", e.Ingredients)
	}
	a.send(t, c, http.MethodGet, path+"/shopping-preview"+week+"&tray=true", nil, http.StatusOK, &pv)
	if len(pv.Entries) != 2 || pv.Entries[0].AlreadyListed || pv.Entries[0].Servings != 4 || *pv.Entries[0].Ingredients[0].Quantity != 200 || pv.Entries[0].Day != nil {
		t.Fatalf("with tray: %+v", pv)
	}
	a.send(t, a.login(t, "ben"), http.MethodGet, path+"/shopping-preview"+week, nil, http.StatusNotFound, nil)
}

func TestStaplesOverHTTP(t *testing.T) {
	a := newAPI(t)
	c := a.login(t, "anna")
	p := a.call(t, c, http.MethodPost, plans, map[string]string{"name": "Familie"}, http.StatusCreated)
	var st struct{ Staples []mealplan.Staple }
	a.send(t, c, http.MethodPut, plans+"/"+p.ID+"/staples", map[string]any{"name": "Zwiebeln", "staple": true}, http.StatusOK, &st)
	if len(st.Staples) != 1 || st.Staples[0].Name != "Zwiebeln" {
		t.Fatalf("staples %+v", st)
	}
	a.send(t, c, http.MethodGet, plans+"/"+p.ID+"/staples", nil, http.StatusOK, &st)
	if len(st.Staples) != 1 {
		t.Fatalf("get staples %+v", st)
	}
	// Ben is not in the plan: whatever list he shares with Anna, he neither reads nor marks its staples.
	ben := a.login(t, "ben")
	a.send(t, ben, http.MethodGet, plans+"/"+p.ID+"/staples", nil, http.StatusNotFound, nil)
	a.send(t, ben, http.MethodPut, plans+"/"+p.ID+"/staples", map[string]any{"name": "Salz", "staple": true}, http.StatusNotFound, nil)
	if a.send(t, c, http.MethodGet, plans+"/"+p.ID+"/staples", nil, http.StatusOK, &st); len(st.Staples) != 1 {
		t.Fatalf("refused staple stored: %+v", st)
	}
	// Staples ride on the plan scopes alone: a list token reaches neither route.
	path, body := plans+"/"+p.ID+"/staples", `{"name":"Salz","staple":true}`
	for _, c := range []struct {
		scopes       []string
		method, body string
		want         int
	}{
		{[]string{auth.ScopeListsRead, auth.ScopeListsWrite}, http.MethodGet, "", http.StatusForbidden},
		{[]string{auth.ScopeListsRead, auth.ScopeListsWrite}, http.MethodPut, body, http.StatusForbidden},
		{[]string{auth.ScopePlanRead}, http.MethodGet, "", http.StatusOK},
		{[]string{auth.ScopePlanRead}, http.MethodPut, body, http.StatusForbidden},
		{[]string{auth.ScopePlanRead, auth.ScopePlanWrite}, http.MethodPut, body, http.StatusOK},
	} {
		if rec := a.bearer(t, c.scopes, c.method, path, c.body); rec.Code != c.want {
			t.Errorf("%s %v = %d, want %d: %s", c.method, c.scopes, rec.Code, c.want, rec.Body.String())
		}
	}
}

func TestToListOverHTTP(t *testing.T) {
	a := newAPI(t)
	owner, member := a.login(t, "anna"), a.login(t, "ben")
	id := a.sharedPlan(t, owner, member)
	r := a.dal(t)
	path := plans + "/" + id
	p := a.call(t, owner, http.MethodPost, path+"/entries"+week, map[string]any{"day": mon, "recipeId": r.ID}, http.StatusOK)
	pick := []map[string]any{{"entryId": p.Entries[0].ID, "ingredients": []map[string]int{{"group": 0, "index": 0}, {"group": 0, "index": 1}}}}
	var out struct {
		Plan   mealplan.Plan
		ListID string
	}
	a.send(t, owner, http.MethodPost, path+"/shopping"+week, map[string]any{"newListName": "Annas Einkauf", "picks": pick}, http.StatusOK, &out)
	if out.ListID == "" || len(out.Plan.Entries[0].Lists) != 1 || out.Plan.Entries[0].Lists[0].ID != out.ListID {
		t.Fatalf("to list %+v", out)
	}
	// Ben cannot reach anna's list: it answers as a list that does not exist.
	a.send(t, member, http.MethodPost, path+"/shopping"+week, map[string]any{"listId": out.ListID, "picks": pick}, http.StatusNotFound, nil)
	a.send(t, member, http.MethodPost, path+"/shopping"+week, map[string]any{"listId": out.ListID, "newListName": "x", "picks": pick}, http.StatusUnprocessableEntity, nil)
	// Ben's own plan view does not show anna's list.
	if got := a.call(t, member, http.MethodGet, path+week+"&today="+mon, nil, http.StatusOK); len(got.Lists) != 0 || len(got.Entries[0].Lists) != 0 || got.Unlisted != 1 {
		t.Fatalf("ben sees %+v / %+v, unlisted %d", got.Lists, got.Entries[0].Lists, got.Unlisted)
	}
}

// The plans week is its own route, not a plan called "week", and reads
// with plan:read only.
func TestPlansWeekOverHTTP(t *testing.T) {
	a := newAPI(t)
	owner, member := a.login(t, "anna"), a.login(t, "ben")
	id := a.sharedPlan(t, owner, member)
	a.call(t, owner, http.MethodPost, plans+"/"+id+"/entries"+week, map[string]any{"day": mon, "title": "Reste"}, http.StatusOK)
	var out mealplan.PlansWeek
	a.send(t, member, http.MethodGet, plans+"/week"+week, nil, http.StatusOK, &out)
	if out.From != mon || out.To != sun || len(out.Plans) != 2 || out.Invitations == nil {
		t.Fatalf("plans week = %+v", out)
	}
	var house mealplan.WeekPlan
	for _, p := range out.Plans {
		if p.ID == id {
			house = p
		}
	}
	if len(house.Entries) != 1 || house.Entries[0].Kind != "text" || house.Lists == nil {
		t.Fatalf("household = %+v", house)
	}
	if rec := a.bearer(t, []string{auth.ScopePlanRead}, http.MethodGet, plans+"/week", ""); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"from"`) {
		t.Fatalf("plan:read plans week, current week: %d %s", rec.Code, rec.Body.String())
	}
	if rec := a.bearer(t, []string{auth.ScopeListsRead, auth.ScopeListsWrite}, http.MethodGet, plans+"/week", ""); rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "plan:read") {
		t.Fatalf("lists token reads the plans week: %d %s", rec.Code, rec.Body.String())
	}
	a.send(t, owner, http.MethodGet, plans+"/week?from="+mon+"&to=2027-03-01", nil, http.StatusUnprocessableEntity, nil)

	// The reader's today decides which days are past, in both reads.
	r := a.dal(t)
	a.call(t, owner, http.MethodPost, plans+"/"+id+"/entries"+week, map[string]any{"day": mon, "recipeId": r.ID}, http.StatusOK)
	for today, want := range map[string]int{mon: 1, sun: 0} {
		a.send(t, owner, http.MethodGet, plans+"/week"+week+"&today="+today, nil, http.StatusOK, &out)
		for _, p := range out.Plans {
			if p.ID == id && p.Unlisted != want {
				t.Fatalf("plans week on %s: unlisted %d, want %d", today, p.Unlisted, want)
			}
		}
		if got := a.call(t, owner, http.MethodGet, plans+"/"+id+week+"&today="+today, nil, http.StatusOK); got.Unlisted != want {
			t.Fatalf("plan on %s: unlisted %d, want %d", today, got.Unlisted, want)
		}
	}
	// A change answers with the plan for the reader's today too.
	if got := a.call(t, owner, http.MethodPost, plans+"/"+id+"/entries"+week+"&today="+sun, map[string]any{"day": mon, "recipeId": r.ID}, http.StatusOK); got.Unlisted != 0 {
		t.Fatalf("add on %s: unlisted %d, want 0", sun, got.Unlisted)
	}
	a.send(t, owner, http.MethodGet, plans+"/week"+week+"&today=morgen", nil, http.StatusUnprocessableEntity, nil)
}

// Over HTTP the bulk call keeps skipping an entry already on one of the
// caller's lists; alsoListed puts it on a second one.
func TestToListAlsoListedOverHTTP(t *testing.T) {
	a := newAPI(t)
	c := a.login(t, "anna")
	r := a.dal(t)
	p := a.call(t, c, http.MethodPost, plans, map[string]string{"name": "Familie"}, http.StatusCreated)
	path := plans + "/" + p.ID
	p = a.call(t, c, http.MethodPost, path+"/entries"+week, map[string]any{"day": mon, "recipeId": r.ID}, http.StatusOK)
	pick := []map[string]any{{"entryId": p.Entries[0].ID, "ingredients": []map[string]int{{"group": 0, "index": 0}}}}
	var out mealplan.ToListResult
	var prev mealplan.PreviewList
	a.send(t, c, http.MethodGet, path+"/shopping-preview"+week+"&entry="+p.Entries[0].ID, nil, http.StatusOK, &prev)
	if len(prev.Entries) != 1 || prev.Entries[0].EntryID != p.Entries[0].ID {
		t.Fatalf("one entry's preview = %+v", prev)
	}
	a.send(t, c, http.MethodPost, path+"/shopping"+week, map[string]any{"newListName": "Eins", "picks": pick}, http.StatusOK, &out)
	a.send(t, c, http.MethodPost, path+"/shopping"+week, map[string]any{"newListName": "Zwei", "picks": pick}, http.StatusOK, &out)
	if got := out.Plan.Entries[0].Lists; len(got) != 1 {
		t.Fatalf("bulk put a listed entry on a second list: %+v", got)
	}
	a.send(t, c, http.MethodPost, path+"/shopping"+week, map[string]any{"listId": out.ListID, "alsoListed": true, "picks": pick}, http.StatusOK, &out)
	if got := out.Plan.Entries[0].Lists; len(got) != 2 {
		t.Fatalf("alsoListed: entry lists %+v, want two", got)
	}
}
