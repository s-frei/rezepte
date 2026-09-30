package avatar_test

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/avatar"
	"github.com/s-frei/rezepte/service/internal/config"
	"github.com/s-frei/rezepte/service/internal/db/dbtest"
	"github.com/s-frei/rezepte/service/internal/httpserver"
	imagesvc "github.com/s-frei/rezepte/service/internal/image"
	"github.com/s-frei/rezepte/service/internal/user"
	"github.com/s-frei/rezepte/service/internal/userapi"
)

type env struct {
	h        http.Handler
	dir      string
	sam, ada user.User
}

func setup(t *testing.T) env {
	t.Helper()
	conn := dbtest.Open(t)
	users := user.NewService(conn, "")
	ctx := context.Background()
	if err := users.EnsureSuperadmin(ctx, "sam", "password1"); err != nil {
		t.Fatal(err)
	}
	ada, err := users.Create(ctx, user.CreateParams{Username: "ada", Password: "password1", Role: user.RoleAdmin})
	if err != nil {
		t.Fatal(err)
	}
	list, _ := users.List(ctx)
	var sam user.User
	for _, u := range list {
		if u.Username == "sam" {
			sam = u
		}
	}
	cfg, _ := config.LoadFrom(map[string]string{})
	sessions := auth.NewService(conn, users)
	tokens := auth.NewTokenService(conn, users)
	srv := httpserver.New(cfg, slog.New(slog.DiscardHandler), fstest.MapFS{},
		httpserver.WithAPIMiddleware(auth.Middleware(sessions, tokens, false)))
	auth.Register(srv.API(), sessions, false)
	dir := filepath.Join(t.TempDir(), "avatars")
	avatars := avatar.NewService(conn, dir, imagesvc.NewService(conn, filepath.Join(t.TempDir(), "images")))
	avatar.Register(srv.API(), avatars)
	userapi.Register(srv.API(), users, sessions, avatars)
	srv.Handle("GET /avatars/{userId}/{file}",
		auth.RequireAuth(sessions, tokens, false, auth.ScopeUsersRead)(avatar.FileHandler(avatars)))
	return env{h: srv.Handler(), dir: dir, sam: sam, ada: ada}
}

