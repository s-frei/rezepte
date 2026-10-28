package push_test

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/config"
	"github.com/s-frei/rezepte/service/internal/db/dbtest"
	"github.com/s-frei/rezepte/service/internal/httpserver"
	"github.com/s-frei/rezepte/service/internal/push"
	"github.com/s-frei/rezepte/service/internal/user"
)

func newHandler(t *testing.T) http.Handler {
	t.Helper()
	conn := dbtest.Open(t)
	users := user.NewService(conn, "")
	if _, err := users.Create(context.Background(), user.CreateParams{Username: "sam", Password: "pw", Role: user.RoleUser}); err != nil {
		t.Fatal(err)
	}
	cfg, _ := config.LoadFrom(map[string]string{})
	sessions := auth.NewService(conn, users)
	tokens := auth.NewTokenService(conn, users)
	srv := httpserver.New(cfg, slog.New(slog.DiscardHandler), fstest.MapFS{},
		httpserver.WithAPIMiddleware(auth.Middleware(sessions, tokens, false)),
		httpserver.WithSecuritySchemes(auth.SecuritySchemes()))
	auth.Register(srv.API(), sessions, false)
	push.Register(srv.API(), push.NewService(conn, "", nil))
	return srv.Handler()
}

func do(h http.Handler, method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Host = "localhost:8060"
	req.Header.Set("Origin", "http://localhost:8060")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func login(t *testing.T, h http.Handler) *http.Cookie {
	t.Helper()
	rec := do(h, http.MethodPost, "/api/v1/auth/login", `{"username":"sam","password":"pw"}`, nil)
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.CookieName {
			return c
		}
	}
	t.Fatalf("login status %d: %s", rec.Code, rec.Body.String())
	return nil
}

func subscription(t *testing.T, endpoint string) string {
	t.Helper()
	k, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	b := base64.RawURLEncoding
	return fmt.Sprintf(`{"endpoint":%q,"p256dh":%q,"auth":%q}`, endpoint,
		b.EncodeToString(k.PublicKey().Bytes()), b.EncodeToString(make([]byte, 16)))
}

func TestPushRoutesNeedSession(t *testing.T) {
	h := newHandler(t)
	for _, r := range [][2]string{
		{http.MethodGet, "/api/v1/push/key"},
		{http.MethodPut, "/api/v1/push/subscription"},
		{http.MethodDelete, "/api/v1/push/subscription"},
		{http.MethodPost, "/api/v1/push/alarms"},
		{http.MethodDelete, "/api/v1/push/alarms/x"},
	} {
		if rec := do(h, r[0], r[1], "", nil); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s: status %d, want 401", r[0], r[1], rec.Code)
		}
	}
}

func TestPushRoutes(t *testing.T) {
	h := newHandler(t)
	c := login(t, h)
	const ep = "https://fcm.googleapis.com/fcm/send/abc"

	rec := do(h, http.MethodGet, "/api/v1/push/key", "", c)
	var key struct{ Key string }
	if err := json.Unmarshal(rec.Body.Bytes(), &key); rec.Code != http.StatusOK || err != nil || len(key.Key) != 87 {
		t.Fatalf("key: %d %s", rec.Code, rec.Body.String())
	}

	if rec := do(h, http.MethodPut, "/api/v1/push/subscription", subscription(t, "https://10.0.0.1/x"), c); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("foreign endpoint: %d %s", rec.Code, rec.Body.String())
	}
	if rec := do(h, http.MethodPut, "/api/v1/push/subscription", subscription(t, ep), c); rec.Code != http.StatusNoContent {
		t.Fatalf("subscribe: %d %s", rec.Code, rec.Body.String())
	}

	fireAt := time.Now().Add(time.Minute).UTC().Format(time.RFC3339)
	alarm := fmt.Sprintf(`{"endpoint":%q,"fireAt":%q,"title":"Rouladen · done","body":"Step 5","url":"/recipes/rouladen/cook?step=5"}`, ep, fireAt)
	rec = do(h, http.MethodPost, "/api/v1/push/alarms", alarm, c)
	var created struct{ ID string }
	if err := json.Unmarshal(rec.Body.Bytes(), &created); rec.Code != http.StatusOK || err != nil || created.ID == "" {
		t.Fatalf("schedule: %d %s", rec.Code, rec.Body.String())
	}
	if rec := do(h, http.MethodDelete, "/api/v1/push/alarms/"+created.ID, "", c); rec.Code != http.StatusNoContent {
		t.Fatalf("cancel: %d %s", rec.Code, rec.Body.String())
	}
	if rec := do(h, http.MethodDelete, "/api/v1/push/alarms/"+created.ID, "", c); rec.Code != http.StatusNoContent {
		t.Fatalf("cancel again: %d %s", rec.Code, rec.Body.String())
	}
	if rec := do(h, http.MethodDelete, "/api/v1/push/subscription", fmt.Sprintf(`{"endpoint":%q}`, ep), c); rec.Code != http.StatusNoContent {
		t.Fatalf("unsubscribe: %d %s", rec.Code, rec.Body.String())
	}
	if rec := do(h, http.MethodPost, "/api/v1/push/alarms", alarm, c); rec.Code != http.StatusNotFound {
		t.Fatalf("schedule without subscription: %d %s", rec.Code, rec.Body.String())
	}
}
