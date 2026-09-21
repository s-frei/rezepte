package image_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"hash/crc32"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/config"
	"github.com/s-frei/rezepte/service/internal/db/dbtest"
	"github.com/s-frei/rezepte/service/internal/httpserver"
	"github.com/s-frei/rezepte/service/internal/image"
	"github.com/s-frei/rezepte/service/internal/recipe"
	"github.com/s-frei/rezepte/service/internal/user"
)

// newHandler mirrors internal/recipe/handler_test.go's stack and adds the
// image operations plus the /images/ file route exactly as main.go wires them.
func newHandler(t *testing.T) http.Handler {
	t.Helper()
	conn := dbtest.Open(t)
	users := user.NewService(conn)
	if _, err := users.Create(context.Background(), "sam", "pw", user.RoleAdmin); err != nil {
		t.Fatal(err)
	}
	cfg, _ := config.LoadFrom(map[string]string{})
	sessions := auth.NewService(conn, users)
	tokens := auth.NewTokenService(conn, users)
	srv := httpserver.New(cfg, slog.New(slog.DiscardHandler), fstest.MapFS{},
		httpserver.WithAPIMiddleware(auth.Middleware(sessions, tokens, false)))
	auth.Register(srv.API(), sessions, false)
	dir := filepath.Join(t.TempDir(), "images")
	recipe.Register(srv.API(), recipe.NewService(conn, recipe.WithImageDir(dir)))
	images := image.NewService(conn, dir)
	image.Register(srv.API(), images)
	srv.Handle("GET /images/{recipeId}/{imageId}/{file}",
		auth.RequireAuth(sessions, tokens, false, auth.ScopeRecipesRead)(image.FileHandler(images)))
	return srv.Handler()
}

func doReq(h http.Handler, method, path string, body []byte, contentType string, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Host = "localhost:8060"
	req.Header.Set("Origin", "http://localhost:8060")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func loginCookie(t *testing.T, h http.Handler) *http.Cookie {
	t.Helper()
	rec := doReq(h, http.MethodPost, "/api/v1/auth/login", []byte(`{"username":"sam","password":"pw"}`), "application/json", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("login status %d: %s", rec.Code, rec.Body.String())
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.CookieName {
			return c
		}
	}
	t.Fatal("no session cookie")
	return nil
}

func createRecipe(t *testing.T, h http.Handler, cookie *http.Cookie) recipe.Recipe {
	t.Helper()
	body := []byte(`{"title":"Bildrezept","description":"","servings":2,"prepMinutes":null,"cookMinutes":null,"sourceUrl":null,"tags":[],"ingredientGroups":[{"name":null,"ingredients":[{"quantity":null,"unit":null,"name":"Salz","note":null}]}],"steps":["Salzen."]}`)
	rec := doReq(h, http.MethodPost, "/api/v1/recipes", body, "application/json", cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create recipe %d: %s", rec.Code, rec.Body.String())
	}
	var r recipe.Recipe
	if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	return r
}

// multipartBody wraps data as the single form part "file".
func multipartBody(t *testing.T, data []byte, partType string) ([]byte, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	hdr := make(map[string][]string)
	hdr["Content-Disposition"] = []string{`form-data; name="file"; filename="photo.bin"`}
	hdr["Content-Type"] = []string{partType}
	part, err := mw.CreatePart(hdr)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes(), mw.FormDataContentType()
}

// pngHeader builds a PNG that is only the signature and a valid IHDR
// claiming w×h - enough for image.DecodeConfig, never decodable in full.
// It exercises the size guards without materialising the pixels. (The
// in-package twin lives in process_test.go; the two test packages cannot
// share a helper.)
func pngHeader(w, h uint32) []byte {
	ihdr := []byte("IHDR")
	ihdr = binary.BigEndian.AppendUint32(ihdr, w)
	ihdr = binary.BigEndian.AppendUint32(ihdr, h)
	ihdr = append(ihdr, 8, 2, 0, 0, 0) // depth 8, RGB, deflate, filter 0, no interlace
	out := []byte("\x89PNG\r\n\x1a\n")
	out = binary.BigEndian.AppendUint32(out, 13)
	out = append(out, ihdr...)
	return binary.BigEndian.AppendUint32(out, crc32.ChecksumIEEE(ihdr))
}

func uploadPNG(t *testing.T, h http.Handler, cookie *http.Cookie, recipeID string, w, hh int) recipe.Image {
	t.Helper()
	body, ct := multipartBody(t, pngBytes(t, w, hh), "image/png")
	rec := doReq(h, http.MethodPost, "/api/v1/recipes/"+recipeID+"/images", body, ct, cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("upload %d: %s", rec.Code, rec.Body.String())
	}
	var img recipe.Image
	if err := json.Unmarshal(rec.Body.Bytes(), &img); err != nil {
		t.Fatal(err)
	}
	return img
}

func getRecipe(t *testing.T, h http.Handler, cookie *http.Cookie, id string) recipe.Recipe {
	t.Helper()
	rec := doReq(h, http.MethodGet, "/api/v1/recipes/"+id, nil, "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("get %d: %s", rec.Code, rec.Body.String())
	}
	var r recipe.Recipe
	if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	return r
}

func TestImageOperationsRequireSession(t *testing.T) {
	h := newHandler(t)
	body, ct := multipartBody(t, pngBytes(t, 64, 64), "image/png")
	for _, c := range []struct {
		method, path string
		body         []byte
		ct           string
	}{
		{http.MethodPost, "/api/v1/recipes/x/images", body, ct},
		{http.MethodDelete, "/api/v1/recipes/x/images/y", nil, ""},
		{http.MethodPut, "/api/v1/recipes/x/images/order", []byte(`{"imageIds":[]}`), "application/json"},
		{http.MethodPut, "/api/v1/recipes/x/cover", []byte(`{"imageId":"y"}`), "application/json"},
		{http.MethodGet, "/images/x/y/thumb.jpg", nil, ""},
	} {
		rec := doReq(h, c.method, c.path, c.body, c.ct, nil)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s: %d", c.method, c.path, rec.Code)
		}
	}
}