func do(h http.Handler, method, path string, body []byte, contentType string, cookie *http.Cookie) *httptest.ResponseRecorder {
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

func login(t *testing.T, h http.Handler, name string) *http.Cookie {
	t.Helper()
	rec := do(h, http.MethodPost, "/api/v1/auth/login", []byte(`{"username":"`+name+`","password":"password1"}`), "application/json", nil)
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

func pngOf(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: uint8(x), G: 90, B: 40, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func form(t *testing.T, data []byte) ([]byte, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreatePart(map[string][]string{
		"Content-Disposition": {`form-data; name="file"; filename="me.png"`},
		"Content-Type":        {"image/png"},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write(data)
	_ = mw.Close()
	return buf.Bytes(), mw.FormDataContentType()
}

func upload(t *testing.T, e env, path string, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	body, ct := form(t, pngOf(t, 400, 300))
	return do(e.h, http.MethodPut, path, body, ct, cookie)
}

func avatarIDOf(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var out struct {
		AvatarID string `json:"avatarId"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out.AvatarID == "" {
		t.Fatalf("no avatarId in %s", rec.Body.String())
	}
	return out.AvatarID
}

func TestOwnAvatarRoundTrip(t *testing.T) {
	e := setup(t)
	c := login(t, e.h, "ada")
	rec := upload(t, e, "/api/v1/auth/me/avatar?crop=0.1,0,0.9", c)
	if rec.Code != http.StatusOK {
		t.Fatalf("upload: %d %s", rec.Code, rec.Body.String())
	}
	id := avatarIDOf(t, rec)

	me := do(e.h, http.MethodGet, "/api/v1/auth/me", nil, "", c)
	if !bytes.Contains(me.Body.Bytes(), []byte(`"avatarId":"`+id+`"`)) {
		t.Fatalf("me does not name the picture: %s", me.Body.String())
	}
	file := do(e.h, http.MethodGet, "/avatars/"+e.ada.ID+"/"+id+".jpg", nil, "", c)
	if file.Code != http.StatusOK || file.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatalf("serve: %d %s", file.Code, file.Header().Get("Content-Type"))
	}
	if got := file.Header().Get("Cache-Control"); got != "private, max-age=31536000, immutable" {
		t.Errorf("Cache-Control %q", got)
	}

	if rec := do(e.h, http.MethodDelete, "/api/v1/auth/me/avatar", nil, "", c); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d", rec.Code)
	}
	if rec := do(e.h, http.MethodGet, "/avatars/"+e.ada.ID+"/"+id+".jpg", nil, "", c); rec.Code != http.StatusNotFound {
		t.Fatalf("after delete: %d, want 404", rec.Code)
	}
	if rec := do(e.h, http.MethodDelete, "/api/v1/auth/me/avatar", nil, "", c); rec.Code != http.StatusNoContent {
		t.Fatalf("second delete: %d, want 204", rec.Code)
	}
}

func TestSetReplacesAndRemovesOldFile(t *testing.T) {
	e := setup(t)
	c := login(t, e.h, "ada")
	first := avatarIDOf(t, upload(t, e, "/api/v1/auth/me/avatar", c))
	second := avatarIDOf(t, upload(t, e, "/api/v1/auth/me/avatar", c))
	if first == second {
		t.Fatal("a new upload must get a new id")
	}
	if _, err := os.Stat(filepath.Join(e.dir, e.ada.ID, first+".jpg")); !os.IsNotExist(err) {
		t.Errorf("old file still there: %v", err)
	}
	if _, err := os.Stat(filepath.Join(e.dir, e.ada.ID, second+".jpg")); err != nil {
		t.Errorf("new file missing: %v", err)
	}
}

func TestUploadRejections(t *testing.T) {
	e := setup(t)
	c := login(t, e.h, "ada")
	for crop, want := range map[string]int{
		"0.1,0.2":   http.StatusUnprocessableEntity,
		"0.9,0,0.5": http.StatusUnprocessableEntity,
		"0,0,0.05":  http.StatusUnprocessableEntity,
	} {
		if rec := upload(t, e, "/api/v1/auth/me/avatar?crop="+crop, c); rec.Code != want {
			t.Errorf("crop %s: %d, want %d", crop, rec.Code, want)
		}
	}
	body, ct := form(t, []byte("not an image"))
	if rec := do(e.h, http.MethodPut, "/api/v1/auth/me/avatar", body, ct, c); rec.Code != http.StatusUnsupportedMediaType {
		t.Errorf("garbage: %d, want 415", rec.Code)
	}
	entries, _ := os.ReadDir(filepath.Join(e.dir, e.ada.ID))
	if len(entries) != 0 {
		t.Errorf("files left after rejections: %v", entries)
	}
}

func TestUserAvatarNeedsOwner(t *testing.T) {
	e := setup(t)
	ada := login(t, e.h, "ada")
	if rec := upload(t, e, "/api/v1/users/"+e.sam.ID+"/avatar", ada); rec.Code != http.StatusForbidden {
		t.Fatalf("admin on owner: %d, want 403", rec.Code)
	}
	if rec := do(e.h, http.MethodDelete, "/api/v1/users/"+e.sam.ID+"/avatar", nil, "", ada); rec.Code != http.StatusForbidden {
		t.Fatalf("admin delete: %d, want 403", rec.Code)
	}
	sam := login(t, e.h, "sam")
	if rec := upload(t, e, "/api/v1/users/"+e.ada.ID+"/avatar", sam); rec.Code != http.StatusOK {
		t.Fatalf("owner on ada: %d %s", rec.Code, rec.Body.String())
	}
	if rec := upload(t, e, "/api/v1/users/00000000-0000-7000-8000-000000000000/avatar", sam); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown user: %d, want 404", rec.Code)
	}
}

func TestFileRouteNeedsSession(t *testing.T) {
	e := setup(t)
	c := login(t, e.h, "ada")
	id := avatarIDOf(t, upload(t, e, "/api/v1/auth/me/avatar", c))
	if rec := do(e.h, http.MethodGet, "/avatars/"+e.ada.ID+"/"+id+".jpg", nil, "", nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous: %d, want 401", rec.Code)
	}
	if rec := do(e.h, http.MethodGet, "/avatars/"+e.ada.ID+"/../x.jpg", nil, "", c); rec.Code == http.StatusOK {
		t.Fatal("path escape served")
	}
}

func TestDeleteUserRemovesAvatarDir(t *testing.T) {
	e := setup(t)
	sam := login(t, e.h, "sam")
	if rec := upload(t, e, "/api/v1/users/"+e.ada.ID+"/avatar", sam); rec.Code != http.StatusOK {
		t.Fatalf("upload: %d", rec.Code)
	}
	if rec := do(e.h, http.MethodDelete, "/api/v1/users/"+e.ada.ID, nil, "", sam); rec.Code != http.StatusNoContent {
		t.Fatalf("delete user: %d %s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(filepath.Join(e.dir, e.ada.ID)); !os.IsNotExist(err) {
		t.Fatalf("avatar dir survived: %v", err)
	}
}
