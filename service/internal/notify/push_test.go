package notify

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/s-frei/rezepte/service/internal/push"
	"github.com/s-frei/rezepte/service/internal/user"
)

type pushed struct {
	to string
	m  push.Message
}

// fakePusher records every push; each one also lands on ch when set.
type fakePusher struct {
	mu   sync.Mutex
	sent []pushed
	ch   chan pushed
}

func (p *fakePusher) SendToUser(_ context.Context, userID string, m push.Message) {
	p.mu.Lock()
	p.sent = append(p.sent, pushed{userID, m})
	p.mu.Unlock()
	if p.ch != nil {
		p.ch <- pushed{userID, m}
	}
}

func (f fanout) pusher() *fakePusher {
	p := &fakePusher{}
	f.svc.SetPusher(p)
	return p
}

// pushesTo maps each recipient to their pushes.
func (p *fakePusher) pushesTo() map[string][]push.Message {
	p.mu.Lock()
	defer p.mu.Unlock()
	got := map[string][]push.Message{}
	for _, s := range p.sent {
		got[s.to] = append(got[s.to], s.m)
	}
	return got
}

func TestShortenRuneSafe(t *testing.T) {
	// Each prefix moves the cut to another rune of the pattern.
	for _, prefix := range []string{"", "x", "xy"} {
		long := prefix + strings.Repeat("ä👍🏽", 67) // 201 runes; the thumb and its skin tone are two
		got := shorten(long, 120)
		if n := utf8.RuneCountInString(got); n > 121 {
			t.Errorf("%q: %d runes", prefix, n)
		}
		if !utf8.ValidString(got) || !strings.HasSuffix(got, "…") {
			t.Errorf("%q: shortened to %q", prefix, got)
		}
		if strings.HasSuffix(got, "👍…") {
			t.Errorf("%q: split the thumb from its skin tone: %q", prefix, got)
		}
	}
	if got := shorten("Mehr  Kreuz\n\tkümmel ", 120); got != "Mehr Kreuz kümmel" {
		t.Errorf("short = %q", got)
	}
}

func TestCommentPushToFollowersNotAuthor(t *testing.T) {
	ctx := context.Background()
	f := newFanout(t)
	p := f.pusher()
	olga := f.person(t, "olga", user.RoleAdmin, true)
	mia := f.person(t, "mia", user.RoleUser, true)
	adam := f.person(t, "adam", user.RoleUser, true)
	r := f.recipe(t, olga.ID)
	f.comment(t, mia, r.ID)
	id := f.comment(t, adam, r.ID)
	if err := f.svc.pushComment(ctx, r.ID, id); err != nil {
		t.Fatal(err)
	}
	got := p.pushesTo()
	if len(got) != 2 || len(got[olga.ID]) != 1 || len(got[mia.ID]) != 1 || got[adam.ID] != nil {
		t.Fatalf("pushed %+v", p.sent)
	}
	m := got[mia.ID][0]
	body := adam.DisplayName + ": <b>Mehr Kreuzkümmel</b>"
	want := push.Message{
		Kind: "comment", Title: "Neuer Kommentar zu „" + r.Title + "“", Body: body, URL: "/recipes/" + r.Slug + "?tab=comments", Tag: "comment:" + r.ID,
		Collapse: &push.Collapse{Title: "Neue Kommentare zu „" + r.Title + "“", BodyMany: "{count} neue Kommentare"},
	}
	if m.Collapse == nil || *m.Collapse != *want.Collapse {
		t.Errorf("collapse %+v, want %+v", m.Collapse, want.Collapse)
	}
	m.Collapse, want.Collapse = nil, nil
	if m != want {
		t.Errorf("push %+v, want %+v", m, want)
	}
}

func TestCommentPushRespectsOptOut(t *testing.T) {
	ctx := context.Background()
	f := newFanout(t)
	p := f.pusher()
	olga := f.person(t, "olga", user.RoleAdmin, true)
	mia := f.person(t, "mia", user.RoleUser, true)
	r := f.recipe(t, olga.ID)
	if _, err := f.svc.SetPreferences(ctx, olga, Prefs{Kinds: map[Kind]map[Channel]bool{KindComment: {ChannelPush: false}}}); err != nil {
		t.Fatal(err)
	}
	id := f.comment(t, mia, r.ID)
	if err := f.svc.mailComment(ctx, r.ID, id); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.pushComment(ctx, r.ID, id); err != nil {
		t.Fatal(err)
	}
	if len(p.sent) != 0 {
		t.Errorf("pushed %+v", p.sent)
	}
	if len(f.mailer.comments) != 1 {
		t.Errorf("mailed %+v, want the mail still", f.mailer.comments)
	}
}

