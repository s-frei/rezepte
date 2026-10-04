package mail

import (
	"context"
	"encoding/base64"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/s-frei/rezepte/service/internal/db/dbtest"
	"github.com/s-frei/rezepte/service/internal/user"
)

var owner = user.User{ID: "o", Role: user.RoleSuperadmin}
var admin = user.User{ID: "a", Role: user.RoleAdmin}

var saved = Config{Host: "smtp.example.org", Port: 587, Security: SecuritySTARTTLS, Username: "u", Password: "p", From: "r@example.org", FromName: "Rezepte"}

func newSvc(t *testing.T, env Config, publicURL string) *Service {
	t.Helper()
	return NewService(dbtest.Open(t), env, publicURL, slog.New(slog.DiscardHandler))
}

func TestEffectiveNoneByDefault(t *testing.T) {
	s := newSvc(t, Config{}, "https://r.example.org")
	got, src, err := s.Effective(context.Background())
	if err != nil || src != SourceNone || s.Enabled(context.Background()) {
		t.Fatalf("src=%q err=%v enabled=%v", src, err, s.Enabled(context.Background()))
	}
	// None still carries the row's defaults, which a first edit starts from.
	if got != (Config{Port: 587, Security: SecuritySTARTTLS, FromName: "Rezepte"}) {
		t.Fatalf("defaults = %+v", got)
	}
}

func TestSaveThenEffective(t *testing.T) {
	s := newSvc(t, Config{}, "https://r.example.org")
	ctx := context.Background()
	if err := s.Save(ctx, owner, saved, false); err != nil {
		t.Fatal(err)
	}
	got, src, _ := s.Effective(ctx)
	if src != SourceSettings || got != saved || !s.Enabled(ctx) {
		t.Fatalf("got %+v from %q", got, src)
	}
}

func TestPutKeepsPasswordWhenOmitted(t *testing.T) {
	s := newSvc(t, Config{}, "https://r.example.org")
	ctx := context.Background()
	_ = s.Save(ctx, owner, saved, false)
	edit := saved
	edit.Port, edit.Password = 465, ""
	if err := s.Save(ctx, owner, edit, true); err != nil {
		t.Fatal(err)
	}
	got, _, _ := s.Effective(ctx)
	if got.Password != "p" || got.Port != 465 {
		t.Fatalf("got %+v", got)
	}
}

// The stored password only ever goes to the server it was stored for.
func TestKeptPasswordStaysWithItsServer(t *testing.T) {
	s := newSvc(t, Config{}, "https://r.example.org")
	ctx := context.Background()
	_ = s.Save(ctx, owner, saved, false)
	otherHost, otherUser := saved, saved
	otherHost.Host, otherHost.Password = "evil.example.org", ""
	otherUser.Username, otherUser.Password = "other", ""
	for name, c := range map[string]Config{"host": otherHost, "username": otherUser} {
		if err := s.Save(ctx, owner, c, true); !errors.Is(err, ErrPasswordRequired) || !errors.Is(err, ErrInvalidConfig) {
			t.Errorf("Save with another %s: err = %v, want ErrPasswordRequired", name, err)
		}
		if err := s.SendTest(ctx, owner, "olga@example.org", &c, true); !errors.Is(err, ErrPasswordRequired) {
			t.Errorf("SendTest with another %s: err = %v, want ErrPasswordRequired", name, err)
		}
	}
	if got, _, _ := s.Effective(ctx); got != saved {
		t.Fatalf("stored config changed: %+v", got)
	}
}

func TestKeptPasswordDroppedWithoutSignIn(t *testing.T) {
	s := newSvc(t, Config{}, "https://r.example.org")
	ctx := context.Background()
	_ = s.Save(ctx, owner, saved, false)
	open := saved
	open.Host, open.Username, open.Password = "relay.example.org", "", ""
	if err := s.Save(ctx, owner, open, true); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if got, _, _ := s.Effective(ctx); got.Host != "relay.example.org" || got.Password != "" {
		t.Fatalf("stored = %+v, want no password", got)
	}
	got, err := s.withStoredPassword(ctx, open, true)
	if err != nil || got.Password != "" {
		t.Fatalf("withStoredPassword = %+v, %v", got, err)
	}
}

