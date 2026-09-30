package share_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/s-frei/rezepte/service/internal/image"
	"github.com/s-frei/rezepte/service/internal/recipe"
)

// publicHeaders are what every public route sends, success or not.
var publicHeaders = map[string]string{
	"Referrer-Policy": "no-referrer",
	"X-Robots-Tag":    "noindex",
	"Cache-Control":   "private, no-cache",
}

func checkPublicHeaders(t *testing.T, what string, rec *httptest.ResponseRecorder) {
	t.Helper()
	for name, want := range publicHeaders {
		if got := rec.Header().Get(name); got != want {
			t.Errorf("%s: %s = %q, want %q", what, name, got, want)
		}
	}
}

func keysOf(t *testing.T, raw []byte) []string {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// TestPublicRecipeCarriesOnlyTheRecipe pins the public response's JSON keys:
// a field added to recipe.Recipe - a person, a permission, a favorite, a
// timestamp - must never reach a stranger by accident.
func TestPublicRecipeCarriesOnlyTheRecipe(t *testing.T) {
	s := newStack(t)
	s.upload(t, s.recipe.ID)
	sh := s.create(t, s.login(t, s.member), s.recipe.ID, `{"days":null}`)

	rec := s.do(http.MethodGet, "/api/v1/public/shares/"+tokenOf(sh), "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("public recipe: %d %s", rec.Code, rec.Body.String())
	}
	checkPublicHeaders(t, "public recipe", rec)
	want := []string{"attribution", "coverImageId", "cookMinutes", "description", "images", "ingredientGroups", "prepMinutes", "servings", "sourceName", "sourceUrl", "steps", "tags", "title"}
	slices.Sort(want)
	if got := keysOf(t, rec.Body.Bytes()); !slices.Equal(got, want) {
		t.Errorf("keys = %v, want %v", got, want)
	}
	var body struct {
		Title  string
		Images []json.RawMessage
		Steps  []recipe.Step
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Images) != 1 {
		t.Fatalf("images = %d, want 1", len(body.Images))
	}
	if got := keysOf(t, body.Images[0]); !slices.Equal(got, []string{"height", "id", "width"}) {
		t.Errorf("image keys = %v", got)
	}
	if body.Title != s.recipe.Title || len(body.Steps) != 1 || len(body.Steps[0].References) != 1 {
		t.Errorf("body = %s", rec.Body.String())
	}
	for _, secret := range []string{s.recipe.ID, s.member.ID, s.member.Username, s.recipe.Slug} {
		if strings.Contains(rec.Body.String(), secret) {
			t.Errorf("the public recipe carries %q: %s", secret, rec.Body.String())
		}
	}
	if link := rec.Header().Get("Link"); link != "" {
		t.Errorf("the public recipe links a schema behind a session: %q", link)
	}
}

// TestPublicRecipeCarriesAttribution: the page learns from the response
// whether the owner wants Rezepte named at its foot - on unless switched off.
func TestPublicRecipeCarriesAttribution(t *testing.T) {
	s := newStack(t)
	sh := s.create(t, s.login(t, s.member), s.recipe.ID, `{"days":null}`)
	attribution := func() bool {
		t.Helper()
		rec := s.do(http.MethodGet, "/api/v1/public/shares/"+tokenOf(sh), "", nil)
		var body struct{ Attribution bool }
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode %s: %v", rec.Body.String(), err)
		}
		return body.Attribution
	}
	if !attribution() {
		t.Error("attribution = false by default, want true")
	}
	if _, err := s.settings.SetPublicShareAttribution(context.Background(), s.owner, false); err != nil {
		t.Fatal(err)
	}
	if attribution() {
		t.Error("attribution = true after the owner switched it off")
	}
}

