package oidc_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/oidc"
	"github.com/s-frei/rezepte/service/internal/user"
)

const signInPath = "/api/v1/settings/sign-in"

// ownerSession adds the instance owner to e and signs them in.
func (e *env) ownerSession(t *testing.T) *http.Cookie {
	t.Helper()
	return e.sessionOf(t, e.create(t, "owner", user.RoleSuperadmin))
}

func signInBody(issuer, secret string) string {
	b := `{"issuer":"` + issuer + `","clientId":"rezepte","name":"Test IdP"`
	if secret != "" {
		b += `,"clientSecret":"` + secret + `"`
	}
	return b + "}"
}

func getSignIn(t *testing.T, e *env, session *http.Cookie) map[string]any {
	t.Helper()
	rec := api(e.h, http.MethodGet, signInPath, "", session)
	if rec.Code != http.StatusOK {
		t.Fatalf("get: %d %s", rec.Code, rec.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	return got
}

func TestGetNeverReturnsSecret(t *testing.T) {
	e := newServer(t, nil)
	idp := newFakeIdP(t)
	owner := e.ownerSession(t)
	if rec := api(e.h, http.MethodPut, signInPath, signInBody(idp.URL, "s3cr3t-value"), owner); rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "s3cr3t-value") {
		t.Fatalf("put: %d %s", rec.Code, rec.Body.String())
	}
	rec := api(e.h, http.MethodGet, signInPath, "", owner)
	if strings.Contains(rec.Body.String(), "s3cr3t-value") {
		t.Fatalf("secret in %s", rec.Body.String())
	}
	got := getSignIn(t, e, owner)
	want := map[string]any{
		"source": "settings", "issuer": idp.URL, "clientId": "rezepte", "secretSet": true, "name": "Test IdP",
		"redirectUri": callbackURL, "publicUrlMissing": false,
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %v, want %v", k, got[k], v)
		}
	}
}

func TestGetOffAndOwnerOnly(t *testing.T) {
	e := newServer(t, nil)
	if rec := api(e.h, http.MethodGet, signInPath, "", e.passwordLogin(t)); rec.Code != http.StatusForbidden {
		t.Fatalf("admin get: %d", rec.Code)
	}
	got := getSignIn(t, e, e.ownerSession(t))
	if got["source"] != "none" || got["issuer"] != "" || got["secretSet"] != false || got["redirectUri"] != callbackURL {
		t.Fatalf("off: %v", got)
	}
}

func TestPutRequiresOwner(t *testing.T) {
	e := newServer(t, nil)
	idp := newFakeIdP(t)
	admin := e.passwordLogin(t)
	if rec := api(e.h, http.MethodPut, signInPath, signInBody(idp.URL, "x"), admin); rec.Code != http.StatusForbidden {
		t.Fatalf("admin put: %d %s", rec.Code, rec.Body.String())
	}
	if rec := api(e.h, http.MethodDelete, signInPath, "", admin); rec.Code != http.StatusForbidden {
		t.Fatalf("admin delete: %d", rec.Code)
	}
}

func TestPutEnvLocked(t *testing.T) {
	e := newEnv(t)
	owner := e.ownerSession(t)
	if rec := api(e.h, http.MethodPut, signInPath, signInBody(newFakeIdP(t).URL, "x"), owner); rec.Code != http.StatusConflict {
		t.Fatalf("put: %d %s", rec.Code, rec.Body.String())
	}
	if rec := api(e.h, http.MethodDelete, signInPath, "", owner); rec.Code != http.StatusConflict {
		t.Fatalf("delete: %d", rec.Code)
	}
	got := getSignIn(t, e, owner)
	if got["source"] != "env" || got["issuer"] != e.idp.URL || got["secretSet"] != true {
		t.Fatalf("env: %v", got)
	}
}

