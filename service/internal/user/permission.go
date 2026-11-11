package user

// The rank rules live here and nowhere else. An admin manages plain users;
// anything touching an admin or the superadmin belongs to the superadmin;
// and the superadmin can be neither deleted nor demoted by anyone, themselves
// included. Both functions take roles rather than users so they stay pure and
// exhaustively testable, and so a caller cannot accidentally pass the target
// where the actor belongs and still compile into something plausible.

// GuardTarget reports whether actor may delete, re-role or reset the password
// of a user whose role is target (issuing a setup link is such a reset), and
// equally whether actor may hand out the role target: creating a user has no
// target row to read, so userapi checks the requested role itself. The
// superadmin check comes first: that it is refused holds for every caller,
// including the superadmin, so it is the more specific answer.
// It is also why the superadmin role is never assignable through the
// application; the bootstrap path is its only writer.
func GuardTarget(actor, target Role) error {
	if target.IsSuperadmin() {
		return ErrSuperadminProtected
	}
	if target.IsAdmin() && !actor.IsSuperadmin() {
		return ErrSuperadminRequired
	}
	return nil
}

// GuardProfileEdit reports whether actor may write the profile fields -
// display name and color - of another user. This is deliberately not
// GuardTarget: that one lets an admin act on a member, which is right for a
// role change or a password reset and wrong for a rename. Managing a member
// is administration; renaming them is not.
//
// Editing one's own profile never reaches here. That is the /auth/me route,
// which has no target to guard.
func GuardProfileEdit(actor Role) error {
	if !actor.IsSuperadmin() {
		return ErrSuperadminRequired
	}
	return nil
}
