package mail

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/textproto"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/s-frei/rezepte/service/internal/db"
	"github.com/s-frei/rezepte/service/internal/db/sqlc"
)

// queueSMTP is a plain-text SMTP server for many connections. rcpt answers
// the n-th RCPT TO overall (from 0); nil accepts every recipient.
type queueSMTP struct {
	conns atomic.Int32
	rcpts atomic.Int32
	rcpt  func(n int) string
	mu    sync.Mutex
	got   []string // recipients whose DATA was accepted
}

func startQueueSMTP(t *testing.T, f *queueSMTP) Config {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			f.conns.Add(1)
			go f.serve(c)
		}
	}()
	return Config{Host: "127.0.0.1", Port: ln.Addr().(*net.TCPAddr).Port, Security: SecurityNone, From: "r@example.org", FromName: "Rezepte"}
}

func (f *queueSMTP) serve(c net.Conn) {
	defer func() { _ = c.Close() }()
	tp := textproto.NewConn(c)
	_ = tp.PrintfLine("220 fake")
	var to string
	for {
		line, err := tp.ReadLine()
		if err != nil {
			return
		}
		switch strings.ToUpper(strings.Fields(line + " x")[0]) {
		case "EHLO", "HELO":
			_ = tp.PrintfLine("250 fake")
		case "RCPT":
			reply := "250 ok"
			if n := int(f.rcpts.Add(1) - 1); f.rcpt != nil && f.rcpt(n) != "" {
				reply = f.rcpt(n)
			}
			to = strings.TrimSuffix(strings.TrimPrefix(line[strings.Index(line, ":")+1:], "<"), ">")
			_ = tp.PrintfLine("%s", reply)
		case "DATA":
			_ = tp.PrintfLine("354 go")
			if _, err := tp.ReadDotBytes(); err != nil {
				return
			}
			f.mu.Lock()
			f.got = append(f.got, to)
			f.mu.Unlock()
			_ = tp.PrintfLine("250 queued")
		case "QUIT":
			_ = tp.PrintfLine("221 bye")
			return
		default:
			_ = tp.PrintfLine("250 ok")
		}
	}
}

// queued returns a started Service sending through f's server, with short
// retries, stopped at the end of the test.
func queued(t *testing.T, f *queueSMTP) *Service {
	t.Helper()
	s := newSvc(t, Config{}, "https://r.example.org")
	if err := s.Save(context.Background(), owner, startQueueSMTP(t, f), false); err != nil {
		t.Fatal(err)
	}
	old := retryAfter
	retryAfter = []time.Duration{10 * time.Millisecond, 10 * time.Millisecond, 10 * time.Millisecond}
	t.Cleanup(func() { retryAfter = old })
	s.Start(context.Background())
	t.Cleanup(func() { s.Stop(time.Second) })
	return s
}