func TestPutDiscoveryFailureStoresNothing(t *testing.T) {
	e := newServer(t, nil)
	owner := e.ownerSession(t)
	rec := api(e.h, http.MethodPut, signInPath, signInBody("http://127.0.0.1:1", "x"), owner)
	var body struct {
		Errors []struct{ Location, Message string }
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if rec.Code != http.StatusUnprocessableEntity || len(body.Errors) != 1 || body.Errors[0].Location != "body.issuer" {
		t.Fatalf("put: %d %s", rec.Code, rec.Body.String())
	}
	if got := getSignIn(t, e, owner); got["source"] != "none" {
		t.Fatalf("after a failed put: %v", got)
	}
}

func TestPutSecretRequiredForOtherProvider(t *testing.T) {
	e := newServer(t, nil)
	owner := e.ownerSession(t)
	first := newFakeIdP(t)
	if rec := api(e.h, http.MethodPut, signInPath, signInBody(first.URL, "s3"), owner); rec.Code != http.StatusOK {
		t.Fatalf("put: %d %s", rec.Code, rec.Body.String())
	}
	rec := api(e.h, http.MethodPut, signInPath, signInBody(newFakeIdP(t).URL, ""), owner)
	var body struct {
		Errors []struct{ Location string }
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if rec.Code != http.StatusUnprocessableEntity || len(body.Errors) != 1 || body.Errors[0].Location != "body.clientSecret" {
		t.Fatalf("put another provider without secret: %d %s", rec.Code, rec.Body.String())
	}
	// The same provider keeps it; an empty secret removes it.
	if got := getSignIn(t, e, owner); got["issuer"] != first.URL || got["secretSet"] != true {
		t.Fatalf("after the refusal: %v", got)
	}
	cleared := `{"issuer":"` + first.URL + `","clientId":"rezepte","clientSecret":""}`
	if rec := api(e.h, http.MethodPut, signInPath, cleared, owner); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"secretSet":false`) {
		t.Fatalf("clear secret: %d %s", rec.Code, rec.Body.String())
	}
}

func TestPutWithoutPublicURL(t *testing.T) {
	e := newServer(t, &oidc.Config{})
	owner := e.ownerSession(t)
	if rec := api(e.h, http.MethodPut, signInPath, signInBody(newFakeIdP(t).URL, "x"), owner); rec.Code != http.StatusConflict {
		t.Fatalf("put: %d %s", rec.Code, rec.Body.String())
	}
	if got := getSignIn(t, e, owner); got["publicUrlMissing"] != true || got["redirectUri"] != "" {
		t.Fatalf("get: %v", got)
	}
}

func TestDeleteClears(t *testing.T) {
	e := newServer(t, nil)
	owner := e.ownerSession(t)
	if rec := api(e.h, http.MethodPut, signInPath, signInBody(newFakeIdP(t).URL, "s3"), owner); rec.Code != http.StatusOK {
		t.Fatalf("put: %d %s", rec.Code, rec.Body.String())
	}
	if rec := api(e.h, http.MethodDelete, signInPath, "", owner); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body.String())
	}
	if got := getSignIn(t, e, owner); got["source"] != "none" || got["secretSet"] != false || got["issuer"] != "" {
		t.Fatalf("after delete: %v", got)
	}
}

func TestGetCounts(t *testing.T) {
	e := newServer(t, nil)
	owner := e.ownerSession(t)
	e.configure(t, newFakeIdP(t))
	e.link(t, e.sam, "sub-sam")
	e.link(t, e.anna, "sub-anna")
	if err := e.users.LinkIdentity(context.Background(), e.create(t, "kim", user.RoleUser).ID, "https://old.example", "sub-kim"); err != nil {
		t.Fatal(err)
	}
	if got := getSignIn(t, e, owner); got["connected"] != float64(2) || got["connectedWithoutPassword"] != float64(1) {
		t.Fatalf("counts: %v", got)
	}
}

func TestSignInRefusesAPITokens(t *testing.T) {
	e := newServer(t, nil)
	raw, _, err := e.tokens.Create(context.Background(), e.sam.ID, "reader", []string{auth.ScopeRecipesRead}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		req := httptest.NewRequest(method, signInPath, strings.NewReader(signInBody("https://id.example", "")))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", publicURL)
		req.Header.Set("Authorization", "Bearer "+raw)
		if rec := send(e.h, req); rec.Code != http.StatusForbidden {
			t.Errorf("%s with a token: %d, want 403: %s", method, rec.Code, rec.Body.String())
		}
	}
}
