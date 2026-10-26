package oidc

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/s-frei/rezepte/service/internal/db/dbtest"
	"github.com/s-frei/rezepte/service/internal/user"
)

var (
	owner = user.User{ID: "o", Role: user.RoleSuperadmin}
	admin = user.User{ID: "a", Role: user.RoleAdmin}
)

const testPublicURL = "https://r.example.org"

// newSettings returns Settings on a fresh database whose discovery confirms
// whatever issuer it is asked for.
func newSettings(t *testing.T, conn *sql.DB, env Config) *Settings {
	t.Helper()
	s := NewSettings(conn, env)
	s.discover = func(_ context.Context, issuer string) (string, error) { return issuer, nil }
	return s
}

func secret(v string) *string { return &v }

var stored = Config{Issuer: "https://id.example", ClientID: "rezepte", Name: "Example ID"}

func TestEnvWinsAndNeverSeeds(t *testing.T) {
	ctx := context.Background()
	conn := dbtest.Open(t)
	if err := newSettings(t, conn, Config{PublicURL: testPublicURL}).Save(ctx, owner, stored, secret("s3")); err != nil {
		t.Fatal(err)
	}
	env := Config{PublicURL: testPublicURL, Issuer: "https://env.example", ClientID: "env", Name: "Env ID"}
	s := newSettings(t, conn, env)
	got, src, err := s.Effective(ctx)
	if err != nil || src != SourceEnv || got != env {
		t.Fatalf("got %+v from %q, %v", got, src, err)
	}
	if err := s.Save(ctx, owner, Config{Issuer: "https://x.example", ClientID: "x"}, nil); !errors.Is(err, ErrEnvLocked) {
		t.Fatalf("Save = %v, want ErrEnvLocked", err)
	}
	if err := s.Clear(ctx, owner); !errors.Is(err, ErrEnvLocked) {
		t.Fatalf("Clear = %v, want ErrEnvLocked", err)
	}
	got, src, _ = newSettings(t, conn, Config{PublicURL: testPublicURL}).Effective(ctx)
	want := stored
	want.PublicURL, want.ClientSecret = testPublicURL, "s3"
	if src != SourceSettings || got != want {
		t.Fatalf("without env: %+v from %q", got, src)
	}
}

func TestEffectiveNoneByDefault(t *testing.T) {
	s := newSettings(t, dbtest.Open(t), Config{PublicURL: testPublicURL})
	got, src, err := s.Effective(context.Background())
	if err != nil || src != SourceNone || got != (Config{PublicURL: testPublicURL}) || s.Enabled(context.Background()) {
		t.Fatalf("got %+v from %q, %v", got, src, err)
	}
}

func TestEffectiveDefaultsName(t *testing.T) {
	ctx := context.Background()
	s := newSettings(t, dbtest.Open(t), Config{PublicURL: testPublicURL})
	c := stored
	c.Name = " "
	if err := s.Save(ctx, owner, c, nil); err != nil {
		t.Fatal(err)
	}
	if got, _, _ := s.Effective(ctx); got.Name != "single sign-on" || !s.Enabled(ctx) {
		t.Fatalf("got %+v", got)
	}
}

func TestSaveRequiresOwner(t *testing.T) {
	s := newSettings(t, dbtest.Open(t), Config{PublicURL: testPublicURL})
	if err := s.Save(context.Background(), admin, stored, nil); !errors.Is(err, ErrOwnerRequired) {
		t.Fatalf("Save = %v", err)
	}
	if err := s.Clear(context.Background(), admin); !errors.Is(err, ErrOwnerRequired) {
		t.Fatalf("Clear = %v", err)
	}
}

func TestSavePublicURLMissing(t *testing.T) {
	s := newSettings(t, dbtest.Open(t), Config{})
	if err := s.Save(context.Background(), owner, stored, nil); !errors.Is(err, ErrPublicURLMissing) {
		t.Fatalf("Save = %v", err)
	}
}

func TestSaveRejectsMissingFields(t *testing.T) {
	s := newSettings(t, dbtest.Open(t), Config{PublicURL: testPublicURL})
	for _, c := range []Config{{ClientID: "x"}, {Issuer: "https://id.example"}, {Issuer: " ", ClientID: " "}} {
		if err := s.Save(context.Background(), owner, c, nil); !errors.Is(err, ErrInvalidConfig) {
			t.Errorf("Save(%+v) = %v", c, err)
		}
	}
}

func TestSaveDiscoveryFailureStoresNothing(t *testing.T) {
	ctx := context.Background()
	s := newSettings(t, dbtest.Open(t), Config{PublicURL: testPublicURL})
	cause := errors.New("connection refused")
	s.discover = func(context.Context, string) (string, error) { return "", cause }
	if err := s.Save(ctx, owner, stored, secret("s3")); !errors.Is(err, ErrDiscovery) || !errors.Is(err, cause) {
		t.Fatalf("Save = %v", err)
	}
	if _, src, _ := s.Effective(ctx); src != SourceNone {
		t.Fatalf("source %q after a failed save", src)
	}
}

func TestSaveStoresIssuerAsDiscovered(t *testing.T) {
	ctx := context.Background()
	s := newSettings(t, dbtest.Open(t), Config{PublicURL: testPublicURL})
	s.discover = func(context.Context, string) (string, error) { return "https://id.example", nil }
	c := stored
	c.Issuer = " https://id.example/ "
	if err := s.Save(ctx, owner, c, nil); err != nil {
		t.Fatal(err)
	}
	if got, _, _ := s.Effective(ctx); got.Issuer != "https://id.example" {
		t.Fatalf("issuer %q", got.Issuer)
	}
}

