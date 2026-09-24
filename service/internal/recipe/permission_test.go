package recipe_test

import (
	"testing"

	"github.com/s-frei/rezepte/service/internal/recipe"
	"github.com/s-frei/rezepte/service/internal/user"
)

func TestLocked(t *testing.T) {
	cases := []struct {
		global bool
		policy recipe.Policy
		want   bool
	}{
		{false, recipe.PolicyDefault, false},
		{true, recipe.PolicyDefault, true},
		{false, recipe.PolicyOpen, false},
		{true, recipe.PolicyOpen, false},
		{false, recipe.PolicyLocked, true},
		{true, recipe.PolicyLocked, true},
	}
	for _, c := range cases {
		got := recipe.Locked(recipe.Guarded{CreatedBy: "a", Policy: c.policy}, c.global)
		if got != c.want {
			t.Errorf("Locked(global=%v, %s) = %v, want %v", c.global, c.policy, got, c.want)
		}
	}
}

func TestCallerTable(t *testing.T) {
	author := user.User{ID: "a", Role: user.RoleUser}
	admin := user.User{ID: "b", Role: user.RoleAdmin}
	owner := user.User{ID: "c", Role: user.RoleSuperadmin}
	other := user.User{ID: "d", Role: user.RoleUser}
	open := recipe.Guarded{CreatedBy: "a", Policy: recipe.PolicyOpen}
	locked := recipe.Guarded{CreatedBy: "a", Policy: recipe.PolicyLocked}

	cases := []struct {
		name                    string
		actor                   user.User
		g                       recipe.Guarded
		edit, del, changePolicy bool
	}{
		{"author, locked", author, locked, true, true, true},
		{"admin, locked", admin, locked, true, true, true},
		{"owner, locked", owner, locked, true, true, true},
		{"other, open", other, open, true, false, false},
		{"other, locked", other, locked, false, false, false},
	}
	for _, c := range cases {
		if got := recipe.CanEdit(c.actor, c.g, false); got != c.edit {
			t.Errorf("%s: CanEdit = %v, want %v", c.name, got, c.edit)
		}
		if got := recipe.CanDelete(c.actor, c.g); got != c.del {
			t.Errorf("%s: CanDelete = %v, want %v", c.name, got, c.del)
		}
		if got := recipe.CanChangePolicy(c.actor, c.g); got != c.changePolicy {
			t.Errorf("%s: CanChangePolicy = %v, want %v", c.name, got, c.changePolicy)
		}
	}
}

func TestCanEditFollowsGlobalOnDefault(t *testing.T) {
	other := user.User{ID: "d", Role: user.RoleUser}
	g := recipe.Guarded{CreatedBy: "a", Policy: recipe.PolicyDefault}
	if !recipe.CanEdit(other, g, false) {
		t.Error("default, global off: other cannot edit, want can")
	}
	if recipe.CanEdit(other, g, true) {
		t.Error("default, global on: other can edit, want cannot")
	}
}
