package notify_test

import (
	"context"
	"errors"
	"maps"
	"testing"

	"github.com/s-frei/rezepte/service/internal/notify"
)

var both = map[notify.Channel]bool{notify.ChannelMail: true, notify.ChannelPush: true}

func off(ch notify.Channel) map[notify.Channel]bool { return map[notify.Channel]bool{ch: false} }

func equalPrefs[K comparable](got, want map[K]map[notify.Channel]bool) bool {
	return maps.EqualFunc(got, want, maps.Equal)
}

func TestPreferencesDefaultAllOn(t *testing.T) {
	f := newFixture(t)
	got, err := f.svc.Preferences(context.Background(), f.users["mia"])
	if err != nil {
		t.Fatal(err)
	}
	if want := map[notify.Kind]map[notify.Channel]bool{notify.KindComment: both, notify.KindRecipe: both, notify.KindTasty: both}; !equalPrefs(got.Kinds, want) {
		t.Errorf("kinds %v, want %v", got.Kinds, want)
	}
	if want := map[notify.Group]map[notify.Channel]bool{notify.GroupHousehold: both}; !equalPrefs(got.Groups, want) {
		t.Errorf("groups %v, want %v", got.Groups, want)
	}
}

func TestMailFailingOnlyOwnerPush(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	olga, err := f.svc.Preferences(ctx, f.users["olga"])
	if err != nil {
		t.Fatal(err)
	}
	if got := olga.Kinds[notify.KindMailFailing]; !maps.Equal(got, map[notify.Channel]bool{notify.ChannelPush: true}) {
		t.Errorf("owner mail_failing %v", got)
	}
	if got := olga.Groups[notify.GroupAdmin]; !maps.Equal(got, both) {
		t.Errorf("owner admin group %v", got)
	}
	adam, err := f.svc.Preferences(ctx, f.users["adam"])
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := adam.Kinds[notify.KindMailFailing]; ok {
		t.Errorf("admin has mail_failing %v", got)
	}
	if !maps.Equal(adam.Kinds[notify.KindInviteAccepted], both) {
		t.Errorf("admin invite_accepted %v", adam.Kinds[notify.KindInviteAccepted])
	}
	_, err = f.svc.SetPreferences(ctx, f.users["olga"], notify.Prefs{Kinds: map[notify.Kind]map[notify.Channel]bool{notify.KindMailFailing: off(notify.ChannelMail)}})
	if !errors.Is(err, notify.ErrUnknownKind) {
		t.Errorf("mail_failing/mail: %v", err)
	}
}

func TestGroupMasterKeepsKindChoices(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	mia := f.users["mia"]
	wants := func(k notify.Kind, ch notify.Channel) bool {
		t.Helper()
		ok, err := f.svc.Wants(ctx, mia.ID, k, ch)
		if err != nil {
			t.Fatal(err)
		}
		return ok
	}
	set := func(p notify.Prefs) notify.Prefs {
		t.Helper()
		got, err := f.svc.SetPreferences(ctx, mia, p)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	set(notify.Prefs{Kinds: map[notify.Kind]map[notify.Channel]bool{notify.KindRecipe: off(notify.ChannelPush)}})
	got := set(notify.Prefs{Groups: map[notify.Group]map[notify.Channel]bool{notify.GroupHousehold: off(notify.ChannelPush)}})
	if wants(notify.KindRecipe, notify.ChannelPush) || wants(notify.KindComment, notify.ChannelPush) || !wants(notify.KindComment, notify.ChannelMail) {
		t.Error("household push off: wrong deliveries")
	}
	// The kind's own choice shows through the group switch.
	if !got.Kinds[notify.KindComment][notify.ChannelPush] || got.Groups[notify.GroupHousehold][notify.ChannelPush] {
		t.Errorf("household push off: %+v", got)
	}
	set(notify.Prefs{Groups: map[notify.Group]map[notify.Channel]bool{notify.GroupHousehold: {notify.ChannelPush: true}}})
	if !wants(notify.KindComment, notify.ChannelPush) || wants(notify.KindRecipe, notify.ChannelPush) {
		t.Error("household push on again did not restore the kinds' own choices")
	}
}

// A group row silences a kind that has no row of its own, as a kind added
// after the group was switched off has none.
func TestGroupOffSilencesLaterKinds(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	mia := f.users["mia"]
	if _, err := f.svc.SetPreferences(ctx, mia, notify.Prefs{Groups: map[notify.Group]map[notify.Channel]bool{notify.GroupHousehold: off(notify.ChannelMail)}}); err != nil {
		t.Fatal(err)
	}
	if ok, err := f.svc.Wants(ctx, mia.ID, notify.KindTasty, notify.ChannelMail); err != nil || ok {
		t.Errorf("tasty mail with household mail off: %v %v", ok, err)
	}
	if ok, err := f.svc.Wants(ctx, mia.ID, notify.KindTasty, notify.ChannelPush); err != nil || !ok {
		t.Errorf("tasty push: %v %v", ok, err)
	}
}

func TestSetPreferencesAllOrNothing(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	mia := f.users["mia"]
	for name, p := range map[string]notify.Prefs{
		"kind of role": {Kinds: map[notify.Kind]map[notify.Channel]bool{notify.KindComment: off(notify.ChannelMail), notify.KindInviteAccepted: off(notify.ChannelMail)}},
		"group of role": {
			Kinds:  map[notify.Kind]map[notify.Channel]bool{notify.KindComment: off(notify.ChannelMail)},
			Groups: map[notify.Group]map[notify.Channel]bool{notify.GroupHousehold: off(notify.ChannelPush), notify.GroupAdmin: off(notify.ChannelPush)},
		},
		"unknown channel": {Kinds: map[notify.Kind]map[notify.Channel]bool{notify.KindComment: {notify.ChannelMail: false, "sms": false}}},
		"unknown kind":    {Kinds: map[notify.Kind]map[notify.Channel]bool{notify.KindComment: off(notify.ChannelMail), "digest": off(notify.ChannelMail)}},
	} {
		if _, err := f.svc.SetPreferences(ctx, mia, p); !errors.Is(err, notify.ErrUnknownKind) {
			t.Errorf("%s: %v", name, err)
		}
	}
	got, err := f.svc.Preferences(ctx, mia)
	if err != nil {
		t.Fatal(err)
	}
	for k, chs := range got.Kinds {
		if !maps.Equal(chs, both) {
			t.Errorf("%s changed by a refused write: %v", k, chs)
		}
	}
	if !maps.Equal(got.Groups[notify.GroupHousehold], both) {
		t.Errorf("household changed by a refused write: %v", got.Groups)
	}
}
