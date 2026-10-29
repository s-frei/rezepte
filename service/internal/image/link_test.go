package image_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"

	"github.com/s-frei/rezepte/service/internal/image"
	"github.com/s-frei/rezepte/service/internal/user"
)

const linkBase = "https://rezepte.example"

// linkStack serves the link route for e and returns the handler and the
// links that sign for it.
func linkStack(t *testing.T, e env) (http.Handler, *image.UploadLinks) {
	t.Helper()
	mux := http.NewServeMux()
	api := humago.New(mux, huma.DefaultConfig("test", "0"))
	links := image.NewUploadLinks(linkBase, user.NewService(e.conn, ""))
	image.RegisterUploadLink(api, e.images, links)
	return mux, links
}

// postLink uploads a small PNG to link's path and query on h.
func postLink(t *testing.T, h http.Handler, link string) *httptest.ResponseRecorder {
	t.Helper()
	body, ct := multipartBody(t, pngBytes(t, 96, 72), "image/png")
	return doReq(h, http.MethodPost, strings.TrimPrefix(link, linkBase), body, ct, nil)
}

func issue(t *testing.T, links *image.UploadLinks, recipeID, userID string) string {
	t.Helper()
	link, _, err := links.Issue(recipeID, userID)
	if err != nil {
		t.Fatal(err)
	}
	return link
}

func imageCount(t *testing.T, e env, recipeID string) int {
	t.Helper()
	r, err := e.recipes.ByID(context.Background(), recipeID)
	if err != nil {
		t.Fatal(err)
	}
	return len(r.Images)
}

func TestUploadLinkAddsPhoto(t *testing.T) {
	e := setup(t)
	h, links := linkStack(t, e)
	rec := postLink(t, h, issue(t, links, e.recipe.ID, e.owner))
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	r, err := e.recipes.ByID(context.Background(), e.recipe.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Images) != 1 || r.CoverImageID == nil || *r.CoverImageID != r.Images[0].ID {
		t.Fatalf("images = %+v, cover = %v; want one image that is the cover", r.Images, r.CoverImageID)
	}
}

func TestUploadLinkWorksTwice(t *testing.T) {
	e := setup(t)
	h, links := linkStack(t, e)
	link := issue(t, links, e.recipe.ID, e.owner)
	for i := range 2 {
		if rec := postLink(t, h, link); rec.Code != http.StatusCreated {
			t.Fatalf("upload %d: status %d: %s", i, rec.Code, rec.Body.String())
		}
	}
	if n := imageCount(t, e, e.recipe.ID); n != 2 {
		t.Fatalf("images = %d, want 2", n)
	}
}

func TestUploadLinkRefusesOtherRecipe(t *testing.T) {
	e := setup(t)
	h, links := linkStack(t, e)
	other, err := e.recipes.Create(context.Background(), e.owner, e.recipe.Input)
	if err != nil {
		t.Fatal(err)
	}
	link := strings.Replace(issue(t, links, e.recipe.ID, e.owner), e.recipe.ID, other.ID, 1)
	if rec := postLink(t, h, link); rec.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403", rec.Code)
	}
	if n := imageCount(t, e, other.ID); n != 0 {
		t.Fatalf("images = %d, want 0", n)
	}
}

func TestUploadLinkRefusesTampered(t *testing.T) {
	e := setup(t)
	h, links := linkStack(t, e)
	u, err := url.Parse(issue(t, links, e.recipe.ID, e.owner))
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	token := []byte(q.Get("token"))
	token[len(token)-1] ^= 1
	q.Set("token", string(token))
	u.RawQuery = q.Encode()
	if rec := postLink(t, h, u.String()); rec.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403", rec.Code)
	}
}

func TestUploadLinkRefusesExpired(t *testing.T) {
	e := setup(t)
	h, links := linkStack(t, e)
	image.SetLinkTTL(links, -time.Second)
	if rec := postLink(t, h, issue(t, links, e.recipe.ID, e.owner)); rec.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403", rec.Code)
	}
}

func TestUploadLinkRefusesDeletedUser(t *testing.T) {
	e := setup(t)
	h, links := linkStack(t, e)
	users := user.NewService(e.conn, "")
	member, err := users.Create(context.Background(), user.CreateParams{Username: "ben", Password: "pw", Role: user.RoleUser})
	if err != nil {
		t.Fatal(err)
	}
	link := issue(t, links, e.recipe.ID, member.ID)
	if err := users.Delete(context.Background(), adminOf(e), member.ID); err != nil {
		t.Fatal(err)
	}
	if rec := postLink(t, h, link); rec.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403", rec.Code)
	}
}

func TestUploadLinkKeepsEditingRights(t *testing.T) {
	e := setup(t)
	h, links := linkStack(t, e)
	member := lockedForMember(t, e)
	if rec := postLink(t, h, issue(t, links, e.recipe.ID, member.ID)); rec.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403", rec.Code)
	}
	if n := imageCount(t, e, e.recipe.ID); n != 0 {
		t.Fatalf("images = %d, want 0", n)
	}
}

func TestIssueWithoutPublicURL(t *testing.T) {
	e := setup(t)
	_, _, err := image.NewUploadLinks("", user.NewService(e.conn, "")).Issue(e.recipe.ID, e.owner)
	if !errors.Is(err, image.ErrNoPublicURL) {
		t.Fatalf("err = %v, want ErrNoPublicURL", err)
	}
}

func TestIssuedLinkShape(t *testing.T) {
	e := setup(t)
	_, links := linkStack(t, e)
	link, expires, err := links.Issue(e.recipe.ID, e.owner)
	if err != nil {
		t.Fatal(err)
	}
	if want := linkBase + "/api/v1/recipes/" + e.recipe.ID + "/images/link?"; !strings.HasPrefix(link, want) {
		t.Fatalf("link = %q, want prefix %q", link, want)
	}
	if d := time.Until(expires); d < 14*time.Minute || d > 15*time.Minute {
		t.Fatalf("expires in %v, want about 15 minutes", d)
	}
}
