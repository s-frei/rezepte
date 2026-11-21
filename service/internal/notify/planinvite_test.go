package notify

import (
	"context"
	"testing"
	"time"

	"github.com/s-frei/rezepte/service/internal/mealplan"
	"github.com/s-frei/rezepte/service/internal/push"
	"github.com/s-frei/rezepte/service/internal/shopping"
	"github.com/s-frei/rezepte/service/internal/user"
)

// plan creates a meal plan of owner and returns its ID.
func (f fanout) plan(t *testing.T, owner user.User, name string) string {
	t.Helper()
	l, err := mealplan.NewService(f.conn, shopping.NewService(f.conn)).Create(context.Background(), owner.ID, name)
	if err != nil {
		t.Fatal(err)
	}
	return l.ID
}

func TestPlanInviteNotifiesInvitee(t *testing.T) {
	ctx := context.Background()
	f := newFanout(t)
	p := f.pusher()
	p.ch = make(chan pushed, 4)
	olga := f.person(t, "olga", user.RoleAdmin, true)
	mia := f.person(t, "mia", user.RoleUser, true)
	id := f.plan(t, olga, "Familienwoche")
	f.svc.PlanInvited(ctx, id, olga.ID, mia.ID)
	select {
	case got := <-p.ch:
		f.face(t, &got.m, olga)
		want := push.Message{Kind: "plan_invite", Title: "Wochenplan", Body: olga.DisplayName + " lädt dich zu „Familienwoche“ ein",
			URL: "/plans/" + id, Tag: "plan-invite:" + id}
		if got.to != mia.ID || got.m != want {
			t.Errorf("pushed %+v, want %+v to mia", got, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no push")
	}
	if len(f.mailer.shareInvites) != 1 {
		t.Fatalf("mailed %+v", f.mailer.shareInvites)
	}
	if m := f.mailer.shareInvites[0]; m.To != mia.Email || m.Name != "Familienwoche" || m.Path != "/plans/"+id || m.InviterName != olga.DisplayName ||
		m.Locale != "de" || m.Unsubscribe.KindURL == "" {
		t.Errorf("mail %+v", m)
	}
}

func TestPlanInviteRespectsChannels(t *testing.T) {
	ctx := context.Background()
	f := newFanout(t)
	p := f.pusher()
	olga := f.person(t, "olga", user.RoleAdmin, true)
	mia := f.person(t, "mia", user.RoleUser, true)
	id := f.plan(t, olga, "Familienwoche")
	if _, err := f.svc.SetPreferences(ctx, mia, Prefs{Kinds: map[Kind]map[Channel]bool{KindPlanInvite: {ChannelMail: false}}}); err != nil {
		t.Fatal(err)
	}
	f.svc.sendPlanInvite(ctx, id, olga.ID, mia.ID)
	if len(p.sent) != 1 || len(f.mailer.shareInvites) != 0 {
		t.Errorf("pushed %+v, mailed %+v, want push only", p.sent, f.mailer.shareInvites)
	}
	if _, err := f.svc.SetPreferences(ctx, mia, Prefs{
		Kinds:  map[Kind]map[Channel]bool{KindPlanInvite: {ChannelMail: true}},
		Groups: map[Group]map[Channel]bool{GroupHousehold: {ChannelPush: false}},
	}); err != nil {
		t.Fatal(err)
	}
	f.svc.sendPlanInvite(ctx, id, olga.ID, mia.ID)
	if len(p.sent) != 1 || len(f.mailer.shareInvites) != 1 {
		t.Errorf("pushed %+v, mailed %+v, want one mail more", p.sent, f.mailer.shareInvites)
	}
}

func TestPlanInviteSkipsGoneAccounts(t *testing.T) {
	ctx := context.Background()
	f := newFanout(t)
	p := f.pusher()
	olga := f.person(t, "olga", user.RoleAdmin, true)
	mia := f.person(t, "mia", user.RoleUser, true)
	jonas := f.person(t, "jonas", user.RoleUser, true)
	id := f.plan(t, olga, "Familienwoche")
	if err := f.users.Delete(ctx, olga, jonas.ID); err != nil {
		t.Fatal(err)
	}
	f.svc.sendPlanInvite(ctx, id, olga.ID, jonas.ID) // invitee gone
	if err := f.users.Delete(ctx, olga, mia.ID); err != nil {
		t.Fatal(err)
	}
	f.svc.sendPlanInvite(ctx, id, mia.ID, olga.ID) // inviter gone
	if len(p.sent) != 0 || len(f.mailer.shareInvites) != 0 {
		t.Errorf("pushed %+v, mailed %+v", p.sent, f.mailer.shareInvites)
	}
}

func TestPlanInviteSkipsPersonalPlan(t *testing.T) {
	ctx := context.Background()
	f := newFanout(t)
	p := f.pusher()
	olga := f.person(t, "olga", user.RoleAdmin, true)
	mia := f.person(t, "mia", user.RoleUser, true)
	plans, _, err := mealplan.NewService(f.conn, shopping.NewService(f.conn)).Plans(ctx, olga.ID)
	if err != nil {
		t.Fatal(err)
	}
	f.svc.sendPlanInvite(ctx, plans[0].ID, olga.ID, mia.ID)
	if len(p.sent) != 0 || len(f.mailer.shareInvites) != 0 {
		t.Errorf("pushed %+v, mailed %+v for a personal plan", p.sent, f.mailer.shareInvites)
	}
}
