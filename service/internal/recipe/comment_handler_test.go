package recipe_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/recipe"
	"github.com/s-frei/rezepte/service/internal/user"
)

func TestCommentEndpoints(t *testing.T) {
	h, conn := newRecipeHandlerWithConn(t)
	sam := loginCookie(t, h)
	rec := doReq(h, http.MethodPost, "/api/v1/recipes", mustMarshal(t, loadFixtures(t)[0]), sam)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create recipe %d: %s", rec.Code, rec.Body)
	}
	var created recipe.Recipe
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	base := "/api/v1/recipes/" + created.ID + "/comments"

	for _, name := range []string{"mara", "tom"} {
		if _, err := user.NewService(conn, "").Create(context.Background(), user.CreateParams{
			Username: name, Password: "pw", Role: user.RoleUser,
		}); err != nil {
			t.Fatal(err)
		}
	}
	mara, tom := loginAs(t, h, "mara", "pw"), loginAs(t, h, "tom", "pw")

	// Validation answers 422 with the service's message.
	for _, body := range []string{"   ", strings.Repeat("ä", 2001)} {
		r := doReq(h, http.MethodPost, base, mustMarshal(t, map[string]string{"body": body}), mara)
		if r.Code != http.StatusUnprocessableEntity || !strings.Contains(r.Body.String(), "1 to 2000") {
			t.Fatalf("invalid body %q... = %d: %s", body[:3], r.Code, r.Body)
		}
	}
	add := doReq(h, http.MethodPost, base, mustMarshal(t, map[string]string{"body": "Kapern zum Schluss"}), mara)
	if add.Code != http.StatusCreated {
		t.Fatalf("add = %d: %s", add.Code, add.Body)
	}
	var c recipe.Comment
	if err := json.Unmarshal(add.Body.Bytes(), &c); err != nil {
		t.Fatal(err)
	}

	list := func(cookie *http.Cookie) []recipe.Comment {
		t.Helper()
		r := doReq(h, http.MethodGet, "/api/v1/comments?recipeId="+created.ID, "", cookie)
		if r.Code != http.StatusOK {
			t.Fatalf("list = %d: %s", r.Code, r.Body)
		}
		var out struct{ Comments []recipe.Comment }
		if err := json.Unmarshal(r.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out.Comments
	}
	if got := list(sam); len(got) != 1 || !got[0].New || got[0].Author == nil {
		t.Fatalf("sam's list: %+v", got)
	}
	if r := doReq(h, http.MethodPut, base+"/seen", `{"upTo":`+strconv.FormatInt(c.ID, 10)+`}`, sam); r.Code != http.StatusNoContent {
		t.Fatalf("seen = %d: %s", r.Code, r.Body)
	}
	if got := list(sam); got[0].New {
		t.Fatal("still new after seen")
	}

	entry := "/api/v1/comments/" + strconv.FormatInt(c.ID, 10)
	if r := doReq(h, http.MethodPatch, entry, mustMarshal(t, map[string]string{"body": "x"}), tom); r.Code != http.StatusForbidden {
		t.Fatalf("tom edits = %d", r.Code)
	}
	if r := doReq(h, http.MethodPatch, entry, mustMarshal(t, map[string]string{"body": "Kapern ganz zum Schluss"}), mara); r.Code != http.StatusOK {
		t.Fatalf("mara edits = %d: %s", r.Code, r.Body)
	}
	if r := doReq(h, http.MethodPatch, entry, `{"body":""}`, mara); r.Code != http.StatusUnprocessableEntity {
		t.Fatalf("mara empties = %d: %s", r.Code, r.Body)
	}
	if r := doReq(h, http.MethodDelete, entry, "", mara); r.Code != http.StatusNoContent {
		t.Fatalf("mara deletes = %d", r.Code)
	}

	// Somebody else's entry: a member is refused, the admin removes it.
	add = doReq(h, http.MethodPost, base, mustMarshal(t, map[string]string{"body": "Lieber Zitrone"}), tom)
	if add.Code != http.StatusCreated {
		t.Fatalf("tom adds = %d: %s", add.Code, add.Body)
	}
	if err := json.Unmarshal(add.Body.Bytes(), &c); err != nil {
		t.Fatal(err)
	}
	toms := "/api/v1/comments/" + strconv.FormatInt(c.ID, 10)
	if r := doReq(h, http.MethodDelete, toms, "", mara); r.Code != http.StatusForbidden {
		t.Fatalf("mara deletes tom's = %d", r.Code)
	}
	if r := doReq(h, http.MethodDelete, toms, "", sam); r.Code != http.StatusNoContent {
		t.Fatalf("admin deletes tom's = %d: %s", r.Code, r.Body)
	}
	if r := doReq(h, http.MethodDelete, entry, "", mara); r.Code != http.StatusNotFound {
		t.Fatalf("delete twice = %d", r.Code)
	}
	if r := doReq(h, http.MethodGet, "/api/v1/comments?recipeId=nope", "", sam); r.Code != http.StatusNotFound {
		t.Fatalf("missing recipe = %d", r.Code)
	}
	if r := doReq(h, http.MethodGet, "/api/v1/comments?recipeId="+created.ID, "", nil); r.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous = %d", r.Code)
	}
}

// TestCommentScopes covers that reading the diary takes recipes:read and
// writing takes recipes:write: a read-only token passes the list (404 for the
// unknown recipe, not 403) and is refused on every write.
func TestCommentScopes(t *testing.T) {
	env := newTokenEnv(t, []string{auth.ScopeRecipesRead})
	if r := env.do(t, http.MethodGet, "/api/v1/comments?recipeId=nope", nil); r.Code != http.StatusNotFound {
		t.Fatalf("read list = %d: %s", r.Code, r.Body)
	}
	body := []byte(`{"body":"x"}`)
	for _, c := range []struct{ method, path string }{
		{http.MethodPost, "/api/v1/recipes/nope/comments"},
		{http.MethodPatch, "/api/v1/comments/1"},
		{http.MethodDelete, "/api/v1/comments/1"},
	} {
		if r := env.do(t, c.method, c.path, body); r.Code != http.StatusForbidden {
			t.Fatalf("%s %s = %d, want 403", c.method, c.path, r.Code)
		}
	}
	if r := env.do(t, http.MethodPut, "/api/v1/recipes/nope/comments/seen", []byte(`{"upTo":1}`)); r.Code != http.StatusNotFound {
		t.Fatalf("seen with read scope = %d", r.Code)
	}
}
