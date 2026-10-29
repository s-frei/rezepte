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
	return newHandlerWith(t, nil)
}

// newHandlerWith serves the routes with client as the way to push services.
func newHandlerWith(t *testing.T, client *http.Client) http.Handler {
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
	push.Register(srv.API(), push.NewService(conn, "", client))
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
		{http.MethodPost, "/api/v1/push/test"},
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

// accepted is a push service that takes every push.
type accepted struct{ n int }

func (a *accepted) RoundTrip(*http.Request) (*http.Response, error) {
	a.n++
	return &http.Response{StatusCode: http.StatusCreated, Body: http.NoBody, Header: http.Header{}}, nil
}

func TestPushTestRoute(t *testing.T) {
	pushes := &accepted{}
	h := newHandlerWith(t, &http.Client{Transport: pushes})
	c := login(t, h)
	const ep = "https://fcm.googleapis.com/fcm/send/abc"
	body := fmt.Sprintf(`{"endpoint":%q}`, ep)

	if rec := do(h, http.MethodPost, "/api/v1/push/test", body, c); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown subscription: %d %s", rec.Code, rec.Body.String())
	}
	if rec := do(h, http.MethodPut, "/api/v1/push/subscription", subscription(t, ep), c); rec.Code != http.StatusNoContent {
		t.Fatalf("subscribe: %d %s", rec.Code, rec.Body.String())
	}
	if rec := do(h, http.MethodPost, "/api/v1/push/test", body, c); rec.Code != http.StatusNoContent {
		t.Fatalf("test: %d %s", rec.Code, rec.Body.String())
	}
	if rec := do(h, http.MethodPost, "/api/v1/push/test", body, c); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("second test: %d %s", rec.Code, rec.Body.String())
	}
	if pushes.n != 1 {
		t.Fatalf("%d pushes, want 1", pushes.n)
	}
}