func TestSaveKeepsSecretForSameProvider(t *testing.T) {
	ctx := context.Background()
	s := newSettings(t, dbtest.Open(t), Config{PublicURL: testPublicURL})
	if err := s.Save(ctx, owner, stored, secret("s3")); err != nil {
		t.Fatal(err)
	}
	c := stored
	c.Name = "Renamed"
	if err := s.Save(ctx, owner, c, nil); err != nil {
		t.Fatal(err)
	}
	if got, _, _ := s.Effective(ctx); got.ClientSecret != "s3" || got.Name != "Renamed" {
		t.Fatalf("got %+v", got)
	}
}

// The stored secret only ever goes to the provider and client it was stored for.
func TestKeptSecretStaysWithItsProvider(t *testing.T) {
	ctx := context.Background()
	s := newSettings(t, dbtest.Open(t), Config{PublicURL: testPublicURL})
	if err := s.Save(ctx, owner, stored, secret("s3")); err != nil {
		t.Fatal(err)
	}
	otherIssuer, otherClient := stored, stored
	otherIssuer.Issuer = "https://evil.example"
	otherClient.ClientID = "other"
	for name, c := range map[string]Config{"issuer": otherIssuer, "client ID": otherClient} {
		if err := s.Save(ctx, owner, c, nil); !errors.Is(err, ErrSecretRequired) || !errors.Is(err, ErrInvalidConfig) {
			t.Errorf("another %s: %v, want ErrSecretRequired", name, err)
		}
	}
	if got, _, _ := s.Effective(ctx); got.Issuer != stored.Issuer || got.ClientSecret != "s3" {
		t.Fatalf("got %+v", got)
	}
}

func TestEmptySecretClears(t *testing.T) {
	ctx := context.Background()
	s := newSettings(t, dbtest.Open(t), Config{PublicURL: testPublicURL})
	if err := s.Save(ctx, owner, stored, secret("s3")); err != nil {
		t.Fatal(err)
	}
	if err := s.Save(ctx, owner, stored, secret("")); err != nil {
		t.Fatal(err)
	}
	if got, src, _ := s.Effective(ctx); got.ClientSecret != "" || src != SourceSettings {
		t.Fatalf("got %+v from %q", got, src)
	}
}

func TestConnectedCounts(t *testing.T) {
	ctx := context.Background()
	conn := dbtest.Open(t)
	s := newSettings(t, conn, Config{PublicURL: testPublicURL})
	if total, without, err := s.Connected(ctx); err != nil || total != 0 || without != 0 {
		t.Fatalf("none: %d %d %v", total, without, err)
	}
	if err := s.Save(ctx, owner, stored, nil); err != nil {
		t.Fatal(err)
	}
	users := user.NewService(conn, "")
	for _, tc := range []struct{ name, password, issuer string }{
		{"sam", "pw", stored.Issuer},
		{"anna", "", stored.Issuer},
		{"kim", "", "https://old.example"},
	} {
		u, err := users.Create(ctx, user.CreateParams{Username: tc.name, Password: tc.password, Role: user.RoleUser})
		if err != nil {
			t.Fatal(err)
		}
		if err := users.LinkIdentity(ctx, u.ID, tc.issuer, "sub-"+tc.name); err != nil {
			t.Fatal(err)
		}
	}
	if total, without, err := s.Connected(ctx); err != nil || total != 2 || without != 1 {
		t.Fatalf("got %d %d %v, want 2 1", total, without, err)
	}
}

func TestClearRemovesEverything(t *testing.T) {
	ctx := context.Background()
	conn := dbtest.Open(t)
	s := newSettings(t, conn, Config{PublicURL: testPublicURL})
	if err := s.Save(ctx, owner, stored, secret("s3")); err != nil {
		t.Fatal(err)
	}
	if err := s.Clear(ctx, owner); err != nil {
		t.Fatal(err)
	}
	if _, src, _ := s.Effective(ctx); src != SourceNone {
		t.Fatalf("source %q", src)
	}
	if st, err := s.stored(ctx); err != nil || st != (Config{}) {
		t.Fatalf("stored %+v, %v", st, err)
	}
}

// TestDiscoverConfirmsIssuer: the real discovery answers the provider's own
// issuer, also for a typed trailing slash it does not use, and fails where
// no provider answers.
func TestDiscoverConfirmsIssuer(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/openid-configuration" {
			http.NotFound(w, r)
			return
		}
		_, _ = fmt.Fprintf(w, `{"issuer":%q,"authorization_endpoint":"%[1]s/authorize","token_endpoint":"%[1]s/token","jwks_uri":"%[1]s/jwks"}`, srv.URL)
	}))
	defer srv.Close()
	for _, typed := range []string{srv.URL, srv.URL + "/"} {
		if got, err := discover(context.Background(), typed); err != nil || got != srv.URL {
			t.Errorf("discover(%q) = %q, %v", typed, got, err)
		}
	}
	if _, err := discover(context.Background(), srv.URL+"/nothing"); err == nil {
		t.Error("discovery succeeded where no provider answers")
	}
}