// Neither a missing confirmation nor mail switched off stops a push.
func TestCommentPushWithoutAddress(t *testing.T) {
	ctx := context.Background()
	f := newFanout(t)
	f.mailer.off = true
	p := f.pusher()
	p.ch = make(chan pushed, 1)
	olga := f.person(t, "olga", user.RoleAdmin, false)
	mia := f.person(t, "mia", user.RoleUser, true)
	r := f.recipe(t, olga.ID)
	f.svc.CommentAdded(ctx, r.ID, f.comment(t, mia, r.ID))
	select {
	case got := <-p.ch:
		if got.to != olga.ID {
			t.Errorf("pushed %+v", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no push")
	}
}

func TestDigestPush(t *testing.T) {
	f := newFanout(t)
	p := f.pusher()
	mila, jonas := f.household(t)
	f.added(t, mila, "Dal", at(0, 10, 0))
	f.added(t, jonas, "Pho", at(0, 11, 0))
	f.added(t, jonas, "Ramen", at(0, 12, 0))
	f.run(t, at(0, 17, 0))
	got := p.pushesTo()
	if len(p.sent) != 2 {
		t.Fatalf("pushed %+v", p.sent)
	}
	want := push.Message{Kind: "recipe", Title: "Neue Rezepte", URL: "/", Tag: "recipe-digest"}
	want.Body = "Dal von " + mila.DisplayName
	if m := got[jonas.ID]; len(m) != 1 || m[0] != want {
		t.Errorf("jonas %+v, want %+v", m, want)
	}
	want.Body = "2 neue Rezepte von " + jonas.DisplayName
	if m := got[mila.ID]; len(m) != 1 || m[0] != want {
		t.Errorf("mila %+v, want %+v", m, want)
	}
	if len(f.mailer.digests) != 2 {
		t.Errorf("mailed %d digests", len(f.mailer.digests))
	}
}

// With mail off the digest still runs for push, and an account without a
// confirmed address gets it.
func TestDigestPushWithMailOff(t *testing.T) {
	f := newFanout(t)
	p := f.pusher()
	f.mailer.off = true
	mila := f.person(t, "mila", user.RoleAdmin, false)
	jonas := f.person(t, "jonas", user.RoleUser, false)
	f.added(t, mila, "Dal", at(0, 10, 0))
	f.run(t, at(0, 17, 0))
	if got := p.pushesTo(); len(p.sent) != 1 || len(got[jonas.ID]) != 1 {
		t.Fatalf("pushed %+v", p.sent)
	}
}

func TestInviteAcceptedPush(t *testing.T) {
	ctx := context.Background()
	f := newFanout(t)
	p := f.pusher()
	p.ch = make(chan pushed, 4)
	admin := f.person(t, "adam", user.RoleAdmin, false) // no address: the push still goes
	optedOut := f.person(t, "otto", user.RoleAdmin, true)
	newcomer := f.person(t, "noah", user.RoleUser, false)
	if _, err := f.svc.SetPreferences(ctx, optedOut, Prefs{Kinds: map[Kind]map[Channel]bool{KindInviteAccepted: {ChannelPush: false}}}); err != nil {
		t.Fatal(err)
	}
	f.svc.InviteAccepted(ctx, optedOut.ID, newcomer)
	f.svc.InviteAccepted(ctx, admin.ID, newcomer)
	select {
	case got := <-p.ch:
		want := push.Message{Kind: "invite_accepted", Title: "Einladung angenommen", Body: newcomer.DisplayName + " meldet sich als noah an",
			URL: "/settings/users", Tag: "invite:" + newcomer.ID}
		if got.to != admin.ID || got.m != want {
			t.Errorf("pushed %+v, want %+v to adam", got, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no push")
	}
	if len(f.mailer.invites) != 1 || f.mailer.invites[0].To != optedOut.Email {
		t.Errorf("mailed %+v, want otto's mail only", f.mailer.invites)
	}
	select {
	case got := <-p.ch:
		t.Errorf("pushed %+v too", got)
	case <-time.After(50 * time.Millisecond):
	}
}

// The push is in the recipient's locale, not the author's.
func TestPushCopyGerman(t *testing.T) {
	ctx := context.Background()
	f := newFanout(t)
	p := f.pusher()
	olga := f.person(t, "olga", user.RoleAdmin, true) // "de", as every person here
	mia := f.person(t, "mia", user.RoleUser, true)
	if _, err := f.conn.ExecContext(ctx, "UPDATE users SET locale = 'en' WHERE id = ?", mia.ID); err != nil {
		t.Fatal(err)
	}
	r := f.recipe(t, olga.ID)
	f.comment(t, olga, r.ID)
	if err := f.svc.pushComment(ctx, r.ID, f.comment(t, mia, r.ID)); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.pushComment(ctx, r.ID, f.comment(t, olga, r.ID)); err != nil {
		t.Fatal(err)
	}
	got := p.pushesTo()
	if m := got[olga.ID]; len(m) != 1 || m[0].Collapse.BodyMany != "{count} neue Kommentare" {
		t.Errorf("olga %+v", m)
	}
	if m := got[mia.ID]; len(m) != 1 || m[0].Collapse.BodyMany != "{count} new comments" || m[0].Title != "New comment on “"+r.Title+"”" {
		t.Errorf("mia %+v", m)
	}
}
