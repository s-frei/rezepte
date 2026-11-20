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
	if got, _ := e.svc.Staples(ctx, e.a.ID, id); len(got) != 0 {
		t.Fatalf("refused staple stored: %+v", got)
	}
}
