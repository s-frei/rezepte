package notify

import (
	"context"
	"testing"
	"time"

	"github.com/s-frei/rezepte/service/internal/push"
	"github.com/s-frei/rezepte/service/internal/user"
)

// mark sets marker's tasty mark on recipeID.
func (f fanout) mark(t *testing.T, marker user.User, recipeID string, on bool) {
	t.Helper()
	if err := f.recipes.SetTasty(context.Background(), marker.ID, recipeID, on); err != nil {
		t.Fatal(err)
	}
}

func TestTastyNotifiesCreator(t *testing.T) {
	ctx := context.Background()
	f := newFanout(t)
	p := f.pusher()
	p.ch = make(chan pushed, 4)
	defer func(d time.Duration) { tastyDelay = d }(tastyDelay)
	tastyDelay = 0
	olga := f.person(t, "olga", user.RoleAdmin, true)
	mia := f.person(t, "mia", user.RoleUser, true)
	r := f.recipe(t, olga.ID)
	f.mark(t, mia, r.ID, true)
	f.svc.TastyMarked(ctx, mia.ID, r.ID)
	select {
	case got := <-p.ch:
		want := push.Message{Kind: "tasty", Title: "Lecker", Body: mia.DisplayName + " findet dein Rezept „" + r.Title + "“ lecker",
			URL: "/recipes/" + r.Slug, Tag: "tasty:" + r.ID + ":" + mia.ID}
		if got.to != olga.ID || got.m != want {
			t.Errorf("pushed %+v, want %+v to olga", got, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no push")
	}
	if len(f.mailer.tasties) != 1 {
		t.Fatalf("mailed %+v", f.mailer.tasties)
	}
	m := f.mailer.tasties[0]
	if m.To != olga.Email || m.MarkerName != mia.DisplayName || m.RecipeTitle != r.Title || m.RecipeURLPath != "/recipes/"+r.Slug ||
		m.Locale != "de" || m.Unsubscribe.KindURL == "" {
		t.Errorf("mail %+v", m)
	}
}

func TestTastyTakenBackSendsNothing(t *testing.T) {
	ctx := context.Background()
	f := newFanout(t)
	p := f.pusher()
	olga := f.person(t, "olga", user.RoleAdmin, true)
	mia := f.person(t, "mia", user.RoleUser, true)
	r := f.recipe(t, olga.ID)
	f.mark(t, mia, r.ID, true)
	f.mark(t, mia, r.ID, false)
	f.svc.sendTasty(ctx, mia.ID, r.ID)
	if len(p.sent) != 0 || len(f.mailer.tasties) != 0 {
		t.Errorf("pushed %+v, mailed %+v", p.sent, f.mailer.tasties)
	}
}

func TestTastyRespectsChannels(t *testing.T) {
	ctx := context.Background()
	f := newFanout(t)
	p := f.pusher()
	olga := f.person(t, "olga", user.RoleAdmin, true)
	jonas := f.person(t, "jonas", user.RoleUser, false) // no confirmed address: push only
	mia := f.person(t, "mia", user.RoleUser, true)
	mailOnly, pushOnly := f.recipe(t, olga.ID), f.recipe(t, jonas.ID)
	if _, err := f.svc.SetPreferences(ctx, olga, Prefs{Kinds: map[Kind]map[Channel]bool{KindTasty: {ChannelPush: false}}}); err != nil {
		t.Fatal(err)
	}
	for _, r := range []string{mailOnly.ID, pushOnly.ID} {
		f.mark(t, mia, r, true)
		f.svc.sendTasty(ctx, mia.ID, r)
	}
	if got := p.pushesTo(); len(p.sent) != 1 || len(got[jonas.ID]) != 1 {
		t.Errorf("pushed %+v, want jonas only", p.sent)
	}
	if len(f.mailer.tasties) != 1 || f.mailer.tasties[0].To != olga.Email {
		t.Errorf("mailed %+v, want olga only", f.mailer.tasties)
	}
	if _, err := f.svc.SetPreferences(ctx, olga, Prefs{Groups: map[Group]map[Channel]bool{GroupHousehold: {ChannelMail: false}}}); err != nil {
		t.Fatal(err)
	}
	f.svc.sendTasty(ctx, mia.ID, mailOnly.ID)
	if len(f.mailer.tasties) != 1 {
		t.Errorf("mailed %+v despite the household switch", f.mailer.tasties)
	}
}