func TestUploadServeReorderCoverDelete(t *testing.T) {
	h := newHandler(t)
	cookie := loginCookie(t, h)
	r := createRecipe(t, h, cookie)

	first := uploadPNG(t, h, cookie, r.ID, 800, 600)
	if first.Width != 800 || first.Height != 600 || first.Position != 0 {
		t.Fatalf("first = %+v", first)
	}
	second := uploadPNG(t, h, cookie, r.ID, 64, 64)

	got := getRecipe(t, h, cookie, r.ID)
	if got.CoverImageID == nil || *got.CoverImageID != first.ID || len(got.Images) != 2 {
		t.Fatalf("after uploads: cover=%v images=%+v", got.CoverImageID, got.Images)
	}

	for _, v := range []string{"thumb", "detail", "original"} {
		rec := doReq(h, http.MethodGet, "/images/"+r.ID+"/"+first.ID+"/"+v+".jpg", nil, "", cookie)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d", v, rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "image/jpeg" {
			t.Fatalf("%s: content type %q", v, ct)
		}
		if cc := rec.Header().Get("Cache-Control"); cc != "private, max-age=31536000, immutable" {
			t.Fatalf("%s: cache control %q", v, cc)
		}
		if !bytes.HasPrefix(rec.Body.Bytes(), []byte{0xFF, 0xD8}) {
			t.Fatalf("%s: body is not a JPEG", v)
		}
	}
	// HEAD is answered by the same "GET /images/..." mux pattern and must
	// carry the headers without a body, so the frontend can probe a file.
	head := doReq(h, http.MethodHead, "/images/"+r.ID+"/"+first.ID+"/thumb.jpg", nil, "", cookie)
	if head.Code != http.StatusOK {
		t.Fatalf("HEAD: %d", head.Code)
	}
	if ct := head.Header().Get("Content-Type"); ct != "image/jpeg" {
		t.Fatalf("HEAD content type %q", ct)
	}
	if cc := head.Header().Get("Cache-Control"); cc != "private, max-age=31536000, immutable" {
		t.Fatalf("HEAD cache control %q", cc)
	}
	if head.Body.Len() != 0 {
		t.Fatalf("HEAD body is %d bytes, want 0", head.Body.Len())
	}

	for _, p := range []string{
		"/images/" + r.ID + "/" + first.ID + "/large.jpg",
		"/images/" + r.ID + "/" + first.ID + "/thumb.png",
		"/images/" + r.ID + "/00000000-0000-7000-8000-000000000000/thumb.jpg",
		"/images/" + r.ID + "/" + first.ID + "/thumb.jpg/extra",
	} {
		if rec := doReq(h, http.MethodGet, p, nil, "", cookie); rec.Code != http.StatusNotFound {
			t.Errorf("%s: %d, want 404", p, rec.Code)
		}
	}

	rec := doReq(h, http.MethodPut, "/api/v1/recipes/"+r.ID+"/images/order",
		[]byte(`{"imageIds":["`+second.ID+`","`+first.ID+`"]}`), "application/json", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("order %d: %s", rec.Code, rec.Body.String())
	}
	var ordered struct {
		Items []recipe.Image `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &ordered); err != nil {
		t.Fatal(err)
	}
	if len(ordered.Items) != 2 || ordered.Items[0].ID != second.ID || ordered.Items[0].Position != 0 {
		t.Fatalf("ordered = %+v", ordered.Items)
	}
	rec = doReq(h, http.MethodPut, "/api/v1/recipes/"+r.ID+"/images/order",
		[]byte(`{"imageIds":["`+second.ID+`"]}`), "application/json", cookie)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("bad order %d: %s", rec.Code, rec.Body.String())
	}

	rec = doReq(h, http.MethodPut, "/api/v1/recipes/"+r.ID+"/cover", []byte(`{"imageId":"`+second.ID+`"}`), "application/json", cookie)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("cover %d: %s", rec.Code, rec.Body.String())
	}
	if got = getRecipe(t, h, cookie, r.ID); *got.CoverImageID != second.ID {
		t.Fatalf("cover = %s", *got.CoverImageID)
	}
	rec = doReq(h, http.MethodPut, "/api/v1/recipes/"+r.ID+"/cover", []byte(`{"imageId":"00000000-0000-7000-8000-000000000000"}`), "application/json", cookie)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("foreign cover %d", rec.Code)
	}

	rec = doReq(h, http.MethodDelete, "/api/v1/recipes/"+r.ID+"/images/"+second.ID, nil, "", cookie)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete %d: %s", rec.Code, rec.Body.String())
	}
	got = getRecipe(t, h, cookie, r.ID)
	if len(got.Images) != 1 || got.CoverImageID == nil || *got.CoverImageID != first.ID {
		t.Fatalf("after delete: cover=%v images=%+v", got.CoverImageID, got.Images)
	}
	if rec := doReq(h, http.MethodGet, "/images/"+r.ID+"/"+second.ID+"/thumb.jpg", nil, "", cookie); rec.Code != http.StatusNotFound {
		t.Fatalf("deleted file still served: %d", rec.Code)
	}
	rec = doReq(h, http.MethodDelete, "/api/v1/recipes/"+r.ID+"/images/"+second.ID, nil, "", cookie)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("second delete %d", rec.Code)
	}
}

func TestUploadRejections(t *testing.T) {
	h := newHandler(t)
	cookie := loginCookie(t, h)
	r := createRecipe(t, h, cookie)
	png := pngBytes(t, 64, 64)

	body, ct := multipartBody(t, png, "image/png")
	if rec := doReq(h, http.MethodPost, "/api/v1/recipes/00000000-0000-7000-8000-000000000000/images", body, ct, cookie); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown recipe: %d %s", rec.Code, rec.Body.String())
	}

	// Declared part type outside the allowed list: huma's form validation, 422.
	body, ct = multipartBody(t, png, "text/plain")
	if rec := doReq(h, http.MethodPost, "/api/v1/recipes/"+r.ID+"/images", body, ct, cookie); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("text/plain part: %d %s", rec.Code, rec.Body.String())
	}

	// Declared as PNG but the bytes are a GIF: the decoder says so, 415.
	gif := []byte("GIF89a\x01\x00\x01\x00\x00\x00\x00;")
	body, ct = multipartBody(t, gif, "image/png")
	if rec := doReq(h, http.MethodPost, "/api/v1/recipes/"+r.ID+"/images", body, ct, cookie); rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("gif bytes: %d %s", rec.Code, rec.Body.String())
	}

	body, ct = multipartBody(t, pngBytes(t, 32, 32), "image/png")
	rec := doReq(h, http.MethodPost, "/api/v1/recipes/"+r.ID+"/images", body, ct, cookie)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "64-12000") {
		t.Fatalf("too small: %d %s", rec.Code, rec.Body.String())
	}

	// 12000x6000 = 72 MP: both sides are legal, the pixel count is not. The
	// header alone reaches the guard, so no 288 MiB buffer is ever needed.
	body, ct = multipartBody(t, pngHeader(12000, 6000), "image/png")
	rec = doReq(h, http.MethodPost, "/api/v1/recipes/"+r.ID+"/images", body, ct, cookie)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "60 megapixels") {
		t.Fatalf("72 MP: %d %s", rec.Code, rec.Body.String())
	}

	// Missing part.
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("other", "x")
	_ = mw.Close()
	if rec := doReq(h, http.MethodPost, "/api/v1/recipes/"+r.ID+"/images", buf.Bytes(), mw.FormDataContentType(), cookie); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("missing part: %d %s", rec.Code, rec.Body.String())
	}

	// Declared Content-Length above the cap: 413 before the body is parsed.
	big := make([]byte, 10<<20+1)
	if rec := doReq(h, http.MethodPost, "/api/v1/recipes/"+r.ID+"/images", big, ct, cookie); rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("11 MiB: %d %s", rec.Code, rec.Body.String())
	}

	for i := 0; i < 20; i++ {
		uploadPNG(t, h, cookie, r.ID, 64, 64)
	}
	body, ct = multipartBody(t, png, "image/png")
	rec = doReq(h, http.MethodPost, "/api/v1/recipes/"+r.ID+"/images", body, ct, cookie)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "20 images") {
		t.Fatalf("21st: %d %s", rec.Code, rec.Body.String())
	}
}