func TestEnvBeatsSettingsAndLocks(t *testing.T) {
	env := saved
	env.Host = "env.example.org"
	s := newSvc(t, env, "https://r.example.org")
	ctx := context.Background()
	if err := s.Save(ctx, owner, saved, false); !errors.Is(err, ErrEnvLocked) {
		t.Fatalf("Save err = %v", err)
	}
	if err := s.Clear(ctx, owner); !errors.Is(err, ErrEnvLocked) {
		t.Fatalf("Clear err = %v", err)
	}
	got, src, _ := s.Effective(ctx)
	if src != SourceEnv || got.Host != "env.example.org" {
		t.Fatalf("got %+v from %q", got, src)
	}
}

func TestOnlyOwnerWrites(t *testing.T) {
	s := newSvc(t, Config{}, "https://r.example.org")
	if err := s.Save(context.Background(), admin, saved, false); !errors.Is(err, ErrOwnerRequired) {
		t.Fatalf("err = %v", err)
	}
}

func TestSaveValidates(t *testing.T) {
	s := newSvc(t, Config{}, "https://r.example.org")
	bad := saved
	bad.From = "not an address"
	if err := s.Save(context.Background(), owner, bad, false); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("err = %v", err)
	}
}

func TestDisabledWithoutPublicURL(t *testing.T) {
	s := newSvc(t, Config{}, "")
	ctx := context.Background()
	_ = s.Save(ctx, owner, saved, false)
	if s.Enabled(ctx) {
		t.Fatal("enabled without a public URL")
	}
	if err := s.SendInvite(ctx, Invite{To: "a@b.c"}); !errors.Is(err, ErrDisabled) {
		t.Fatalf("err = %v", err)
	}
}

func TestClear(t *testing.T) {
	s := newSvc(t, Config{}, "https://r.example.org")
	ctx := context.Background()
	_ = s.Save(ctx, owner, saved, false)
	if err := s.Clear(ctx, owner); err != nil {
		t.Fatal(err)
	}
	if _, src, _ := s.Effective(ctx); src != SourceNone {
		t.Fatalf("src = %q", src)
	}
}

func TestSendTestOverrideUsesStoredPassword(t *testing.T) {
	s := newSvc(t, Config{}, "https://r.example.org")
	ctx := context.Background()
	f := &fakeSMTP{ext: []string{"STARTTLS", "AUTH PLAIN"}, authReply: "235 ok"}
	f.tlsConf = testTLS(t)
	host, port := startFake(t, f)
	stored := saved
	stored.Host = host
	if err := s.Save(ctx, owner, stored, false); err != nil {
		t.Fatal(err)
	}
	override := Config{Host: host, Port: port, Security: SecuritySTARTTLS, Username: "u", From: "r@example.org"}
	if err := s.SendTest(ctx, owner, "olga@example.org", &override, true); err != nil {
		t.Fatalf("SendTest: %v", err)
	}
	want := base64.StdEncoding.EncodeToString([]byte("\x00u\x00p"))
	if got := <-f.auth; !strings.HasSuffix(got, want) {
		t.Fatalf("AUTH = %q, want the stored password p in %q", got, want)
	}
}

func TestNewMailsDisabledWithoutConfig(t *testing.T) {
	s := newSvc(t, Config{}, "https://r.example.org")
	ctx := context.Background()
	if err := s.SendReset(ctx, Reset{To: "a@b.c", Username: "a", Path: "/welcome#t"}); !errors.Is(err, ErrDisabled) {
		t.Errorf("SendReset: %v", err)
	}
	if err := s.SendHint(ctx, Hint{To: "a@b.c"}); !errors.Is(err, ErrDisabled) {
		t.Errorf("SendHint: %v", err)
	}
	if err := s.SendConfirm(ctx, Confirm{To: "a@b.c", Path: "/confirm-email#t"}); !errors.Is(err, ErrDisabled) {
		t.Errorf("SendConfirm: %v", err)
	}
}