// TestPublicRecipeFailsAlike: a revoked, paused, limited or unknown token is
// the same 404 with the same body, so a stranger learns nothing about why.
func TestPublicRecipeFailsAlike(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	mia := s.login(t, s.member)
	get := func(token string) *httptest.ResponseRecorder {
		return s.do(http.MethodGet, "/api/v1/public/shares/"+token, "", nil)
	}
	var bodies []string
	fail := func(what, token string) {
		t.Helper()
		rec := get(token)
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: %d, want 404: %s", what, rec.Code, rec.Body.String())
		}
		checkPublicHeaders(t, what, rec)
		if link := rec.Header().Get("Link"); link != "" {
			t.Errorf("%s: the 404 links a schema behind a session: %q", what, link)
		}
		bodies = append(bodies, rec.Body.String())
	}

	fail("unknown", "AAAAAAAAAAAAAAAAAAAAAA")
	fail("empty-looking", "x")

	sh := s.create(t, mia, s.recipe.ID, `{"days":null}`)
	if rec := get(tokenOf(sh)); rec.Code != http.StatusOK {
		t.Fatalf("active: %d", rec.Code)
	}

	if _, err := s.settings.SetPublicShares(ctx, s.owner, false); err != nil {
		t.Fatal(err)
	}
	fail("paused by the owner", tokenOf(sh))
	if _, err := s.settings.SetPublicShares(ctx, s.owner, true); err != nil {
		t.Fatal(err)
	}

	if _, err := s.users.SetCanSharePublicly(ctx, s.admin, s.member.ID, false); err != nil {
		t.Fatal(err)
	}
	fail("paused by an admin", tokenOf(sh))
	if _, err := s.users.SetCanSharePublicly(ctx, s.admin, s.member.ID, true); err != nil {
		t.Fatal(err)
	}

	if _, err := s.settings.SetShareLifetimes(ctx, s.owner, nil, new(1)); err != nil {
		t.Fatal(err)
	}
	s.clock = s.clock.Add(48 * time.Hour)
	fail("limited by the maximum", tokenOf(sh))
	if _, err := s.settings.SetShareLifetimes(ctx, s.owner, nil, nil); err != nil {
		t.Fatal(err)
	}
	if rec := get(tokenOf(sh)); rec.Code != http.StatusOK {
		t.Fatalf("revived after raising the maximum: %d", rec.Code)
	}

	if rec := s.do(http.MethodDelete, "/api/v1/shares/"+sh.ID, "", mia); rec.Code != http.StatusNoContent {
		t.Fatalf("revoke: %d", rec.Code)
	}
	fail("revoked", tokenOf(sh))

	// A photo behind a dead link is the same 404 too, so no public route
	// answers differently - "$schema" included.
	photo := s.do(http.MethodGet, "/public-images/"+tokenOf(sh)+"/"+s.recipe.ID+"/thumb.jpg", "", nil)
	bodies = append(bodies, photo.Body.String())
	if got := keysOf(t, []byte(bodies[0])); !slices.Equal(got, []string{"detail", "status", "title"}) {
		t.Errorf("404 keys = %v, want detail, status and title", got)
	}

	for i, b := range bodies {
		if b != bodies[0] {
			t.Errorf("body %d = %q, want %q like the first", i, b, bodies[0])
		}
	}
}

