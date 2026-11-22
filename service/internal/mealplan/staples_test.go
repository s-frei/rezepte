package mealplan_test

import (
	"context"
	"errors"
	"testing"

	"github.com/s-frei/rezepte/service/internal/mealplan"
)

func TestStapleKeyMergesPlurals(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	id := e.plan(t).ID
	if _, err := e.svc.SetStaple(ctx, e.a.ID, id, "Zwiebeln", true); err != nil {
		t.Fatal(err)
	}
	got, err := e.svc.SetStaple(ctx, e.a.ID, id, "zwiebel", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "Zwiebeln" {
		t.Fatalf("staples = %+v, want one Zwiebeln", got)
	}
	if got, err = e.svc.SetStaple(ctx, e.a.ID, id, "ZWIEBEL", false); err != nil || len(got) != 0 {
		t.Fatalf("after unset = %+v, %v", got, err)
	}
}

func TestStaplesArePerPlan(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	if _, err := e.svc.SetStaple(ctx, e.a.ID, e.plan(t).ID, "Salz", true); err != nil {
		t.Fatal(err)
	}
	bens, err := e.svc.Create(ctx, e.b.ID, "Bens Plan")
	if err != nil {
		t.Fatal(err)
	}
	got, err := e.svc.Staples(ctx, e.b.ID, bens.ID)
	if err != nil || len(got) != 0 {
		t.Fatalf("ben's plan staples = %+v, %v", got, err)
	}
}

func TestStaplesNeedAccess(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	id := e.plan(t).ID
	if _, err := e.svc.Staples(ctx, e.b.ID, id); !errors.Is(err, mealplan.ErrNotFound) {
		t.Fatalf("stranger reads staples: %v", err)
	}
	if _, err := e.svc.SetStaple(ctx, e.b.ID, id, "Salz", true); !errors.Is(err, mealplan.ErrNotFound) {
		t.Fatalf("stranger sets staple: %v", err)
	}
	// Invited is not in: the staples wait for the accept.
	if _, err := e.svc.Invite(ctx, e.a.ID, id, e.b.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.SetStaple(ctx, e.b.ID, id, "Salz", true); !errors.Is(err, mealplan.ErrNotFound) {
		t.Fatalf("invitee sets staple: %v", err)
	}
	if _, err := e.svc.Staples(ctx, e.b.ID, id); !errors.Is(err, mealplan.ErrNotFound) {
		t.Fatalf("invitee reads staples: %v", err)
	}
	if got, _ := e.svc.Staples(ctx, e.a.ID, id); len(got) != 0 {
		t.Fatalf("refused staple stored: %+v", got)
	}
}

// The owner's case: Ben is on Anna's list, which holds her plan's meal, but
// not in the plan. The list gives him nothing of the plan's staples.
func TestStaplesStayClosedToAListMemberOutsideThePlan(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	id := e.plan(t).ID
	_, listID := e.toList(t, e.a.ID, id, "", e.planned(t, e.a.ID, id, e.dal(t), 4))
	if _, err := e.lists.Invite(ctx, e.a.ID, listID, e.b.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.lists.Accept(ctx, e.b.ID, listID); err != nil {
		t.Fatal(err)
	}
	if l := e.list(t, e.b.ID, listID); len(l.Entries) != 1 || l.Entries[0].PlanID == nil || *l.Entries[0].PlanID != id {
		t.Fatalf("ben's list entries = %+v", l.Entries)
	}
	if _, err := e.svc.Staples(ctx, e.b.ID, id); !errors.Is(err, mealplan.ErrNotFound) {
		t.Fatalf("list member reads staples: %v", err)
	}
	if _, err := e.svc.SetStaple(ctx, e.b.ID, id, "Salz", true); !errors.Is(err, mealplan.ErrNotFound) {
		t.Fatalf("list member sets staple: %v", err)
	}
}

// Leaving or being removed ends the access with the membership.
func TestStaplesCloseOnLeavingAndRemoval(t *testing.T) {
	ctx := context.Background()
	e := setup(t)
	id := e.shared(t)
	if _, err := e.svc.SetStaple(ctx, e.b.ID, id, "Salz", true); err != nil {
		t.Fatalf("member sets staple: %v", err)
	}
	if err := e.svc.Leave(ctx, e.b.ID, id); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Staples(ctx, e.b.ID, id); !errors.Is(err, mealplan.ErrNotFound) {
		t.Fatalf("left member reads staples: %v", err)
	}
	other := e.shared(t)
	if _, err := e.svc.RemoveMember(ctx, e.a.ID, other, e.b.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.SetStaple(ctx, e.b.ID, other, "Pfeffer", true); !errors.Is(err, mealplan.ErrNotFound) {
		t.Fatalf("removed member sets staple: %v", err)
	}
	if _, err := e.svc.Staples(ctx, e.b.ID, other); !errors.Is(err, mealplan.ErrNotFound) {
		t.Fatalf("removed member reads staples: %v", err)
	}
}
