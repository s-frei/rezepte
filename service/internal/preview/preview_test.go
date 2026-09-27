package preview_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	stdimage "image"
	"image/color"
	"image/png"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/s-frei/rezepte/service/internal/db/dbtest"
	"github.com/s-frei/rezepte/service/internal/image"
	"github.com/s-frei/rezepte/service/internal/preview"
	"github.com/s-frei/rezepte/service/internal/recipe"
	"github.com/s-frei/rezepte/service/internal/settings"
	"github.com/s-frei/rezepte/service/internal/user"
)

type env struct {
	conn     *sql.DB
	previews *preview.Service
	settings *settings.Service
	recipes  *recipe.Service
	images   *image.Service
	owner    user.User
	recipe   recipe.Recipe
	coverID  string
	otherID  string
}

// setup makes a recipe with two photos, a 1200×800 cover and a second one.
func setup(t *testing.T) env {
	t.Helper()
	ctx := context.Background()
	conn := dbtest.Open(t)
	u, err := user.NewService(conn).Create(ctx, user.CreateParams{Username: "olga", Password: "pw", Role: user.RoleSuperadmin})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	recipes := recipe.NewService(conn, recipe.WithImageDir(dir))
	r, err := recipes.Create(ctx, u.ID, recipe.Input{
		Title: "Asiatischer Gurkensalat", Description: "Knackig,  frisch\nund scharf.", Servings: 2,
		IngredientGroups: []recipe.IngredientGroup{{Ingredients: []recipe.Ingredient{{Name: "Gurke"}}}},
		Steps:            []recipe.Step{{Text: "Schneiden."}},
	})
	if err != nil {
		t.Fatal(err)
	}
	images := image.NewService(conn, dir)
	cover, err := images.Upload(ctx, r.ID, u, bytes.NewReader(pngBytes(t, 1200, 800)))
	if err != nil {
		t.Fatal(err)
	}
	other, err := images.Upload(ctx, r.ID, u, bytes.NewReader(pngBytes(t, 100, 100)))
	if err != nil {
		t.Fatal(err)
	}
	st := settings.NewService(conn)
	if _, err := st.SetLinkPreviews(ctx, u, true); err != nil {
		t.Fatal(err)
	}
	e := env{conn: conn, settings: st, recipes: recipes, images: images, owner: u, recipe: r, coverID: cover.ID, otherID: other.ID}
	e.previews = e.newPreviews(t)
	return e
}

