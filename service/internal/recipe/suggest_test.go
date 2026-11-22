package recipe_test

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/s-frei/rezepte/service/internal/recipe"
)

func suggest(t *testing.T, svc *recipe.Service, q string, limit int) []recipe.IngredientSuggestion {
	t.Helper()
	got, err := svc.IngredientSuggestions(context.Background(), q, limit)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func suggestionNames(s []recipe.IngredientSuggestion) []string {
	var out []string
	for _, i := range s {
		out = append(out, i.Name)
	}
	return out
}

func TestIngredientSuggestions(t *testing.T) {
	// The fixtures have no ingredient that merely contains "zwiebel", so one more
	// recipe brings Röstzwiebeln.
	svc, uid := setup(t)
	fixtures := loadFixtures(t)
	extra := fixtures[0]
	extra.Title = "Röstzwiebelbrot"
	extra.IngredientGroups = []recipe.IngredientGroup{{Ingredients: []recipe.Ingredient{{Name: "Röstzwiebeln"}}}}
	extra.Steps = nil
	for _, in := range append(fixtures, extra) {
		if _, err := svc.Create(context.Background(), uid, in); err != nil {
			t.Fatal(err)
		}
	}

	z := suggest(t, svc, "zwie", 10)
	if len(z) == 0 || z[0].Name != "Zwiebel" || !slices.Contains(z[0].Also, "Röstzwiebeln") {
		t.Fatalf("zwie = %+v", z)
	}
	all := listPantry(t, svc, recipe.ListParams{Ingredients: []string{"Zwiebel"}})
	if z[0].Count != all.Total {
		t.Fatalf("zwiebel count %d, recipes using it %d", z[0].Count, all.Total)
	}

	if g := suggest(t, svc, "gew", 10); !slices.Contains(suggestionNames(g), "Gewürzgurken") {
		t.Fatalf("gew = %v", suggestionNames(g))
	}

	h := suggest(t, svc, "hack", 10)
	if len(h) == 0 || h[0].Name != "Hackfleisch" || !slices.Contains(h[0].Also, "Rinderhackfleisch") {
		t.Fatalf("hack = %+v", h)
	}

	if got := suggest(t, svc, "", 4); len(got) != 4 {
		t.Fatalf("limit 4 gave %d", len(got))
	}

	s := suggestionNames(suggest(t, svc, "sch", 50))
	if len(s) == 0 || s[0] != "Schweineschnitzel" || slices.Contains(s, "Butterschmalz") {
		t.Fatalf("sch = %v", s)
	}
}

func TestListRecipesPantryParams(t *testing.T) {
	h, _ := newRecipeHandlerWithConn(t)
	cookie := loginCookie(t, h)
	for _, fx := range loadFixtures(t) {
		if rec := doReq(h, http.MethodPost, "/api/v1/recipes", mustMarshal(t, fx), cookie); rec.Code != http.StatusCreated {
			t.Fatalf("create status %d: %s", rec.Code, rec.Body.String())
		}
	}
	rec := doReq(h, http.MethodGet, "/api/v1/recipes?ingredients=kartoffel,zwiebel&have=salz", "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var page recipe.Page
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if page.Pantry == nil || len(page.Pantry.Ticked) != 1 || len(page.Items) == 0 || page.Items[0].Pantry == nil {
		t.Fatalf("page = %+v", page)
	}

	rec = doReq(h, http.MethodGet, "/api/v1/ingredients?q=zwie&limit=3", "", cookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"Zwiebel"`) {
		t.Fatalf("suggestions %d: %s", rec.Code, rec.Body.String())
	}
}
