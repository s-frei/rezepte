package recipe_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/s-frei/rezepte/service/internal/recipe"
	"github.com/s-frei/rezepte/service/internal/user"
)

// TestTastyEndpoints walks the tasty pair through a second member: the
// author may not mark their own recipe, the other member may, and both see
// the count and the name, while only the marker sees their own flag.
func TestTastyEndpoints(t *testing.T) {
	h, conn := newRecipeHandlerWithConn(t)
	author := loginCookie(t, h)
	rec := doReq(h, http.MethodPost, "/api/v1/recipes", mustMarshal(t, loadFixtures(t)[0]), author)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status %d: %s", rec.Code, rec.Body.String())
	}
	var created recipe.Recipe
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.TastyBy == nil {
		t.Fatal("tastyBy must be an empty array, not null")
	}
	path := "/api/v1/recipes/" + created.ID + "/tasty"

	if resp := doReq(h, http.MethodPut, path, "", author); resp.Code != http.StatusForbidden {
		t.Fatalf("author put = %d, body %s", resp.Code, resp.Body)
	}

	if _, err := user.NewService(conn).Create(context.Background(), user.CreateParams{
		Username: "mara", Password: "pw", Role: user.RoleUser, DisplayName: "Mara",
	}); err != nil {
		t.Fatal(err)
	}
	mara := loginAs(t, h, "mara", "pw")
	if resp := doReq(h, http.MethodPut, path, "", mara); resp.Code != http.StatusNoContent {
		t.Fatalf("member put = %d, body %s", resp.Code, resp.Body)
	}

	get := func(cookie *http.Cookie) recipe.Recipe {
		t.Helper()
		rec := doReq(h, http.MethodGet, "/api/v1/recipes/"+created.ID, "", cookie)
		if rec.Code != http.StatusOK {
			t.Fatalf("get status %d: %s", rec.Code, rec.Body.String())
		}
		var r recipe.Recipe
		if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil {
			t.Fatal(err)
		}
		return r
	}
	if r := get(mara); r.TastyCount != 1 || !r.Tasty || len(r.TastyBy) != 1 || r.TastyBy[0].DisplayName != "Mara" {
		t.Fatalf("marker sees count=%d tasty=%v by=%+v", r.TastyCount, r.Tasty, r.TastyBy)
	}
	if r := get(author); r.TastyCount != 1 || r.Tasty {
		t.Fatalf("author sees count=%d tasty=%v", r.TastyCount, r.Tasty)
	}

	rec = doReq(h, http.MethodGet, "/api/v1/recipes?sort=tasty", "", author)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status %d: %s", rec.Code, rec.Body.String())
	}
	var page recipe.Page
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].TastyCount != 1 || page.Items[0].Tasty {
		t.Fatalf("author's card: %+v", page.Items)
	}

	if resp := doReq(h, http.MethodDelete, path, "", mara); resp.Code != http.StatusNoContent {
		t.Fatalf("member delete = %d, body %s", resp.Code, resp.Body)
	}
	if r := get(mara); r.TastyCount != 0 || r.Tasty {
		t.Fatalf("after delete: count=%d tasty=%v", r.TastyCount, r.Tasty)
	}
}

func TestTastyEndpointsStatusCodes(t *testing.T) {
	h := newRecipeHandler(t)
	cookie := loginCookie(t, h)
	if resp := doReq(h, http.MethodPut, "/api/v1/recipes/invented-id/tasty", "", cookie); resp.Code != http.StatusNotFound {
		t.Fatalf("put on invented id = %d, body %s", resp.Code, resp.Body)
	}
	if resp := doReq(h, http.MethodDelete, "/api/v1/recipes/invented-id/tasty", "", cookie); resp.Code != http.StatusNoContent {
		t.Fatalf("delete on invented id = %d, body %s", resp.Code, resp.Body)
	}
	if resp := doReq(h, http.MethodPut, "/api/v1/recipes/invented-id/tasty", "", nil); resp.Code != http.StatusUnauthorized {
		t.Fatalf("put without session = %d, body %s", resp.Code, resp.Body)
	}
	if resp := doReq(h, http.MethodDelete, "/api/v1/recipes/invented-id/tasty", "", nil); resp.Code != http.StatusUnauthorized {
		t.Fatalf("delete without session = %d, body %s", resp.Code, resp.Body)
	}
}
