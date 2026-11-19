package update

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

func feed(t *testing.T, body string, code int) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n.Add(1)
		w.WriteHeader(code)
		fmt.Fprint(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv, &n
}

func release(tag string, pre, draft bool) string {
	return fmt.Sprintf(`{"tag_name":%q,"published_at":"2026-11-01T10:00:00Z","prerelease":%t,"draft":%t}`, tag, pre, draft)
}

func check(t *testing.T, body string, code int) Status {
	t.Helper()
	srv, _ := feed(t, body, code)
	s := NewService("1.5.0", srv.URL, true, srv.Client(), quiet)
	s.Check(context.Background())
	return s.Status()
}

func TestCheckAvailable(t *testing.T) {
	st := check(t, release("v1.6.0", false, false), 200)
	if st.State != "available" || st.Latest == nil || st.Latest.Version != "1.6.0" {
		t.Fatalf("status = %+v", st)
	}
	if want := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC); !st.Latest.PublishedAt.Equal(want) {
		t.Errorf("PublishedAt = %v", st.Latest.PublishedAt)
	}
}

func TestCheckCurrent(t *testing.T) {
	st := check(t, release("v1.5.0", false, false), 200)
	if st.State != "current" || st.Latest != nil {
		t.Fatalf("status = %+v", st)
	}
}

func TestPrereleaseIgnored(t *testing.T) {
	for _, body := range []string{release("v9.0.0", true, false), release("v9.0.0", false, true)} {
		if st := check(t, body, 200); st.State != "unknown" {
			t.Errorf("first check, %s: state = %q", body, st.State)
		}
		var pre atomic.Bool
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if pre.Load() {
				fmt.Fprint(w, body)
				return
			}
			fmt.Fprint(w, release("v1.6.0", false, false))
		}))
		s := NewService("1.5.0", srv.URL, true, srv.Client(), quiet)
		s.Check(context.Background())
		pre.Store(true)
		s.Check(context.Background())
		srv.Close()
		if st := s.Status(); st.State != "available" || st.Latest.Version != "1.6.0" {
			t.Errorf("after available, %s: status = %+v", body, st)
		}
	}
}

func TestFirstCheckFailureIsUnknown(t *testing.T) {
	if st := check(t, "", 500); st.State != "unknown" {
		t.Fatalf("state = %q", st.State)
	}
}

func TestMalformedJSON(t *testing.T) {
	if st := check(t, "{", 200); st.State != "unknown" {
		t.Fatalf("state = %q", st.State)
	}
}

func TestCheckFailureKeepsPrevious(t *testing.T) {
	var fail atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail.Load() {
			w.WriteHeader(500)
			return
		}
		fmt.Fprint(w, release("v1.6.0", false, false))
	}))
	defer srv.Close()
	s := NewService("1.5.0", srv.URL, true, srv.Client(), quiet)
	s.Check(context.Background())
	fail.Store(true)
	s.Check(context.Background())
	if st := s.Status(); st.State != "available" || st.Latest.Version != "1.6.0" {
		t.Fatalf("status = %+v", st)
	}
}

func TestStatusIsACopy(t *testing.T) {
	srv, _ := feed(t, release("v1.6.0", false, false), 200)
	s := NewService("1.5.0", srv.URL, true, srv.Client(), quiet)
	s.Check(context.Background())
	s.Status().Latest.Version = "x"
	if v := s.Status().Latest.Version; v != "1.6.0" {
		t.Fatalf("cache mutated: %q", v)
	}
}

func TestRequestHeaders(t *testing.T) {
	var ua, accept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua, accept = r.Header.Get("User-Agent"), r.Header.Get("Accept")
		fmt.Fprint(w, release("v1.5.0", false, false))
	}))
	defer srv.Close()
	NewService("1.5.0", srv.URL, true, srv.Client(), quiet).Check(context.Background())
	if ua != "Rezepte" || accept != "application/vnd.github+json" {
		t.Fatalf("headers: %q %q", ua, accept)
	}
}

func TestDisabledNeverRequests(t *testing.T) {
	srv, n := feed(t, release("v9.0.0", false, false), 200)
	for _, tc := range []struct {
		current string
		enabled bool
	}{{"1.5.0", false}, {"dev", true}} {
		s := NewService(tc.current, srv.URL, tc.enabled, srv.Client(), quiet)
		<-s.Start(context.Background())
		if st := s.Status(); st.State != "off" || st.Latest != nil || st.Current != tc.current {
			t.Errorf("%+v: status = %+v", tc, st)
		}
	}
	if n.Load() != 0 {
		t.Fatalf("requests = %d", n.Load())
	}
}

func TestStartChecksAndStopsOnCancel(t *testing.T) {
	srv, _ := feed(t, release("v1.6.0", false, false), 200)
	s := NewService("1.5.0", srv.URL, true, srv.Client(), quiet)
	ctx, cancel := context.WithCancel(context.Background())
	done := s.Start(ctx)
	deadline := time.Now().Add(time.Second)
	for s.Status().State != "available" && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if s.Status().State != "available" {
		t.Fatal("Start did not check")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("done did not close")
	}
}
