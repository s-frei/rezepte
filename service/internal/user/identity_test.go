package user_test

import (
	"context"
	"errors"
	"testing"

	"github.com/s-frei/rezepte/service/internal/user"
)

const issuer = "https://id.example"

func TestLinkIdentity(t *testing.T) {
	ctx := context.Background()
	_, svc, sam, kim := seedTwo(t)

	if err := svc.LinkIdentity(ctx, sam.ID, issuer, "sub-sam"); err != nil {
		t.Fatalf("link: %v", err)
	}
	if got, err := svc.ByIdentity(ctx, issuer, "sub-sam"); err != nil || got.ID != sam.ID {
		t.Fatalf("ByIdentity = %+v, %v; want sam", got, err)
	}
	if err := svc.LinkIdentity(ctx, sam.ID, issuer, "sub-sam"); err != nil {
		t.Fatalf("relinking the same identity: %v, want nil", err)
	}
	if err := svc.LinkIdentity(ctx, kim.ID, issuer, "sub-sam"); !errors.Is(err, user.ErrIdentityTaken) {
		t.Fatalf("same identity to kim: %v, want ErrIdentityTaken", err)
	}
	if err := svc.LinkIdentity(ctx, sam.ID, issuer, "sub-other"); !errors.Is(err, user.ErrIdentityTaken) {
		t.Fatalf("second identity at one issuer: %v, want ErrIdentityTaken", err)
	}
	if _, err := svc.ByIdentity(ctx, "https://other.example", "sub-sam"); !errors.Is(err, user.ErrNotFound) {
		t.Fatalf("other issuer: %v, want ErrNotFound", err)
	}
	if _, err := svc.IdentityLinkedAt(ctx, sam.ID, issuer); err != nil {
		t.Fatalf("IdentityLinkedAt: %v", err)
	}
	if _, err := svc.IdentityLinkedAt(ctx, kim.ID, issuer); !errors.Is(err, user.ErrNotFound) {
		t.Fatalf("IdentityLinkedAt of kim: %v, want ErrNotFound", err)
	}
	if err := svc.LinkIdentity(ctx, kim.ID, "https://other.example", "sub-kim"); err != nil {
		t.Fatal(err)
	}
	linked, err := svc.LinkedUserIDs(ctx, issuer)
	if err != nil || !linked[sam.ID] || linked[kim.ID] {
		t.Fatalf("LinkedUserIDs = %v, %v; want only sam", linked, err)
	}
}

func TestUnlinkIdentity(t *testing.T) {
	ctx := context.Background()
	_, svc, sam, _ := seedTwo(t)
	anna, err := svc.Create(ctx, user.CreateParams{Username: "anna", Role: user.RoleUser})
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range []user.User{sam, anna} {
		if err := svc.LinkIdentity(ctx, u.ID, issuer, "sub-"+u.Username); err != nil {
			t.Fatal(err)
		}
	}
	if err := svc.UnlinkIdentity(ctx, anna.ID, issuer); !errors.Is(err, user.ErrLastCredential) {
		t.Fatalf("unlink without password: %v, want ErrLastCredential", err)
	}
	if err := svc.UnlinkIdentity(ctx, sam.ID, issuer); err != nil {
		t.Fatalf("unlink: %v", err)
	}
	if err := svc.UnlinkIdentity(ctx, sam.ID, issuer); !errors.Is(err, user.ErrNotFound) {
		t.Fatalf("unlink twice: %v, want ErrNotFound", err)
	}
	if _, err := svc.ByIdentity(ctx, issuer, "sub-sam"); !errors.Is(err, user.ErrNotFound) {
		t.Fatalf("after unlink: %v, want ErrNotFound", err)
	}
}

func TestDeletedUserTakesTheIdentity(t *testing.T) {
	ctx := context.Background()
	_, svc, sam, kim := seedTwo(t)
	if err := svc.LinkIdentity(ctx, kim.ID, issuer, "sub-kim"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(ctx, sam, kim.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ByIdentity(ctx, issuer, "sub-kim"); !errors.Is(err, user.ErrNotFound) {
		t.Fatalf("after delete: %v, want ErrNotFound", err)
	}
}
