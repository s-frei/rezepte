package user

// The rank rules live here and nowhere else. An admin manages plain users;
// anything touching an admin or the superadmin belongs to the superadmin;
// and the superadmin can be neither deleted nor demoted by anyone, themselves
// included. Both functions take roles rather than users so they stay pure and
// exhaustively testable, and so a caller cannot accidentally pass the target
// where the actor belongs and still compile into something plausible.

// guardTarget reports whether actor may delete, re-role or reset the password
// of a user whose role is target. The superadmin check comes first: that it is
// refused holds for every caller, including the superadmin, so it is the more
// specific answer.
func guardTarget(actor, target Role) error {
	if target.IsSuperadmin() {
		return ErrSuperadminProtected
	}
	if target.IsAdmin() && !actor.IsSuperadmin() {
		return ErrSuperadminRequired
	}
	return nil
}

// guardAssignRole reports whether actor may hand out role. The superadmin role
// is never assignable through the application; the bootstrap path is its only
// writer.
func guardAssignRole(actor, role Role) error {
	if role.IsSuperadmin() {
		return ErrSuperadminProtected
	}
	if role.IsAdmin() && !actor.IsSuperadmin() {
		return ErrSuperadminRequired
	}
	return nil
}

// CanAssignRole is guardAssignRole for callers outside this package. Creating
// a user has no target row to read, so userapi checks the requested role
// itself before calling Create.
func CanAssignRole(actor, role Role) error { return guardAssignRole(actor, role) }

// guardProfileEdit reports whether actor may write the profile fields -
// display name and color - of another user. This is deliberately not
// guardTarget: that one lets an admin act on a member, which is right for a
// role change or a password reset and wrong for a rename. Managing a member
// is administration; renaming them is not.
//
// Editing one's own profile never reaches here. That is the /auth/me route,
// which has no target to guard.
func guardProfileEdit(actor Role) error {
	if !actor.IsSuperadmin() {
		return ErrSuperadminRequired
	}
	return nil
}

// CanEditProfile is guardProfileEdit for callers outside this package.
func CanEditProfile(actor Role) error { return guardProfileEdit(actor) }
