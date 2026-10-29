package mcpserver

import (
	"bytes"
	"context"
	stdimage "image"
	"image/color"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/recipe"
	"github.com/s-frei/rezepte/service/internal/recipeimport"
)

// photoSite serves a JPEG at /p.jpg and an HTML page at /page.
func photoSite(t *testing.T) string {
	t.Helper()
	img := stdimage.NewNRGBA(stdimage.Rect(0, 0, 96, 72))
	for i := range img.Pix {
		img.Pix[i] = 200
	}
	img.Set(0, 0, color.NRGBA{10, 20, 30, 255})
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/p.jpg", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(buf.Bytes())
	})
	mux.HandleFunc("/page", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><body>no photo</body></html>"))
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts.URL
}

func (e env) recipeByID(t *testing.T, id string) recipe.Recipe {
	t.Helper()
	r, err := e.svc.ByID(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestPhotoToolsByScope(t *testing.T) {
	e := newEnv(t)
	read := toolNames(t, e.connect(t, auth.ScopeRecipesRead))
	if !slices.Contains(read, "get_photo") || slices.Contains(read, "add_photo") || slices.Contains(read, "photo_upload_link") {
		t.Fatalf("read tools = %v, want get_photo without the write tools", read)
	}
	write := toolNames(t, e.connect(t, auth.ScopeRecipesRead, auth.ScopeRecipesWrite))
	for _, name := range []string{"get_photo", "add_photo", "photo_upload_link"} {
		if !slices.Contains(write, name) {
			t.Fatalf("write tools = %v, missing %s", write, name)
		}
	}
}

func TestAddPhotoFromURL(t *testing.T) {
	e := newEnv(t)
	r := e.seed(t, "Leek soup")
	res := call(t, e.connect(t, full...), "add_photo", map[string]any{"recipeId": r.ID, "url": photoSite(t) + "/p.jpg"})
	if res.IsError {
		t.Fatalf("add_photo: %s", text(res))
	}
	got := e.recipeByID(t, r.ID)
	if len(got.Images) != 1 || got.CoverImageID == nil || *got.CoverImageID != got.Images[0].ID {
		t.Fatalf("images = %+v, cover = %v; want one image that is the cover", got.Images, got.CoverImageID)
	}
}

func TestAddPhotoRejectsNonImage(t *testing.T) {
	e := newEnv(t)
	r := e.seed(t, "Leek soup")
	res := call(t, e.connect(t, full...), "add_photo", map[string]any{"recipeId": r.ID, "url": photoSite(t) + "/page"})
	if !res.IsError || !strings.Contains(text(res), "JPEG, PNG or WebP") {
		t.Fatalf("result = %v %q, want an error naming JPEG, PNG or WebP", res.IsError, text(res))
	}
	if n := len(e.recipeByID(t, r.ID).Images); n != 0 {
		t.Fatalf("images = %d, want 0", n)
	}
}

func TestAddPhotoRefusesPrivateAddress(t *testing.T) {
	e := newEnvWith(t, testPublicURL, recipeimport.PublicOnly)
	r := e.seed(t, "Leek soup")
	res := call(t, e.connect(t, full...), "add_photo", map[string]any{"recipeId": r.ID, "url": photoSite(t) + "/p.jpg"})
	if !res.IsError || !strings.Contains(text(res), "could not fetch") {
		t.Fatalf("result = %v %q, want could not fetch", res.IsError, text(res))
	}
	if n := len(e.recipeByID(t, r.ID).Images); n != 0 {
		t.Fatalf("images = %d, want 0", n)
	}
}

func TestAddPhotoKeepsEditingRights(t *testing.T) {
	e := newEnv(t)
	r := e.seedAs(t, e.owner, recipe.PolicyLocked, "Locked soup")
	res := call(t, e.connectAs(t, e.member(t, "ben"), full...), "add_photo", map[string]any{"recipeId": r.ID, "url": photoSite(t) + "/p.jpg"})
	if !res.IsError {
		t.Fatalf("add_photo on a locked recipe succeeded for a member: %s", text(res))
	}
	if n := len(e.recipeByID(t, r.ID).Images); n != 0 {
		t.Fatalf("images = %d, want 0", n)
	}
}

func TestPhotoUploadLink(t *testing.T) {
	e := newEnv(t)
	r := e.seed(t, "Leek soup")
	res := call(t, e.connect(t, full...), "photo_upload_link", map[string]any{"recipeId": r.ID})
	if res.IsError {
		t.Fatalf("photo_upload_link: %s", text(res))
	}
	out, ok := res.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("structured content = %#v", res.StructuredContent)
	}
	want := "curl -F file=@<path> '" + testPublicURL + "/api/v1/recipes/" + r.ID + "/images/link?"
	if curl, _ := out["curl"].(string); !strings.HasPrefix(curl, want) {
		t.Fatalf("curl = %q, want prefix %q", curl, want)
	}
}

func TestPhotoUploadLinkWithoutPublicURL(t *testing.T) {
	e := newEnvWith(t, "", anyAddr)
	r := e.seed(t, "Leek soup")
	res := call(t, e.connect(t, full...), "photo_upload_link", map[string]any{"recipeId": r.ID})
	if !res.IsError || !strings.Contains(text(res), "REZEPTE_PUBLIC_URL") {
		t.Fatalf("result = %v %q, want an error naming REZEPTE_PUBLIC_URL", res.IsError, text(res))
	}
}

func TestGetPhotoReturnsImage(t *testing.T) {
	e := newEnv(t)
	r := e.seed(t, "Leek soup")
	cs := e.connect(t, full...)
	if res := call(t, cs, "add_photo", map[string]any{"recipeId": r.ID, "url": photoSite(t) + "/p.jpg"}); res.IsError {
		t.Fatalf("add_photo: %s", text(res))
	}
	photoID := e.recipeByID(t, r.ID).Images[0].ID
	res := call(t, cs, "get_photo", map[string]any{"recipeId": r.ID, "photoId": photoID})
	if res.IsError || len(res.Content) != 1 {
		t.Fatalf("get_photo: error %v, %d contents", res.IsError, len(res.Content))
	}
	img, ok := res.Content[0].(*mcp.ImageContent)
	if !ok || img.MIMEType != "image/jpeg" || !bytes.HasPrefix(img.Data, []byte{0xFF, 0xD8}) {
		t.Fatalf("content = %#v, want a JPEG image", res.Content[0])
	}
}

func TestGetPhotoUnknown(t *testing.T) {
	e := newEnv(t)
	r := e.seed(t, "Leek soup")
	res := call(t, e.connect(t, full...), "get_photo", map[string]any{"recipeId": r.ID, "photoId": "00000000-0000-4000-8000-000000000000"})
	if !res.IsError || !strings.Contains(text(res), "not found") {
		t.Fatalf("result = %v %q, want not found", res.IsError, text(res))
	}
}