// TestPublicRecipeIsLive: the public page shows the recipe as it is now.
func TestPublicRecipeIsLive(t *testing.T) {
	s := newStack(t)
	sh := s.create(t, s.login(t, s.member), s.recipe.ID, `{"days":null}`)
	in := s.recipe.Input
	in.Title = "Gurkensalat mit Sesam"
	in.Steps = []recipe.Step{{Text: "Sesam rösten."}, {Text: "Gurke schneiden."}}
	if _, err := s.recipes.Update(context.Background(), s.recipe.ID, s.member, in); err != nil {
		t.Fatal(err)
	}
	rec := s.do(http.MethodGet, "/api/v1/public/shares/"+tokenOf(sh), "", nil)
	var body struct {
		Title string
		Steps []recipe.Step
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Title != in.Title || len(body.Steps) != 2 || body.Steps[0].Text != "Sesam rösten." {
		t.Errorf("after an edit: %s", rec.Body.String())
	}
}

// TestPublicImages: only photos of the shared recipe, only while the share
// serves, and a photo removed from the recipe is gone from the link too.
func TestPublicImages(t *testing.T) {
	s := newStack(t)
	mia := s.login(t, s.member)
	photo := s.upload(t, s.recipe.ID)
	other := s.newRecipe(t, "Tomatensalat")
	foreign := s.upload(t, other.ID)
	sh := s.create(t, mia, s.recipe.ID, `{"days":null}`)
	token := tokenOf(sh)

	for _, variant := range []string{"thumb", "detail", "original"} {
		rec := s.do(http.MethodGet, "/public-images/"+token+"/"+photo.ID+"/"+variant+".jpg", "", nil)
		if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/jpeg" {
			t.Errorf("%s of the shared recipe: %d %q", variant, rec.Code, rec.Header().Get("Content-Type"))
		}
		checkPublicHeaders(t, variant, rec)
	}

	var bodies []string
	fail := func(what, path string) {
		t.Helper()
		rec := s.do(http.MethodGet, path, "", nil)
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: %d, want 404", what, rec.Code)
		}
		checkPublicHeaders(t, what, rec)
		bodies = append(bodies, rec.Body.String())
	}
	fail("another recipe's photo", "/public-images/"+token+"/"+foreign.ID+"/thumb.jpg")
	fail("an unknown variant", "/public-images/"+token+"/"+photo.ID+"/huge.jpg")
	fail("no .jpg", "/public-images/"+token+"/"+photo.ID+"/thumb")
	fail("an unknown token", "/public-images/AAAAAAAAAAAAAAAAAAAAAA/"+photo.ID+"/thumb.jpg")

	if err := s.images.Delete(context.Background(), s.recipe.ID, photo.ID, s.member); err != nil {
		t.Fatal(err)
	}
	fail("a deleted photo", "/public-images/"+token+"/"+photo.ID+"/thumb.jpg")

	second := s.upload(t, s.recipe.ID)
	if rec := s.do(http.MethodDelete, "/api/v1/shares/"+sh.ID, "", mia); rec.Code != http.StatusNoContent {
		t.Fatalf("revoke: %d", rec.Code)
	}
	fail("a revoked link", "/public-images/"+token+"/"+second.ID+"/thumb.jpg")

	for i, b := range bodies {
		if b != bodies[0] {
			t.Errorf("body %d = %q, want %q like the first", i, b, bodies[0])
		}
	}
}

func TestLinkPreviewOfAPublicShare(t *testing.T) {
	s := newStack(t)
	cover := s.upload(t, s.recipe.ID)
	sh := s.create(t, s.login(t, s.member), s.recipe.ID, `{"days":null}`)
	token := tokenOf(sh)

	p := s.shares.ForRequest(httptest.NewRequest(http.MethodGet, "/s/"+token, nil))
	if p == nil {
		t.Fatal("no preview for /s/<token>")
	}
	w, h := image.ThumbSize(cover.Width, cover.Height)
	if p.URL != "/s/"+token || p.Title != s.recipe.Title || p.Description != "Knackig, frisch und scharf." ||
		p.Image != "/public-images/"+token+"/"+cover.ID+"/thumb.jpg" || p.ImageWidth != w || p.ImageHeight != h ||
		p.ImageType != "image/jpeg" {
		t.Errorf("preview = %+v", p)
	}

	for _, path := range []string{"/s/" + token + "/x", "/s/", "/s/AAAAAAAAAAAAAAAAAAAAAA", "/recipes/" + s.recipe.Slug} {
		if p := s.shares.ForRequest(httptest.NewRequest(http.MethodGet, path, nil)); p != nil {
			t.Errorf("preview for %s = %+v, want nil", path, p)
		}
	}

	// Through the whole server: the shell for /s/<token> carries the
	// recipe's preview and the public headers; the shell elsewhere does not.
	rec := s.do(http.MethodGet, "/s/"+token, "", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `content="`+s.recipe.Title+`"`) {
		t.Errorf("shell for /s/<token>: %d %s", rec.Code, rec.Body.String())
	}
	checkPublicHeaders(t, "shell for /s/<token>", rec)
	checkPublicHeaders(t, "shell for an unknown /s/ link", s.do(http.MethodGet, "/s/nope", "", nil))
	if rec := s.do(http.MethodGet, "/recipes/x", "", nil); rec.Header().Get("X-Robots-Tag") != "" {
		t.Errorf("the shell outside /s/ says noindex")
	}

	if _, err := s.settings.SetPublicShares(context.Background(), s.owner, false); err != nil {
		t.Fatal(err)
	}
	if p := s.shares.ForRequest(httptest.NewRequest(http.MethodGet, "/s/"+token, nil)); p != nil {
		t.Errorf("preview of a paused link = %+v, want nil", p)
	}
}
