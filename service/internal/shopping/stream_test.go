package shopping_test

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/s-frei/rezepte/service/internal/shopping"
)

// stream is an open event stream: the response and its lines as they arrive.
type stream struct {
	resp  *http.Response
	lines chan string
}

// openStream opens the list's event stream over a real connection, since a
// ResponseRecorder neither flushes nor streams.
func (a api) openStream(t *testing.T, c *http.Cookie, listID string) stream {
	t.Helper()
	srv := httptest.NewServer(a.h)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() { cancel(); srv.Close() })
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+base+"/"+listID+"/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.AddCookie(c)
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	s := stream{resp: resp, lines: make(chan string, 64)}
	go func() {
		defer close(s.lines)
		defer resp.Body.Close()
		sc := bufio.NewScanner(resp.Body)
		for sc.Scan() {
			s.lines <- sc.Text()
		}
	}()
	return s
}

// expect waits up to a second for line; anything else in between is skipped.
func (s stream) expect(t *testing.T, line string) {
	t.Helper()
	timeout := time.After(time.Second)
	for {
		select {
		case got, ok := <-s.lines:
			if !ok {
				t.Fatalf("stream ended before %q", line)
			}
			if got == line {
				return
			}
		case <-timeout:
			t.Fatalf("no %q within a second", line)
		}
	}
}

// expectEnd waits up to a second for the server to end the stream.
func (s stream) expectEnd(t *testing.T) {
	t.Helper()
	timeout := time.After(time.Second)
	for {
		select {
		case _, ok := <-s.lines:
			if !ok {
				return
			}
		case <-timeout:
			t.Fatal("stream still open after a second")
		}
	}
}

// sharedList makes a list of anna's that ben has accepted.
func (a api) sharedList(t *testing.T, owner, member *http.Cookie) string {
	t.Helper()
	ben, err := a.users.ByUsername(context.Background(), "ben")
	if err != nil {
		t.Fatal(err)
	}
	l := a.call(t, owner, http.MethodPost, base, map[string]string{"name": "Woche"}, http.StatusCreated)
	a.call(t, owner, http.MethodPost, base+"/"+l.ID+"/invitations", map[string]string{"userId": ben.ID}, http.StatusOK)
	a.call(t, member, http.MethodPost, base+"/"+l.ID+"/membership", nil, http.StatusOK)
	return l.ID
}

func TestStreamFlushesEachEvent(t *testing.T) {
	a := newAPI(t)
	owner, member := a.login(t, "anna"), a.login(t, "ben")
	id := a.sharedList(t, owner, member)
	s := a.openStream(t, owner, id)
	s.expect(t, "retry: 3000")
	l := a.call(t, member, http.MethodPost, base+"/"+id+"/items", map[string]string{"text": "Milch"}, http.StatusOK)
	s.expect(t, "event: changed")
	// The event names the revision the change's own answer carried, so that page skips the refetch.
	s.expect(t, fmt.Sprintf(`data: {"revision":%d}`, l.Revision))
	a.call(t, member, http.MethodPost, base+"/"+id+"/items", map[string]string{"text": "Brot"}, http.StatusOK)
	s.expect(t, "event: changed")
}

func TestStreamForbidsStrangers(t *testing.T) {
	a := newAPI(t)
	owner, stranger := a.login(t, "anna"), a.login(t, "ben")
	l := a.call(t, owner, http.MethodPost, base, map[string]string{"name": "Woche"}, http.StatusCreated)
	for _, id := range []string{l.ID, "nope"} {
		if rec := a.do(http.MethodGet, base+"/"+id+"/events", "", stranger); rec.Code != http.StatusNotFound {
			t.Fatalf("%s: status %d, want 404", id, rec.Code)
		}
	}
	if rec := a.do(http.MethodGet, base+"/"+l.ID+"/events", "", nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("no session: status %d, want 401", rec.Code)
	}
}

func TestStreamEndsWhenAccessLost(t *testing.T) {
	a := newAPI(t)
	owner, member := a.login(t, "anna"), a.login(t, "ben")
	id := a.sharedList(t, owner, member)
	s := a.openStream(t, member, id)
	s.expect(t, "retry: 3000")
	ben, err := a.users.ByUsername(context.Background(), "ben")
	if err != nil {
		t.Fatal(err)
	}
	a.call(t, owner, http.MethodDelete, base+"/"+id+"/members/"+ben.ID, nil, http.StatusOK)
	s.expectEnd(t)
}

func TestStreamHeaders(t *testing.T) {
	old := *shopping.PingEvery
	*shopping.PingEvery = 10 * time.Millisecond
	t.Cleanup(func() { *shopping.PingEvery = old })
	a := newAPI(t)
	c := a.login(t, "anna")
	l := a.call(t, c, http.MethodPost, base, map[string]string{"name": "Woche"}, http.StatusCreated)
	s := a.openStream(t, c, l.ID)
	if s.resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(s.resp.Body)
		t.Fatalf("status %d: %s", s.resp.StatusCode, body)
	}
	for k, want := range map[string]string{"Content-Type": "text/event-stream", "Cache-Control": "no-cache", "X-Accel-Buffering": "no"} {
		if got := s.resp.Header.Get(k); !strings.HasPrefix(got, want) {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
	s.expect(t, ": ping")
}
