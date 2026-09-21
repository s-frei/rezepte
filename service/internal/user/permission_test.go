package user

import (
	"errors"
	"testing"
)

func TestIsAdminIncludesTheSuperadmin(t *testing.T) {
	for _, tc := range []struct {
		role       Role
		admin      bool
		superadmin bool
	}{
		{RoleUser, false, false},
		{RoleAdmin, true, false},
		{RoleSuperadmin, true, true},
	} {
		if got := tc.role.IsAdmin(); got != tc.admin {
			t.Errorf("%s.IsAdmin() = %v, want %v", tc.role, got, tc.admin)
		}
		if got := tc.role.IsSuperadmin(); got != tc.superadmin {
			t.Errorf("%s.IsSuperadmin() = %v, want %v", tc.role, got, tc.superadmin)
		}
	}
}

func TestGuardTarget(t *testing.T) {
	for _, tc := range []struct {
		name          string
		actor, target Role
		want          error
	}{
		{"admin manages a member", RoleAdmin, RoleUser, nil},
		{"admin may not manage an admin", RoleAdmin, RoleAdmin, ErrSuperadminRequired},
		{"admin may not manage the superadmin", RoleAdmin, RoleSuperadmin, ErrSuperadminProtected},
		{"superadmin manages a member", RoleSuperadmin, RoleUser, nil},
		{"superadmin manages an admin", RoleSuperadmin, RoleAdmin, nil},
		{"nobody manages the superadmin", RoleSuperadmin, RoleSuperadmin, ErrSuperadminProtected},
		{"a member manages nobody", RoleUser, RoleUser, nil},
	} {
		if got := guardTarget(tc.actor, tc.target); !errors.Is(got, tc.want) {
			t.Errorf("%s: guardTarget(%s, %s) = %v, want %v", tc.name, tc.actor, tc.target, got, tc.want)
		}
	}
}

func TestGuardAssignRole(t *testing.T) {
	for _, tc := range []struct {
		name        string
		actor, role Role
		want        error
	}{
		{"admin hands out member", RoleAdmin, RoleUser, nil},
		{"admin may not hand out admin", RoleAdmin, RoleAdmin, ErrSuperadminRequired},
		{"superadmin hands out admin", RoleSuperadmin, RoleAdmin, nil},
		{"superadmin hands out member", RoleSuperadmin, RoleUser, nil},
		{"superadmin is never assignable", RoleSuperadmin, RoleSuperadmin, ErrSuperadminProtected},
		{"nor by an admin", RoleAdmin, RoleSuperadmin, ErrSuperadminProtected},
	} {
		if got := guardAssignRole(tc.actor, tc.role); !errors.Is(got, tc.want) {
			t.Errorf("%s: guardAssignRole(%s, %s) = %v, want %v", tc.name, tc.actor, tc.role, got, tc.want)
		}
		if got := CanAssignRole(tc.actor, tc.role); !errors.Is(got, tc.want) {
			t.Errorf("%s: CanAssignRole disagrees with guardAssignRole: %v", tc.name, got)
		}
	}
}
