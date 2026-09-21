package main

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/s-frei/rezepte/service/internal/user"
)

// TestResetErrorKeepsTheSentinel pins both halves of the translation at once,
// because each is easy to buy with the other: the sentinel stays reachable
// for errors.Is, and the printed message stays the operator's. fmt.Errorf
// with %w satisfies the first and breaks the second - it renders the
// sentinel's own startup wording, which tells someone who mistyped
// REZEPTE_DATA_DIR to recreate a database they still have. Hence the second
// assertion: the sentinel's own sentence must not reach the operator running
// a reset.
func TestResetErrorKeepsTheSentinel(t *testing.T) {
	const dir = "/srv/rezepte/data"
	for _, tc := range []struct {
		name     string
		err      error
		contains string
	}{
		{"no instance at the data directory", user.ErrNoSuperadmin, dir},
		{"no password for the reset", user.ErrAdminPasswordRequired, "REZEPTE_ADMIN_PASSWORD"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := resetError(tc.err, dir)
			if !errors.Is(got, tc.err) {
				t.Errorf("errors.Is(%v, %v) = false, want true", got, tc.err)
			}
			if !strings.Contains(got.Error(), tc.contains) {
				t.Errorf("message %q does not mention %q", got, tc.contains)
			}
			// Against the sentinel's own text rather than a copy of it, so
			// rewording the sentinel cannot quietly retire the assertion.
			if strings.Contains(got.Error(), tc.err.Error()) {
				t.Errorf("message %q leaks the startup wording %q", got, tc.err)
			}
		})
	}

	// Anything else passes through untouched: the reset only knows how to
	// restate the two errors it can actually provoke.
	other := errors.New("disk on fire")
	if got := resetError(other, dir); !errors.Is(got, other) || got.Error() != other.Error() {
		t.Errorf("resetError(%v) = %v, want it unchanged", other, got)
	}
}

// TestDemoCredentialsNeverStandInForAReset covers the one flag combination
// that would set the instance owner's password to a credential printed in the
// user documentation: --demo --reset-superadmin-password with nothing
// configured. The demo fallback is for a start with no configuration at all;
// a reset with no REZEPTE_ADMIN_PASSWORD must reach
// user.ErrAdminPasswordRequired instead.
func TestDemoCredentialsNeverStandInForAReset(t *testing.T) {
	for _, tc := range []struct {
		name            string
		demoMode, reset bool
		adminPassword   string
		want            bool
	}{
		{"demo start with no password configured", true, false, "", true},
		{"demo start with a password configured", true, false, "secret123", false},
		{"demo reset with no password configured", true, true, "", false},
		{"demo reset with a password configured", true, true, "secret123", false},
		{"plain start", false, false, "", false},
		{"plain reset", false, true, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := useDemoDefaults(tc.demoMode, tc.reset, tc.adminPassword); got != tc.want {
				t.Errorf("useDemoDefaults(%t, %t, %q) = %t, want %t", tc.demoMode, tc.reset, tc.adminPassword, got, tc.want)
			}
		})
	}
}

// listenerAddr starts a server on the loopback interface and returns the
// address in the form REZEPTE_ADDR takes, so the tests exercise the same
// parsing the container's HEALTHCHECK does.
func listenerAddr(t *testing.T, status int) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)

	_, port, err := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	if err != nil {
		t.Fatalf("split %q: %v", srv.URL, err)
	}
	return ":" + port
}

func TestHealthcheckAcceptsAHealthyInstance(t *testing.T) {
	if err := healthcheck(listenerAddr(t, http.StatusOK)); err != nil {
		t.Fatalf("healthcheck of a healthy instance: %v", err)
	}
}

func TestHealthcheckRejectsANonOKStatus(t *testing.T) {
	if err := healthcheck(listenerAddr(t, http.StatusServiceUnavailable)); err == nil {
		t.Fatal("healthcheck accepted a 503, want an error")
	}
}

func TestHealthcheckRejectsAClosedPort(t *testing.T) {
	// Take a port, then give it back: nothing is listening on it afterwards,
	// which is what a container whose server has died looks like.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	_, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("split %q: %v", ln.Addr(), err)
	}
	if err := ln.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	if err := healthcheck(":" + port); err == nil {
		t.Fatal("healthcheck accepted a closed port, want an error")
	}
}

func TestHealthcheckRejectsAnUnparsableAddress(t *testing.T) {
	if err := healthcheck("not-an-address"); err == nil {
		t.Fatal("healthcheck accepted a malformed address, want an error")
	}
}
