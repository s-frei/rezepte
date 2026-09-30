import type { UserRole } from '$lib/api/users';
import { isAdminRole } from '$lib/roles';

/** What the viewer may do to one row of the people list. */
export type RowPermissions = {
	/** The role select instead of the role pill. */
	changeRole: boolean;
	/** Reset the password, issue or revoke a setup link, and delete the account. */
	manageAccount: boolean;
	/** Rename and recolor someone else. */
	editProfile: boolean;
	/** Withdraw or restore the right to share recipes publicly. */
	toggleSharing: boolean;
};

/**
 * The actions a row offers, mirroring the rank rules in
 * service/internal/user/permission.go. Absent rather than disabled: a control
 * that can never be used is noise, not information.
 *
 * Every account reads the list, so the viewer's rank comes first: a member
 * acts on no row. The owner's row offers nothing to anyone, because the API
 * refuses every action on it. Role changes belong to the owner alone; delete
 * and reset belong to any admin, but only over a plain member. A name and a
 * color are the owner's to hand out for every account but their own - the
 * profile page is where anyone renames themselves, and the own account is
 * never reset or deleted from here, because a reset ends the viewer's own
 * sessions. The right to share publicly follows reset and delete: the API
 * guards all three with the same rank check, so one request may carry them
 * together.
 */
export function rowPermissions(
	actorRole: UserRole,
	target: { role: UserRole },
	isSelf: boolean
): RowPermissions {
	if (!isAdminRole(actorRole) || target.role === 'superadmin') {
		return { changeRole: false, manageAccount: false, editProfile: false, toggleSharing: false };
	}
	const isOwner = actorRole === 'superadmin';
	const manageAccount = !isSelf && (target.role === 'user' || isOwner);
	return {
		changeRole: isOwner,
		manageAccount,
		editProfile: !isSelf && isOwner,
		toggleSharing: manageAccount
	};
}
