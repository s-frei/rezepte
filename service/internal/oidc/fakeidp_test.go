package oidc_test

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"
)

// fakeIdP is the smallest OIDC provider the flow needs: discovery, JWKS and a
// token endpoint that answers with an ID token for whatever subject the test
// put behind the code. It signs RS256 with a key made per test.
type fakeIdP struct {
	*httptest.Server
	key      *rsa.PrivateKey
	clientID string
	mu       sync.Mutex
	codes    map[string]grant // code -> what the provider approved
	// tamper lets a test change the token after the claims are set.
	tamper func(claims map[string]any) map[string]any
	signer *rsa.PrivateKey // nil: key
}

// grant is what approve hands out a code for: the claims to issue, and the
// PKCE challenge and redirect URI the token request must match.
type grant struct {
	claims              idClaims
	challenge, redirect string
}

// callbackURL is the only redirect URI the fake provider accepts, as a real
// one accepts only the URIs registered for the client.
const callbackURL = publicURL + "/api/v1/auth/oidc/callback"

type idClaims struct {
	Subject, Email string
	EmailVerified  bool
	Nonce          string
}

func newFakeIdP(t *testing.T) *fakeIdP {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeIdP{key: key, clientID: "rezepte", codes: map[string]grant{}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                                f.URL,
			"authorization_endpoint":                f.URL + "/authorize",
			"token_endpoint":                        f.URL + "/token",
			"jwks_uri":                              f.URL + "/jwks",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("GET /jwks", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{map[string]any{
			"kty": "RSA", "alg": "RS256", "use": "sig", "kid": "k1",
			"n": b64(f.key.N.Bytes()), "e": b64(big.NewInt(int64(f.key.E)).Bytes()),
		}}})
	})
	mux.HandleFunc("POST /token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		f.mu.Lock()
		g, ok := f.codes[r.Form.Get("code")]
		delete(f.codes, r.Form.Get("code"))
		f.mu.Unlock()
		sum := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
		if !ok || b64(sum[:]) != g.challenge || r.Form.Get("redirect_uri") != g.redirect {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
			return
		}
		c := g.claims
		claims := map[string]any{
			"iss": f.URL, "aud": f.clientID, "sub": c.Subject, "nonce": c.Nonce,
			"email": c.Email, "email_verified": c.EmailVerified,
			"iat": time.Now().Unix(), "exp": time.Now().Add(time.Minute).Unix(),
		}
		if f.tamper != nil {
			claims = f.tamper(claims)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "at", "token_type": "Bearer", "expires_in": 60,
			"id_token": f.sign(t, claims),
		})
	})
	f.Server = httptest.NewServer(mux)
	t.Cleanup(f.Close)
	return f
}

// approve plays the provider's login page: it takes the authorize URL
// Rezepte redirected to and returns the callback query the provider would
// send back, for subject.
func (f *fakeIdP) approve(t *testing.T, authorizeURL string, c idClaims) (state, code string) {
	t.Helper()
	u := mustParse(t, authorizeURL)
	q := u.Query()
	if q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") == "" {
		t.Fatalf("no PKCE in %s", authorizeURL)
	}
	if q.Get("redirect_uri") != callbackURL {
		t.Fatalf("redirect_uri %q, want %q", q.Get("redirect_uri"), callbackURL)
	}
	c.Nonce = q.Get("nonce")
	code = rand.Text()
	f.mu.Lock()
	f.codes[code] = grant{claims: c, challenge: q.Get("code_challenge"), redirect: q.Get("redirect_uri")}
	f.mu.Unlock()
	return q.Get("state"), code
}

func (f *fakeIdP) sign(t *testing.T, claims map[string]any) string {
	t.Helper()
	key := f.key
	if f.signer != nil {
		key = f.signer
	}
	header := b64(mustJSON(t, map[string]any{"alg": "RS256", "typ": "JWT", "kid": "k1"}))
	payload := b64(mustJSON(t, claims))
	sum := sha256.Sum256([]byte(header + "." + payload))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatal(err)
	}
	return header + "." + payload + "." + b64(sig)
}

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func mustParse(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u
}
