package mealplan_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/s-frei/rezepte/service/internal/mealplan"
)

// newFeed creates or replaces the caller's feed and returns its path.
func (a api) newFeed(t *testing.T, c *http.Cookie) string {
	t.Helper()
	var out struct{ URL string }
	a.send(t, c, http.MethodPost, "/api/v1/calendar-feed", nil, http.StatusOK, &out)
	u, err := url.Parse(out.URL)
	if err != nil || u.Host != "localhost:8060" || !strings.HasPrefix(u.Path, "/calendar/") || !strings.HasSuffix(u.Path, ".ics") {
		t.Fatalf("feed url %q", out.URL)
	}
	return u.Path
}

// feed reads a feed, failing unless it answers want; it returns the body.
func (a api) feed(t *testing.T, path string, want int) string {
	t.Helper()
	rec := a.do(http.MethodGet, path, "", nil)
	if rec.Code != want {
		t.Fatalf("GET %s: %d, want %d: %s", path, rec.Code, want, rec.Body.String())
	}
	if want == http.StatusOK && rec.Header().Get("Content-Type") != "text/calendar; charset=utf-8" {
		t.Fatalf("Content-Type %q", rec.Header().Get("Content-Type"))
	}
	return rec.Body.String()
}

func day(offset int) string { return time.Now().AddDate(0, 0, offset).Format("2006-01-02") }

// plannedOn adds a free text on day (nil: the tray) and returns the plan.
func (a api) plannedOn(t *testing.T, c *http.Cookie, planID string, day *string, title string) mealplan.Plan {
	t.Helper()
	body := map[string]any{"title": title}
	if day != nil {
		body["day"] = *day
	}
	return a.call(t, c, http.MethodPost, plans+"/"+planID+"/entries", body, http.StatusOK)
}

func (a api) personal(t *testing.T, c *http.Cookie) string {
	t.Helper()
	var o struct{ Plans []mealplan.PlanSummary }
	a.send(t, c, http.MethodGet, plans, nil, http.StatusOK, &o)
	return o.Plans[0].ID
}

func TestFeedReplaceRevokesOld(t *testing.T) {
	a := newAPI(t)
	c := a.login(t, "anna")
	old := a.newFeed(t, c)
	if body := a.feed(t, old, http.StatusOK); !strings.HasPrefix(body, "BEGIN:VCALENDAR\r\n") {
		t.Fatalf("body %q", body)
	}
	fresh := a.newFeed(t, c)
	if fresh == old {
		t.Fatal("replacing kept the secret")
	}
	a.feed(t, old, http.StatusNotFound)
	a.feed(t, fresh, http.StatusOK)
	a.feed(t, strings.TrimSuffix(fresh, ".ics"), http.StatusNotFound)
	a.feed(t, "/calendar/nope.ics", http.StatusNotFound)
	a.send(t, c, http.MethodDelete, "/api/v1/calendar-feed", nil, http.StatusNoContent, nil)
	a.feed(t, fresh, http.StatusNotFound)
	// A token cannot hand out a feed: the secret is session-only.
	rec := a.bearer(t, []string{"plan:read", "plan:write"}, http.MethodPost, "/api/v1/calendar-feed", "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("token creates a feed: %d", rec.Code)
	}
}

func TestFeedStateTellsWithoutTheSecret(t *testing.T) {
	a := newAPI(t)
	c := a.login(t, "anna")
	state := func() map[string]any {
		var out map[string]any
		a.send(t, c, http.MethodGet, "/api/v1/calendar-feed", nil, http.StatusOK, &out)
		return out
	}
	if got := state(); got["active"] != false || got["createdAt"] != nil {
		t.Fatalf("before: %v", got)
	}
	secret := strings.TrimSuffix(strings.TrimPrefix(a.newFeed(t, c), "/calendar/"), ".ics")
	got := state()
	if got["active"] != true || got["createdAt"] == nil {
		t.Fatalf("after create: %v", got)
	}
	if strings.Contains(a.do(http.MethodGet, "/api/v1/calendar-feed", "", c).Body.String(), secret) {
		t.Fatal("state shows the secret")
	}
	a.send(t, c, http.MethodDelete, "/api/v1/calendar-feed", nil, http.StatusNoContent, nil)
	if got := state(); got["active"] != false {
		t.Fatalf("after delete: %v", got)
	}
}

func TestFeedShowsOnlyAccessiblePlans(t *testing.T) {
	a := newAPI(t)
	anna, ben := a.login(t, "anna"), a.login(t, "ben")
	shared := a.sharedPlan(t, anna, ben)
	pending := a.call(t, anna, http.MethodPost, plans, map[string]string{"name": "Nur Anna"}, http.StatusCreated)
	a.call(t, anna, http.MethodPost, plans+"/"+pending.ID+"/invitations", map[string]string{"userId": a.user(t, "ben").ID}, http.StatusOK)
	a.plannedOn(t, anna, shared, ptr(day(1)), "Gemeinsam")
	a.plannedOn(t, anna, pending.ID, ptr(day(1)), "Annas Einladung")
	a.plannedOn(t, anna, a.personal(t, anna), ptr(day(1)), "Annas Plan")
	a.plannedOn(t, ben, a.personal(t, ben), ptr(day(2)), "Bens Plan")

	body := a.feed(t, a.newFeed(t, ben), http.StatusOK)
	for title, want := range map[string]bool{"Gemeinsam": true, "Bens Plan": true, "Annas Einladung": false, "Annas Plan": false} {
		if got := strings.Contains(body, "SUMMARY:"+title+"\r\n"); got != want {
			t.Errorf("%s in ben's feed: %v, want %v", title, got, want)
		}
	}
}

func TestFeedSkipsTrayAndRemoved(t *testing.T) {
	a := newAPI(t)
	c := a.login(t, "anna")
	id := a.personal(t, c)
	r := a.dal(t)
	a.plannedOn(t, c, id, nil, "Irgendwann")
	p := a.plannedOn(t, c, id, ptr(day(0)), "Weg")
	a.call(t, c, http.MethodDelete, plans+"/"+id+"/entries/"+entryByTitle(t, p, "Weg").ID, nil, http.StatusOK)
	a.plannedOn(t, c, id, ptr(day(-400)), "Lange her")
	a.plannedOn(t, c, id, ptr(day(-300)), "Letzten Winter")
	p = a.call(t, c, http.MethodPost, plans+"/"+id+"/entries?from="+day(0)+"&to="+day(6), map[string]any{"day": day(3), "recipeId": r.ID}, http.StatusOK)

	body := a.feed(t, a.newFeed(t, c), http.StatusOK)
	if strings.Count(body, "BEGIN:VEVENT") != 2 || !strings.Contains(body, "SUMMARY:Letzten Winter\r\n") {
		t.Fatalf("feed:\n%s", body)
	}
	dal := entryByTitle(t, p, "Dal")
	for _, want := range []string{
		"UID:" + dal.ID + "@localhost:8060\r\n",
		"DTSTART;VALUE=DATE:" + strings.ReplaceAll(day(3), "-", "") + "\r\n",
		"URL:http://localhost:8060/recipes/" + *dal.RecipeSlug + "\r\n",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q in\n%s", want, body)
		}
	}
}
