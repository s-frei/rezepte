package mailapi_test

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/config"
	"github.com/s-frei/rezepte/service/internal/db/dbtest"
	"github.com/s-frei/rezepte/service/internal/httpserver"
	"github.com/s-frei/rezepte/service/internal/mail"
	"github.com/s-frei/rezepte/service/internal/mailapi"
	"github.com/s-frei/rezepte/service/internal/user"
)

func newMailHandler(t *testing.T, env mail.Config) (http.Handler, map[string]*http.Cookie) {
	h, cookies, _ := newMailHandlerDB(t, env)
	return h, cookies
}

func newMailHandlerDB(t *testing.T, env mail.Config) (http.Handler, map[string]*http.Cookie, *sql.DB) {
	t.Helper()
	conn := dbtest.Open(t)
	users := user.NewService(conn, "")
	for name, role := range map[string]user.Role{"olga": user.RoleSuperadmin, "adam": user.RoleAdmin} {
		if _, err := users.Create(context.Background(), user.CreateParams{Username: name, Password: "pw", Role: role}); err != nil {
			t.Fatal(err)
		}
	}
	cfg, _ := config.LoadFrom(map[string]string{})
	sessions := auth.NewService(conn, users)
	tokens := auth.NewTokenService(conn, users)
	logger := slog.New(slog.DiscardHandler)
	srv := httpserver.New(cfg, logger, fstest.MapFS{},
		httpserver.WithAPIMiddleware(auth.Middleware(sessions, tokens, false)))
	auth.Register(srv.API(), sessions, false)
	mailapi.Register(srv.API(), mail.NewService(conn, env, "https://r.example.org", logger))
	h := srv.Handler()
	cookies := map[string]*http.Cookie{}
	for _, name := range []string{"olga", "adam"} {
		cookies[name] = login(t, h, name)
	}
	return h, cookies, conn
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

func login(t *testing.T, h http.Handler, username string) *http.Cookie {
	t.Helper()
	rec := do(h, http.MethodPost, "/api/v1/auth/login", `{"username":"`+username+`","password":"pw"}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("login %s: %d %s", username, rec.Code, rec.Body.String())
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.CookieName {
			return c
		}
	}
	t.Fatal("no session cookie")
	return nil
}

func TestMailSettingsOwnerOnly(t *testing.T) {
	h, cookies := newMailHandler(t, mail.Config{})
	if rec := do(h, "GET", "/api/v1/settings/mail", "", cookies["adam"]); rec.Code != 403 {
		t.Fatalf("admin GET = %d", rec.Code)
	}
	// Off still answers the defaults a first edit starts from, inside the enums.
	if rec := do(h, "GET", "/api/v1/settings/mail", "", cookies["olga"]); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"source":"none"`) ||
		!strings.Contains(rec.Body.String(), `"port":587`) || !strings.Contains(rec.Body.String(), `"security":"starttls"`) {
		t.Fatalf("owner GET = %d %s", rec.Code, rec.Body)
	}
}

func TestMailSettingsPutHidesPassword(t *testing.T) {
	h, cookies := newMailHandler(t, mail.Config{})
	body := `{"host":"smtp.example.org","port":587,"security":"starttls","username":"u","password":"secret","from":"r@example.org","fromName":"Rezepte"}`
	rec := do(h, "PUT", "/api/v1/settings/mail", body, cookies["olga"])
	if rec.Code != 200 || strings.Contains(rec.Body.String(), "secret") || !strings.Contains(rec.Body.String(), `"passwordSet":true`) {
		t.Fatalf("PUT = %d %s", rec.Code, rec.Body)
	}
}

func TestMailSettingsEnvLocked(t *testing.T) {
	h, cookies := newMailHandler(t, mail.Config{Host: "env", Port: 25, Security: mail.SecurityNone, From: "r@example.org", FromName: "Rezepte"})
	body := `{"host":"x","port":25,"security":"none","from":"r@example.org"}`
	if rec := do(h, "PUT", "/api/v1/settings/mail", body, cookies["olga"]); rec.Code != 409 {
		t.Fatalf("PUT = %d", rec.Code)
	}
	if rec := do(h, "DELETE", "/api/v1/settings/mail", "", cookies["olga"]); rec.Code != 409 {
		t.Fatalf("DELETE = %d", rec.Code)
	}
	if rec := do(h, "GET", "/api/v1/settings/mail", "", cookies["olga"]); !strings.Contains(rec.Body.String(), `"source":"env"`) {
		t.Fatalf("GET = %s", rec.Body)
	}
}

func TestSendTestReportsSMTPReply(t *testing.T) {
	h, cookies := newMailHandler(t, mail.Config{})
	// A port nothing listens on: the handler must answer 502 with a detail, not 500.
	body := `{"to":"olga@example.org","config":{"host":"127.0.0.1","port":1,"security":"none","from":"r@example.org"}}`
	rec := do(h, "POST", "/api/v1/settings/mail/test", body, cookies["olga"])
	if rec.Code != 502 || !strings.Contains(rec.Body.String(), "127.0.0.1:1") {
		t.Fatalf("test = %d %s", rec.Code, rec.Body)
	}
}

func TestSendTestShowsServerReplyUnquoted(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		fmt.Fprint(conn, "220 hi\r\n")
		r := bufio.NewReader(conn)
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			switch strings.ToUpper(strings.Fields(line + " x")[0]) {
			case "EHLO":
				fmt.Fprint(conn, "250-hi\r\n250 AUTH PLAIN\r\n")
			case "AUTH":
				fmt.Fprint(conn, "535 5.7.8 authentication failed\r\n")
			default:
				fmt.Fprint(conn, "250 ok\r\n")
			}
		}
	}()
	h, cookies := newMailHandler(t, mail.Config{})
	body := fmt.Sprintf(`{"to":"olga@example.org","config":{"host":"127.0.0.1","port":%d,"security":"none","username":"u","password":"p","from":"r@example.org"}}`, ln.Addr().(*net.TCPAddr).Port)
	rec := do(h, "POST", "/api/v1/settings/mail/test", body, cookies["olga"])
	if rec.Code != 502 || !strings.Contains(rec.Body.String(), `"detail":"send failed: sign-in: 535 5.7.8 authentication failed"`) {
		t.Fatalf("test = %d %s", rec.Code, rec.Body)
	}
}