// settled waits until no mail in the history is queued and returns it.
func settled(t *testing.T, s *Service, n int) []Sent {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		box, err := s.History(context.Background(), 0, "")
		if err != nil {
			t.Fatal(err)
		}
		h := box.Items
		done := len(h) == n
		for _, m := range h {
			done = done && m.Status != "queued"
		}
		if done {
			return h
		}
		if time.Now().After(deadline) {
			t.Fatalf("history did not settle: %+v", h)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func reset(to string) Reset { return Reset{To: to, Username: "lea", Path: "/setup/x", Locale: "en"} }

// Mails queued together go out over one connection, and the call returns
// before the server has seen them.
func TestQueueSendsOverOneConnection(t *testing.T) {
	f := &queueSMTP{}
	s := queued(t, f)
	ctx := context.Background()
	for _, to := range []string{"a@example.org", "b@example.org", "c@example.org"} {
		if err := s.SendReset(ctx, reset(to)); err != nil {
			t.Fatal(err)
		}
	}
	h := settled(t, s, 3)
	for _, m := range h {
		if m.Status != "sent" || m.Attempts != 1 || m.Kind != "reset" || m.Subject == "" {
			t.Fatalf("history %+v", m)
		}
	}
	if n := f.conns.Load(); n > 2 {
		// The first mail may start its session before the others are queued.
		t.Fatalf("%d connections for 3 queued mails", n)
	}
}

// A refusal that may pass is tried again; the mail is sent once and its
// OnFail never runs.
func TestQueueRetriesATemporaryRefusal(t *testing.T) {
	f := &queueSMTP{rcpt: func(n int) string {
		if n == 0 {
			return "451 4.3.0 try again later"
		}
		return ""
	}}
	s := queued(t, f)
	var failed atomic.Bool
	if err := s.SendConfirm(context.Background(), Confirm{To: "a@example.org", Username: "lea", Path: "/confirm/x", Locale: "en",
		OnFail: func(context.Context, error) { failed.Store(true) }}); err != nil {
		t.Fatal(err)
	}
	h := settled(t, s, 1)
	if h[0].Status != "sent" || h[0].Attempts != 2 || failed.Load() || h[0].NextAttemptAt != nil {
		t.Fatalf("history %+v, OnFail ran: %v", h[0], failed.Load())
	}
	// The outbox still says what the first attempt ran into.
	if h[0].Error != "recipient: 451 4.3.0 try again later" {
		t.Fatalf("error = %q", h[0].Error)
	}
}

// A permanent refusal is final at once: no retry, OnFail runs with the
// delivery error, and the history keeps the server's reply.
func TestQueueGivesUpOnAPermanentRefusal(t *testing.T) {
	f := &queueSMTP{rcpt: func(int) string { return "550 5.1.1 no such user" }}
	s := queued(t, f)
	got := make(chan error, 1)
	if err := s.SendConfirm(context.Background(), Confirm{To: "a@example.org", Username: "lea", Path: "/confirm/x", Locale: "en",
		OnFail: func(_ context.Context, err error) { got <- err }}); err != nil {
		t.Fatal(err)
	}
	h := settled(t, s, 1)
	if h[0].Status != "failed" || h[0].Attempts != 1 || h[0].Error != "recipient: 550 5.1.1 no such user" {
		t.Fatalf("history %+v", h[0])
	}
	if err := <-got; !errors.Is(err, ErrSend) {
		t.Fatalf("OnFail err = %v", err)
	}
}

// A mail that keeps failing for a passing reason gives up after the last
// retry.
func TestQueueGivesUpAfterTheLastRetry(t *testing.T) {
	f := &queueSMTP{rcpt: func(int) string { return "421 4.7.0 busy" }}
	s := queued(t, f)
	if err := s.SendReset(context.Background(), reset("a@example.org")); err != nil {
		t.Fatal(err)
	}
	h := settled(t, s, 1)
	if h[0].Status != "failed" || h[0].Attempts != len(retryAfter)+1 {
		t.Fatalf("history %+v", h[0])
	}
}

// Stop gives up a mail waiting for its retry, and Start marks a mail an
// ended run left queued: the queue lives in memory.
func TestStopAndStartMarkUnsentMails(t *testing.T) {
	f := &queueSMTP{rcpt: func(int) string { return "421 busy" }}
	s := newSvc(t, Config{}, "https://r.example.org")
	ctx := context.Background()
	if err := s.Save(ctx, owner, startQueueSMTP(t, f), false); err != nil {
		t.Fatal(err)
	}
	s.Start(ctx)
	if err := s.SendReset(ctx, reset("a@example.org")); err != nil {
		t.Fatal(err)
	}
	for f.rcpts.Load() == 0 {
		time.Sleep(5 * time.Millisecond)
	}
	s.Stop(time.Second)
	h := settled(t, s, 1)
	if h[0].Status != "failed" || h[0].Error != errInterrupted.Error() {
		t.Fatalf("after Stop: %+v", h[0])
	}

	if _, err := s.q.InsertMailLog(ctx, sqlc.InsertMailLogParams{Kind: "reset", Recipient: "b@example.org", Subject: "s", CreatedAt: db.FormatTime(time.Now())}); err != nil {
		t.Fatal(err)
	}
	next := &Service{q: s.q, logger: s.logger, now: time.Now, publicURL: s.publicURL}
	next.Start(ctx)
	t.Cleanup(func() { next.Stop(time.Second) })
	for _, m := range settled(t, next, 2) {
		if m.Status != "failed" {
			t.Fatalf("after Start: %+v", m)
		}
	}
}

// Without Start a send happens inside the call, as for the command line,
// and still lands in the history.
func TestUnstartedSendsInsideTheCall(t *testing.T) {
	f := &queueSMTP{}
	s := newSvc(t, Config{}, "https://r.example.org")
	ctx := context.Background()
	if err := s.Save(ctx, owner, startQueueSMTP(t, f), false); err != nil {
		t.Fatal(err)
	}
	if err := s.SendReset(ctx, reset("a@example.org")); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	n := len(f.got)
	f.mu.Unlock()
	if box, _ := s.History(ctx, 0, ""); n != 1 || len(box.Items) != 1 || box.Items[0].Status != "sent" {
		h := box.Items
		t.Fatalf("sent %d, history %+v", n, h)
	}
}

func TestRetryable(t *testing.T) {
	for _, c := range []struct {
		err  error
		want bool
	}{
		{&textproto.Error{Code: 421, Msg: "busy"}, true},
		{&textproto.Error{Code: 535, Msg: "no"}, false},
		{ErrDisabled, false},
		{ErrNoSTARTTLS, false},
		{&net.OpError{Op: "read", Err: errors.New("i/o timeout")}, true},
		{errors.New("unencrypted connection"), false},
		{&net.DNSError{Err: "no such host", Name: "smtp.exmaple.org", IsNotFound: true}, false},
		{fmt.Errorf("message: %w: %w", errUnconfirmed, &net.OpError{Op: "read", Err: errors.New("reset")}), false},
	} {
		if got := retryable(c.err); got != c.want {
			t.Errorf("retryable(%v) = %v", c.err, got)
		}
	}
}

// An accepted setup mail calls OnSent, which is what marks the link as
// mailed; a refused one never does.
func TestQueueCallsOnSentOnlyForAnAcceptedMail(t *testing.T) {
	f := &queueSMTP{rcpt: func(n int) string {
		if n == 1 {
			return "550 no such user"
		}
		return ""
	}}
	s := queued(t, f)
	var sent sync.Map
	for _, to := range []string{"a@example.org", "b@example.org"} {
		if err := s.SendInvite(context.Background(), Invite{To: to, Name: "A", Inviter: "O", Path: "/setup/x", Locale: "en",
			OnSent: func(context.Context) { sent.Store(to, true) }}); err != nil {
			t.Fatal(err)
		}
	}
	settled(t, s, 2)
	_, a := sent.Load("a@example.org")
	_, b := sent.Load("b@example.org")
	if !a || b {
		t.Fatalf("OnSent for a: %v, for the refused b: %v", a, b)
	}
}

// A mail handed in once Stop began is failed at once, never left queued.
func TestEnqueueAfterStopFails(t *testing.T) {
	s := queued(t, &queueSMTP{})
	s.Stop(time.Second)
	got := make(chan error, 1)
	if err := s.SendConfirm(context.Background(), Confirm{To: "a@example.org", Username: "lea", Path: "/confirm/x", Locale: "en",
		OnFail: func(_ context.Context, err error) { got <- err }}); err != nil {
		t.Fatal(err)
	}
	if err := <-got; !errors.Is(err, errInterrupted) {
		t.Fatalf("OnFail err = %v", err)
	}
	if h := settled(t, s, 1); h[0].Status != "failed" {
		t.Fatalf("history %+v", h[0])
	}
}

// A mail waiting for its retry says when the next attempt is due.
func TestWaitingRetryShowsTheNextAttempt(t *testing.T) {
	f := &queueSMTP{rcpt: func(int) string { return "421 busy" }}
	s := newSvc(t, Config{}, "https://r.example.org")
	ctx := context.Background()
	if err := s.Save(ctx, owner, startQueueSMTP(t, f), false); err != nil {
		t.Fatal(err)
	}
	s.Start(ctx)
	t.Cleanup(func() { s.Stop(time.Second) })
	if err := s.SendReset(ctx, reset("a@example.org")); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		box, err := s.History(ctx, 0, "")
		if err != nil {
			t.Fatal(err)
		}
		if m := box.Items[0]; m.NextAttemptAt != nil {
			if m.Status != "queued" || m.Attempts != 1 || m.Error != "recipient: 421 busy" || box.Queued != 1 {
				t.Fatalf("waiting mail %+v, counts %+v", m, box)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("no next attempt recorded")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// The outbox pages by id, filters to problems and counts across pages.
func TestHistoryPagesAndFilters(t *testing.T) {
	s := newSvc(t, Config{}, "https://r.example.org")
	ctx := context.Background()
	now := db.FormatTime(time.Now())
	for i := range historyPage + 5 {
		id, err := s.q.InsertMailLog(ctx, sqlc.InsertMailLogParams{Kind: "reset", Recipient: "a@example.org", Subject: "s", CreatedAt: now})
		if err != nil {
			t.Fatal(err)
		}
		status := "sent"
		if i == 0 {
			status = "failed"
		}
		if err := s.q.FinishMailLog(ctx, sqlc.FinishMailLogParams{Status: status, Attempts: 1, FinishedAt: &now, ID: id}); err != nil {
			t.Fatal(err)
		}
	}
	first, err := s.History(ctx, 0, "")
	if err != nil || len(first.Items) != historyPage || !first.More || first.Total != historyPage+5 || first.Failed != 1 {
		t.Fatalf("first page: %d items, more %v, counts %d/%d, err %v", len(first.Items), first.More, first.Total, first.Failed, err)
	}
	next, err := s.History(ctx, first.Items[len(first.Items)-1].ID, "")
	if err != nil || len(next.Items) != 5 || next.More || next.Items[0].ID >= first.Items[len(first.Items)-1].ID {
		t.Fatalf("second page: %+v, err %v", next, err)
	}
	problems, err := s.History(ctx, 0, "failed")
	if err != nil || len(problems.Items) != 1 || problems.Items[0].Status != "failed" || problems.More {
		t.Fatalf("problems: %+v, err %v", problems, err)
	}
}

// Stop still sends what is queued when it is called.
func TestStopSendsWhatIsQueued(t *testing.T) {
	f := &queueSMTP{}
	s := queued(t, f)
	ctx := context.Background()
	for _, to := range []string{"a@example.org", "b@example.org", "c@example.org"} {
		if err := s.SendReset(ctx, reset(to)); err != nil {
			t.Fatal(err)
		}
	}
	s.Stop(5 * time.Second)
	f.mu.Lock()
	n := len(f.got)
	f.mu.Unlock()
	if n != 3 {
		t.Fatalf("%d of 3 queued mails sent before Stop returned", n)
	}
}
