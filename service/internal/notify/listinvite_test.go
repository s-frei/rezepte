package notify

import (
	"context"
	"testing"
	"time"

	"github.com/s-frei/rezepte/service/internal/push"
	"github.com/s-frei/rezepte/service/internal/shopping"
	"github.com/s-frei/rezepte/service/internal/user"
)

// list creates a shopping list of owner and returns its ID.
func (f fanout) list(t *testing.T, owner user.User, name string) string {
	t.Helper()
	l, err := shopping.NewService(f.conn).Create(context.Background(), owner.ID, name)
	if err != nil {
		t.Fatal(err)
	}
	return l.ID
}

func TestListInviteNotifiesInvitee(t *testing.T) {
	ctx := context.Background()
	f := newFanout(t)
	p := f.pusher()
	p.ch = make(chan pushed, 4)
	olga := f.person(t, "olga", user.RoleAdmin, true)
	mia := f.person(t, "mia", user.RoleUser, true)
	id := f.list(t, olga, "Grillfest")
	f.svc.ListInvited(ctx, id, olga.ID, mia.ID)
	select {
	case got := <-p.ch:
		f.face(t, &got.m, olga)
		want := push.Message{Kind: "list_invite", Title: "Einkaufsliste", Body: olga.DisplayName + " lädt dich zu „Grillfest“ ein",
			URL: "/lists", Tag: "list-invite:" + id}
		if got.to != mia.ID || got.m != want {
			t.Errorf("pushed %+v, want %+v to mia", got, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no push")
	}
	if len(f.mailer.listInvites) != 1 {
		t.Fatalf("mailed %+v", f.mailer.listInvites)
	}
	if m := f.mailer.listInvites[0]; m.To != mia.Email || m.ListName != "Grillfest" || m.InviterName != olga.DisplayName ||
		m.Locale != "de" || m.Unsubscribe.KindURL == "" {
		t.Errorf("mail %+v", m)
	}
}

func TestListInviteRespectsChannels(t *testing.T) {
	ctx := context.Background()
	f := newFanout(t)
	p := f.pusher()
	olga := f.person(t, "olga", user.RoleAdmin, true)
	mia := f.person(t, "mia", user.RoleUser, true)
	id := f.list(t, olga, "Grillfest")
	if _, err := f.svc.SetPreferences(ctx, mia, Prefs{Kinds: map[Kind]map[Channel]bool{KindListInvite: {ChannelMail: false}}}); err != nil {
		t.Fatal(err)
	}
	f.svc.sendListInvite(ctx, id, olga.ID, mia.ID)
	if len(p.sent) != 1 || len(f.mailer.listInvites) != 0 {
		t.Errorf("pushed %+v, mailed %+v, want push only", p.sent, f.mailer.listInvites)
	}
	if _, err := f.svc.SetPreferences(ctx, mia, Prefs{
		Kinds:  map[Kind]map[Channel]bool{KindListInvite: {ChannelMail: true}},
		Groups: map[Group]map[Channel]bool{GroupHousehold: {ChannelPush: false}},
	}); err != nil {
		t.Fatal(err)
	}
	f.svc.sendListInvite(ctx, id, olga.ID, mia.ID)
	if len(p.sent) != 1 || len(f.mailer.listInvites) != 1 {
		t.Errorf("pushed %+v, mailed %+v, want one mail more", p.sent, f.mailer.listInvites)
	}
}

func TestListInviteSkipsGoneAccounts(t *testing.T) {
	ctx := context.Background()
	f := newFanout(t)
	p := f.pusher()
	olga := f.person(t, "olga", user.RoleAdmin, true)
	mia := f.person(t, "mia", user.RoleUser, true)
	jonas := f.person(t, "jonas", user.RoleUser, true)
	id := f.list(t, olga, "Grillfest")
	if err := f.users.Delete(ctx, olga, jonas.ID); err != nil {
		t.Fatal(err)
	}
	f.svc.sendListInvite(ctx, id, olga.ID, jonas.ID) // invitee gone
	if err := f.users.Delete(ctx, olga, mia.ID); err != nil {
		t.Fatal(err)
	}
	f.svc.sendListInvite(ctx, id, mia.ID, olga.ID) // inviter gone
	if len(p.sent) != 0 || len(f.mailer.listInvites) != 0 {
		t.Errorf("pushed %+v, mailed %+v", p.sent, f.mailer.listInvites)
	}
}
