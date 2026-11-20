package mealplan_test

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// stream is an open event stream: the response and its lines as they arrive.
type stream struct {
	resp  *http.Response
	lines chan string
}

// openStream opens the plan's event stream over a real connection, since a
// ResponseRecorder neither flushes nor streams.
func (a api) openStream(t *testing.T, c *http.Cookie, planID string) stream {
	t.Helper()
	srv := httptest.NewServer(a.h)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() { cancel(); srv.Close() })
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+plans+"/"+planID+"/events", nil)
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

func TestStreamForbidsStrangers(t *testing.T) {
	a := newAPI(t)
	owner, stranger := a.login(t, "anna"), a.login(t, "ben")
	p := a.call(t, owner, http.MethodPost, plans, map[string]string{"name": "Familie"}, http.StatusCreated)
	for _, id := range []string{p.ID, "nope"} {
		if rec := a.do(http.MethodGet, plans+"/"+id+"/events", "", stranger); rec.Code != http.StatusNotFound {
			t.Fatalf("%s: status %d, want 404", id, rec.Code)
		}
	}
	if rec := a.do(http.MethodGet, plans+"/"+p.ID+"/events", "", nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("no session: status %d, want 401", rec.Code)
	}
}

func TestStreamEndsWhenAccessLost(t *testing.T) {
	a := newAPI(t)
	owner, member := a.login(t, "anna"), a.login(t, "ben")
	id := a.sharedPlan(t, owner, member)
	s := a.openStream(t, member, id)
	if got := s.resp.Header.Get("Content-Type"); !strings.HasPrefix(got, "text/event-stream") {
		t.Fatalf("Content-Type %q", got)
	}
	s.expect(t, "retry: 3000")
	p := a.call(t, owner, http.MethodPost, plans+"/"+id+"/entries"+week, map[string]any{"title": "Reste"}, http.StatusOK)
	s.expect(t, "event: changed")
	s.expect(t, fmt.Sprintf(`data: {"revision":%d}`, p.Revision))
	a.call(t, owner, http.MethodDelete, plans+"/"+id+"/members/"+a.user(t, "ben").ID, nil, http.StatusOK)
	s.expectEnd(t)
}
