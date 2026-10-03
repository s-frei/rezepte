package recipeimport_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/s-frei/rezepte/service/internal/recipeimport"
)

func TestPublicOnly(t *testing.T) {
	for addr, want := range map[string]bool{
		"93.184.216.34":    true,
		"2606:4700::1111":  true,
		"127.0.0.1":        false,
		"::1":              false,
		"10.1.2.3":         false,
		"172.16.0.1":       false,
		"192.168.1.1":      false,
		"169.254.169.254":  false,
		"100.64.0.1":       false,
		"0.0.0.0":          false,
		"::":               false,
		"fe80::1":          false,
		"fd00::1":          false,
		"224.0.0.1":        false,
		"::ffff:127.0.0.1": false,
		"::ffff:10.0.0.1":  false,
		"64:ff9b::a00:1":   false,
		"64:ff9b:1::1":     false,
		"2002:a00:1::1":    false,
		"2001::1":          false,
		"::a00:1":          false,
		"fec0::1":          false,
		"192.0.0.1":        false,
		"198.18.0.1":       false,
		"240.0.0.1":        false,
	} {
		if got := recipeimport.PublicOnly(netip.MustParseAddr(addr)); got != want {
			t.Errorf("PublicOnly(%s) = %v, want %v", addr, got, want)
		}
	}
}

func allowAll(netip.Addr) bool { return true }

func TestFetcherRefusesLoopbackInProduction(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("hi")) }))
	defer srv.Close()
	f := recipeimport.NewFetcher("test", recipeimport.PublicOnly)
	if _, err := f.Get(context.Background(), srv.URL, "text/html", 1<<20); !errors.Is(err, recipeimport.ErrUnreachable) {
		t.Fatalf("err = %v, want ErrUnreachable", err)
	}
}

func TestFetcherRefusesRedirectIntoPrivateNetwork(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://192.168.1.1/admin", http.StatusFound)
	}))
	defer srv.Close()
	onlyLoopback := func(a netip.Addr) bool { return a.IsLoopback() }
	f := recipeimport.NewFetcher("test", onlyLoopback)
	if _, err := f.Get(context.Background(), srv.URL, "text/html", 1<<20); !errors.Is(err, recipeimport.ErrUnreachable) {
		t.Fatalf("err = %v, want ErrUnreachable", err)
	}
}

func TestFetcherReadsBodyAndFinalURL(t *testing.T) {
	var mu sync.Mutex
	var hits []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits = append(hits, r.Header.Get("User-Agent"))
		mu.Unlock()
		if r.URL.Path == "/old" {
			http.Redirect(w, r, "/new", http.StatusMovedPermanently)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<html></html>"))
	}))
	defer srv.Close()
	got, err := recipeimport.NewFetcher("1.2.3", allowAll).Get(context.Background(), srv.URL+"/old", "text/html", 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if string(got.Body) != "<html></html>" || got.URL.Path != "/new" || !strings.HasPrefix(got.ContentType, "text/html") {
		t.Fatalf("got %+v", got)
	}
	mu.Lock()
	defer mu.Unlock()
	if !strings.HasPrefix(hits[0], "Rezepte/1.2.3") {
		t.Fatalf("User-Agent = %q", hits[0])
	}
}

func TestFetcherLimits(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/big":
			_, _ = w.Write([]byte(strings.Repeat("x", 101)))
		case "/slow":
			time.Sleep(200 * time.Millisecond)
		case "/loop":
			http.Redirect(w, r, "/loop", http.StatusFound)
		}
	}))
	defer srv.Close()
	f := recipeimport.NewFetcher("test", allowAll)
	if _, err := f.Get(context.Background(), srv.URL+"/big", "", 100); !errors.Is(err, recipeimport.ErrUnreachable) {
		t.Errorf("big: err = %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := f.Get(ctx, srv.URL+"/slow", "", 100); !errors.Is(err, recipeimport.ErrUnreachable) {
		t.Errorf("slow: err = %v", err)
	}
	if _, err := f.Get(context.Background(), srv.URL+"/loop", "", 100); !errors.Is(err, recipeimport.ErrUnreachable) {
		t.Errorf("loop: err = %v", err)
	}
	for _, raw := range []string{"file:///etc/passwd", "ftp://example.com/", "javascript:alert(1)", "not a url"} {
		if _, err := f.Get(context.Background(), raw, "", 100); !errors.Is(err, recipeimport.ErrUnreachable) {
			t.Errorf("%s: err = %v", raw, err)
		}
	}
}

func TestFetcherRedirects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ftp" {
			http.Redirect(w, r, "ftp://example.com/recipe", http.StatusFound)
			return
		}
		// /hop/N redirects N more times, then answers.
		n, _ := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/hop/"))
		if n > 0 {
			http.Redirect(w, r, "/hop/"+strconv.Itoa(n-1), http.StatusFound)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()
	f := recipeimport.NewFetcher("test", allowAll)
	if _, err := f.Get(context.Background(), srv.URL+"/hop/5", "", 100); err != nil {
		t.Errorf("5 redirects: err = %v", err)
	}
	if _, err := f.Get(context.Background(), srv.URL+"/hop/6", "", 100); !errors.Is(err, recipeimport.ErrUnreachable) {
		t.Errorf("6 redirects: err = %v", err)
	}
	if _, err := f.Get(context.Background(), srv.URL+"/ftp", "", 100); !errors.Is(err, recipeimport.ErrUnreachable) {
		t.Errorf("redirect to ftp: err = %v", err)
	}
}