func TestSendTestRejectsBadAddress(t *testing.T) {
	h, cookies := newMailHandler(t, mail.Config{})
	rec := do(h, "POST", "/api/v1/settings/mail/test", `{"to":"nope"}`, cookies["olga"])
	if rec.Code != 422 {
		t.Fatalf("test = %d", rec.Code)
	}
}

// Anything but a delivery failure stays a 500: a broken database is not the
// mail server's fault.
func TestSendTestOtherFailureIs500(t *testing.T) {
	h, cookies, conn := newMailHandlerDB(t, mail.Config{})
	if _, err := conn.ExecContext(context.Background(), `ALTER TABLE instance_settings RENAME TO gone`); err != nil {
		t.Fatal(err)
	}
	if rec := do(h, "POST", "/api/v1/settings/mail/test", `{"to":"olga@example.org"}`, cookies["olga"]); rec.Code != 500 {
		t.Fatalf("test = %d %s", rec.Code, rec.Body)
	}
}

func TestKeptPasswordRefusedForAnotherServer(t *testing.T) {
	h, cookies := newMailHandler(t, mail.Config{})
	body := `{"host":"smtp.example.org","port":587,"security":"starttls","username":"u","password":"secret","from":"r@example.org"}`
	if rec := do(h, "PUT", "/api/v1/settings/mail", body, cookies["olga"]); rec.Code != 200 {
		t.Fatalf("PUT = %d %s", rec.Code, rec.Body)
	}
	other := `{"host":"evil.example.org","port":587,"security":"starttls","username":"u","from":"r@example.org"}`
	if rec := do(h, "PUT", "/api/v1/settings/mail", other, cookies["olga"]); rec.Code != 422 || !strings.Contains(rec.Body.String(), "password") {
		t.Fatalf("PUT another host = %d %s", rec.Code, rec.Body)
	}
	test := `{"to":"olga@example.org","config":` + other + `}`
	if rec := do(h, "POST", "/api/v1/settings/mail/test", test, cookies["olga"]); rec.Code != 422 {
		t.Fatalf("test another host = %d %s", rec.Code, rec.Body)
	}
}
