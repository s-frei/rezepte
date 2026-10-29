package notify

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/s-frei/rezepte/service/internal/push"
	"github.com/s-frei/rezepte/service/internal/user"
)

// alerting is a fanout with an owner, a pusher that hands every push to ch
// and a clock the test moves.
type alerting struct {
	fanout
	owner user.User
	p     *fakePusher
	at    time.Time
}

func newAlerting(t *testing.T) *alerting {
	t.Helper()
	a := &alerting{fanout: newFanout(t), at: time.Date(2026, 10, 29, 8, 0, 0, 0, time.UTC)}
	a.owner = a.person(t, "olga", user.RoleSuperadmin, true)
	a.use(a.svc)
	return a
}

// use points svc at a's pusher and clock.
func (a *alerting) use(svc *Service) {
	a.svc = svc
	a.p = &fakePusher{ch: make(chan pushed, 8)}
	svc.SetPusher(a.p)
	svc.now = func() time.Time { return a.at }
}

// fail reports a failure and returns its push, or nil when none came.
func (a *alerting) fail(t *testing.T, reason string) *push.Message {
	t.Helper()
	a.svc.MailFailed(context.Background(), reason)
	select {
	case got := <-a.p.ch:
		if got.to != a.owner.ID {
			t.Fatalf("pushed to %s, want the owner", got.to)
		}
		return &got.m
	case <-time.After(200 * time.Millisecond):
		return nil
	}
}

func TestMailAlertFirstFailureFreshInstance(t *testing.T) {
	a := newAlerting(t)
	m := a.fail(t, "auth")
	want := push.Message{Kind: "mail_failing", Title: "Mails gehen nicht raus", Body: "Der Mailserver hat die Anmeldung abgelehnt.", URL: "/settings/mail", Tag: "mail-failing"}
	if m == nil || *m != want {
		t.Fatalf("push %+v, want %+v", m, want)
	}
}

func TestMailAlertOncePerDay(t *testing.T) {
	a := newAlerting(t)
	if a.fail(t, "connect") == nil {
		t.Fatal("first failure: no push")
	}
	a.at = a.at.Add(23 * time.Hour)
	if m := a.fail(t, "connect"); m != nil {
		t.Fatalf("same day: pushed %+v", m)
	}
	a.at = a.at.Add(2 * time.Hour)
	if a.fail(t, "connect") == nil {
		t.Fatal("25 h later: no push")
	}
}

// TestMailAlertReset: after a working send or new settings, the next
// failure alerts at once.
func TestMailAlertReset(t *testing.T) {
	a := newAlerting(t)
	if a.fail(t, "tls") == nil {
		t.Fatal("first failure: no push")
	}
	a.svc.MailReset(context.Background())
	a.at = a.at.Add(time.Hour)
	if a.fail(t, "tls") == nil {
		t.Fatal("failure after a reset: no push")
	}
}

func TestMailAlertSurvivesRestart(t *testing.T) {
	a := newAlerting(t)
	if a.fail(t, "auth") == nil {
		t.Fatal("first failure: no push")
	}
	a.use(NewService(a.conn, bytes.Repeat([]byte{7}, 32), a.users, nil, a.images, a.mailer, "https://r.example.org"))
	if m := a.fail(t, "auth"); m != nil {
		t.Fatalf("after a restart: pushed %+v", m)
	}
}

func TestMailAlertRespectsOptOut(t *testing.T) {
	a := newAlerting(t)
	if _, err := a.svc.SetPreferences(context.Background(), a.owner, Prefs{Groups: map[Group]map[Channel]bool{GroupAdmin: {ChannelPush: false}}}); err != nil {
		t.Fatal(err)
	}
	if m := a.fail(t, "auth"); m != nil {
		t.Fatalf("opted out: pushed %+v", m)
	}
}
