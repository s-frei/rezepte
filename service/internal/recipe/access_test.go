package recipe_test

import (
	"context"
	"errors"
	"testing"

	"github.com/s-frei/rezepte/service/internal/recipe"
	"github.com/s-frei/rezepte/service/internal/settings"
	"github.com/s-frei/rezepte/service/internal/user"
)

// accessEnv is an author (member), another member, and the owner, on one
// database with one recipe written by the author.
type accessEnv struct {
	svc                  *recipe.Service
	settings             *settings.Service
	author, other, owner user.User
	rec                  recipe.Recipe
}

func newAccessEnv(t *testing.T, policy recipe.Policy) accessEnv {
	t.Helper()
	svc, _ := setup(t)
	conn := testConns[t]
	users := user.NewService(conn)
	ctx := context.Background()
	mk := func(name string, role user.Role) user.User {
		u, err := users.Create(ctx, user.CreateParams{Username: name, Password: "pw", Role: role})
		if err != nil {
			t.Fatal(err)
		}
		return u
	}
	author := mk("anna", user.RoleUser)
	other := mk("ben", user.RoleUser)
	owner := mk("olga", user.RoleSuperadmin)
	in := loadFixtures(t)[0]
	in.EditPolicy = policy
	rec, err := svc.Create(ctx, author.ID, in)
	if err != nil {
		t.Fatal(err)
	}
	return accessEnv{svc: svc, settings: settings.NewService(conn), author: author, other: other, owner: owner, rec: rec}
}

func TestUpdateLockedByOtherIsRefused(t *testing.T) {
	e := newAccessEnv(t, recipe.PolicyLocked)
	_, err := e.svc.Update(context.Background(), e.rec.ID, e.other, e.rec.Input)
	if !errors.Is(err, recipe.ErrEditForbidden) {
		t.Fatalf("err = %v, want ErrEditForbidden", err)
	}
}

func TestUpdateOpenByOtherLands(t *testing.T) {
	e := newAccessEnv(t, recipe.PolicyOpen)
	in := e.rec.Input
	in.Title = "Changed by Ben"
	got, err := e.svc.Update(context.Background(), e.rec.ID, e.other, in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Changed by Ben" || got.EditPolicy != recipe.PolicyOpen {
		t.Errorf("got title %q policy %q", got.Title, got.EditPolicy)
	}
}

func TestPolicyChangeByOtherIsRefused(t *testing.T) {
	e := newAccessEnv(t, recipe.PolicyOpen)
	in := e.rec.Input
	in.EditPolicy = recipe.PolicyLocked
	_, err := e.svc.Update(context.Background(), e.rec.ID, e.other, in)
	if !errors.Is(err, recipe.ErrEditForbidden) {
		t.Fatalf("err = %v, want ErrEditForbidden", err)
	}
}

func TestUpdateWithoutPolicyKeepsIt(t *testing.T) {
	e := newAccessEnv(t, recipe.PolicyOpen)
	in := e.rec.Input
	in.EditPolicy = ""
	got, err := e.svc.Update(context.Background(), e.rec.ID, e.other, in)
	if err != nil {
		t.Fatal(err)
	}
	if got.EditPolicy != recipe.PolicyOpen {
		t.Errorf("EditPolicy = %q, want open", got.EditPolicy)
	}
}

func TestAuthorAndOwnerMayChangePolicy(t *testing.T) {
	for _, who := range []string{"author", "owner"} {
		e := newAccessEnv(t, recipe.PolicyDefault)
		actor := map[string]user.User{"author": e.author, "owner": e.owner}[who]
		in := e.rec.Input
		in.EditPolicy = recipe.PolicyLocked
		got, err := e.svc.Update(context.Background(), e.rec.ID, actor, in)
		if err != nil {
			t.Fatalf("%s: %v", who, err)
		}
		if got.EditPolicy != recipe.PolicyLocked {
			t.Errorf("%s: EditPolicy = %q, want locked", who, got.EditPolicy)
		}
	}
}

func TestDeleteOpenByOtherIsRefused(t *testing.T) {
	e := newAccessEnv(t, recipe.PolicyOpen)
	err := e.svc.Delete(context.Background(), e.rec.ID, e.other)
	if !errors.Is(err, recipe.ErrDeleteForbidden) {
		t.Fatalf("err = %v, want ErrDeleteForbidden", err)
	}
	if err := e.svc.Delete(context.Background(), e.rec.ID, e.author); err != nil {
		t.Fatalf("author delete: %v", err)
	}
}

func TestDefaultPolicyFollowsGlobalLive(t *testing.T) {
	e := newAccessEnv(t, recipe.PolicyDefault)
	ctx := context.Background()
	if _, err := e.settings.SetRecipesLockedByDefault(ctx, e.owner, true); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Update(ctx, e.rec.ID, e.other, e.rec.Input); !errors.Is(err, recipe.ErrEditForbidden) {
		t.Fatalf("global on: err = %v, want ErrEditForbidden", err)
	}
	if _, err := e.settings.SetRecipesLockedByDefault(ctx, e.owner, false); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Update(ctx, e.rec.ID, e.other, e.rec.Input); err != nil {
		t.Fatalf("global off again: %v", err)
	}
}

func TestFillAccess(t *testing.T) {
	e := newAccessEnv(t, recipe.PolicyLocked)
	r := e.rec
	if err := e.svc.FillAccess(context.Background(), e.other, &r); err != nil {
		t.Fatal(err)
	}
	if !r.Locked || r.CanEdit || r.CanDelete || r.CanChangePolicy {
		t.Errorf("other on locked: %+v", [4]bool{r.Locked, r.CanEdit, r.CanDelete, r.CanChangePolicy})
	}
	if err := e.svc.FillAccess(context.Background(), e.author, &r); err != nil {
		t.Fatal(err)
	}
	if !r.Locked || !r.CanEdit || !r.CanDelete || !r.CanChangePolicy {
		t.Errorf("author on locked: %+v", [4]bool{r.Locked, r.CanEdit, r.CanDelete, r.CanChangePolicy})
	}
}