func (e env) newPreviews(t *testing.T) *preview.Service {
	t.Helper()
	svc, err := preview.NewService(context.Background(), e.conn, e.settings, e.images, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := stdimage.NewNRGBA(stdimage.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.NRGBA{60, 140, 70, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func (e env) switchPreviews(t *testing.T, on bool) {
	t.Helper()
	if _, err := e.settings.SetLinkPreviews(context.Background(), e.owner, on); err != nil {
		t.Fatal(err)
	}
}

func (e env) lifetime(t *testing.T, minutes int) {
	t.Helper()
	if _, err := e.settings.SetLinkPreviewMinutes(context.Background(), e.owner, minutes); err != nil {
		t.Fatal(err)
	}
}

// near reports whether got lies within a minute of now + d.
func near(got *time.Time, d time.Duration) bool {
	if got == nil {
		return false
	}
	diff := time.Until(*got) - d
	return diff > -time.Minute && diff < time.Minute
}

func (e env) share(t *testing.T) preview.Link {
	t.Helper()
	link, err := e.previews.ShareLink(context.Background(), e.recipe.ID)
	if err != nil {
		t.Fatal(err)
	}
	return link
}

func token(t *testing.T, path string) string {
	t.Helper()
	u, err := url.Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	return u.Query().Get("share")
}

func (e env) previewFor(target string) bool {
	return e.previews.ForRequest(httptest.NewRequest(http.MethodGet, target, nil)) != nil
}

func (e env) cover(target string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	mux.Handle("GET /link-preview/{recipeId}/{imageId}", e.previews.CoverHandler())
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

func TestShareLinkPreviewsTheRecipe(t *testing.T) {
	e := setup(t)
	link := e.share(t)
	if !near(link.ExpiresAt, 15*time.Minute) || !strings.HasPrefix(link.Path, "/recipes/asiatischer-gurkensalat?share=") {
		t.Fatalf("link = %+v, want a share path expiring in 15 minutes", link)
	}

	p := e.previews.ForRequest(httptest.NewRequest(http.MethodGet, link.Path, nil))
	if p == nil {
		t.Fatal("no preview for the share link")
	}
	tok := token(t, link.Path)
	if p.URL != link.Path || p.Title != "Asiatischer Gurkensalat" || p.Description != "Knackig, frisch und scharf." {
		t.Errorf("preview = %+v", p)
	}
	wantImage := "/link-preview/" + e.recipe.ID + "/" + e.coverID + "?share=" + tok
	if p.Image != wantImage || p.ImageWidth != 960 || p.ImageHeight != 640 || p.ImageType != "image/jpeg" {
		t.Errorf("image = %q %d×%d %q, want %q 960×640 image/jpeg", p.Image, p.ImageWidth, p.ImageHeight, p.ImageType, wantImage)
	}

	for _, target := range []string{
		"/recipes/asiatischer-gurkensalat",
		"/recipes/asiatischer-gurkensalat?share=nope",
		"/recipes/asiatischer-gurkensalat/cook?share=" + tok,
		"/recipes/unknown?share=" + tok,
		"/?share=" + tok,
	} {
		if e.previewFor(target) {
			t.Errorf("%s: got a recipe preview", target)
		}
	}

	if rec := e.cover(p.Image); rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/jpeg" {
		t.Errorf("cover with token: %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	for name, target := range map[string]string{
		"no token":    "/link-preview/" + e.recipe.ID + "/" + e.coverID,
		"other photo": "/link-preview/" + e.recipe.ID + "/" + e.otherID + "?share=" + tok,
	} {
		if rec := e.cover(target); rec.Code != http.StatusNotFound {
			t.Errorf("%s: %d, want 404", name, rec.Code)
		}
	}
}

func TestLifetimeFollowsTheSetting(t *testing.T) {
	e := setup(t)
	before := e.share(t)
	e.lifetime(t, 1440)
	after := e.share(t)
	if !near(after.ExpiresAt, 24*time.Hour) {
		t.Errorf("after the change: expires %v, want in 24 hours", after.ExpiresAt)
	}
	if !near(before.ExpiresAt, 15*time.Minute) || !e.previewFor(before.Path) {
		t.Errorf("a link issued before the change lost its 15 minutes: %+v", before)
	}
}

func TestOffShowsTheWordmark(t *testing.T) {
	e := setup(t)
	link := e.share(t)
	e.switchPreviews(t, false)
	if e.previewFor(link.Path) {
		t.Error("share link still shows the recipe")
	}
	if rec := e.cover("/link-preview/" + e.recipe.ID + "/" + e.coverID + "?share=" + token(t, link.Path)); rec.Code != http.StatusNotFound {
		t.Errorf("cover: %d, want 404", rec.Code)
	}
	if again := e.share(t); again.Path != "/recipes/asiatischer-gurkensalat" || again.ExpiresAt != nil {
		t.Errorf("link = %+v, want the plain path", again)
	}
}

func TestLinksSurviveARestart(t *testing.T) {
	e := setup(t)
	link := e.share(t)
	restarted := e.newPreviews(t)
	if restarted.ForRequest(httptest.NewRequest(http.MethodGet, link.Path, nil)) == nil {
		t.Error("a restarted service refused a link issued before the restart")
	}
}

func TestRenamedRecipe(t *testing.T) {
	e := setup(t)
	link := e.share(t)
	in := recipe.Input{
		Title: "Gurkensalat mit Sesam", Servings: 2,
		IngredientGroups: []recipe.IngredientGroup{{Ingredients: []recipe.Ingredient{{Name: "Gurke"}}}},
		Steps:            []recipe.Step{{Text: "Schneiden."}},
	}
	if _, err := e.recipes.Update(context.Background(), e.recipe.ID, e.owner, in); err != nil {
		t.Fatal(err)
	}
	// Slugs are fixed at creation, so a link sent before a rename keeps
	// working and shows the new title.
	p := e.previews.ForRequest(httptest.NewRequest(http.MethodGet, link.Path, nil))
	if p == nil || p.Title != "Gurkensalat mit Sesam" {
		t.Errorf("preview after the rename = %+v, want the new title", p)
	}
}

func TestReplacedCover(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	link := e.share(t)
	tok := token(t, link.Path)
	if err := e.images.SetCover(ctx, e.recipe.ID, e.otherID, e.owner); err != nil {
		t.Fatal(err)
	}
	if rec := e.cover("/link-preview/" + e.recipe.ID + "/" + e.coverID + "?share=" + tok); rec.Code != http.StatusNotFound {
		t.Errorf("old cover: %d, want 404", rec.Code)
	}
	p := e.previews.ForRequest(httptest.NewRequest(http.MethodGet, link.Path, nil))
	if p == nil || !strings.Contains(p.Image, e.otherID) {
		t.Fatalf("preview = %+v, want the new cover", p)
	}
	for _, id := range []string{e.coverID, e.otherID} {
		if err := e.images.Delete(ctx, e.recipe.ID, id, e.owner); err != nil {
			t.Fatal(err)
		}
	}
	p = e.previews.ForRequest(httptest.NewRequest(http.MethodGet, link.Path, nil))
	if p == nil || p.Image != "" {
		t.Errorf("preview = %+v, want the recipe without a picture", p)
	}
}

func TestEmptyDescription(t *testing.T) {
	e := setup(t)
	in := recipe.Input{
		Title: "Asiatischer Gurkensalat", Servings: 2,
		IngredientGroups: []recipe.IngredientGroup{{Ingredients: []recipe.Ingredient{{Name: "Gurke"}}}},
		Steps:            []recipe.Step{{Text: "Schneiden."}},
	}
	if _, err := e.recipes.Update(context.Background(), e.recipe.ID, e.owner, in); err != nil {
		t.Fatal(err)
	}
	p := e.previews.ForRequest(httptest.NewRequest(http.MethodGet, e.share(t).Path, nil))
	if p == nil || p.Description != "" {
		t.Errorf("preview = %+v, want an empty description", p)
	}
}

func TestShareLinkOfAnUnknownRecipe(t *testing.T) {
	e := setup(t)
	if _, err := e.previews.ShareLink(context.Background(), "nope"); !errors.Is(err, preview.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestLinkJSON(t *testing.T) {
	b, err := json.Marshal(preview.Link{Path: "/recipes/x"})
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"path":"/recipes/x","expiresAt":null}` {
		t.Errorf("json = %s", b)
	}
}
