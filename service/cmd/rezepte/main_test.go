package main

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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
