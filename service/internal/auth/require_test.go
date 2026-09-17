package auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/db/dbtest"
	"github.com/s-frei/rezepte/service/internal/user"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestRequireSessionRejectsMissingCookie(t *testing.T) {
	conn := dbtest.Open(t)
	sessions := auth.NewService(conn, user.NewService(conn))
	h := auth.RequireSession(sessions, false)(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/images/x", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/problem+json") {
		t.Fatalf("content type %q", ct)
	}
	if !strings.Contains(rec.Body.String(), `"status":401`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestRequireSessionAcceptsValidCookieAndStoresUser(t *testing.T) {
	conn := dbtest.Open(t)
	users := user.NewService(conn)
	if _, err := users.Create(context.Background(), "sam", "pw", user.RoleAdmin); err != nil {
		t.Fatal(err)
	}
	sessions := auth.NewService(conn, users)
	sess, err := sessions.Login(context.Background(), "sam", "pw")
	if err != nil {
		t.Fatal(err)
	}

	var gotUser user.User
	var gotOK bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser, gotOK = auth.UserFrom(r.Context())
		w.WriteHeader(http.StatusOK)
	})
	h := auth.RequireSession(sessions, false)(next)

	req := httptest.NewRequest(http.MethodGet, "/images/x", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: sess.Token})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if !gotOK || gotUser.Username != "sam" {
		t.Fatalf("UserFrom = %+v, %v", gotUser, gotOK)
	}
}

func TestRequireSessionReissuesCookieOnRenewal(t *testing.T) {
	conn := dbtest.Open(t)
	users := user.NewService(conn)
	if _, err := users.Create(context.Background(), "sam", "pw", user.RoleAdmin); err != nil {
		t.Fatal(err)
	}
	sessions := auth.NewService(conn, users)

	clock := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	sessions.SetClock(func() time.Time { return clock })

	sess, err := sessions.Login(context.Background(), "sam", "pw")
	if err != nil {
		t.Fatal(err)
	}

	// Past renewAfter (24h), so Authenticate reports Renewed.
	clock = clock.Add(10 * 24 * time.Hour)
	h := auth.RequireSession(sessions, false)(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/images/x", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: sess.Token})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	found := false
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.CookieName {
			found = true
		}
	}
	if !found {
		t.Fatal("renewed session did not re-issue the cookie")
	}
}
