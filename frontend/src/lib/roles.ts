import { m } from '$lib/paraglide/messages';
import type { UserRole } from '$lib/api/users';

/**
 * The superadmin is an admin, so a bare comparison against 'admin' excludes
 * the instance owner from every admin surface. This mirrors Role.IsAdmin in
 * service/internal/user/service.go and is the only place the frontend asks.
 */
export function isAdminRole(role: UserRole | undefined): boolean {
	return role === 'admin' || role === 'superadmin';
}

/**
 * The label for a role in the active language, for every surface that shows
 * one. An unknown or not yet loaded role reads as a plain member, which is
 * the least any account can be.
 */
export function roleLabel(role: UserRole | undefined): string {
	if (role === 'superadmin') return m.users_role_owner();
	if (role === 'admin') return m.users_role_admin();
	return m.users_role_member();
}

/**
 * The rank of a role, owner first: the order the member list sorts by when it
 * sorts by role, and the same descending order the permission table uses.
 * An unknown role ranks as a plain member, matching `roleLabel`.
 */
export function roleRank(role: UserRole | undefined): number {
	if (role === 'superadmin') return 0;
	if (role === 'admin') return 1;
	return 2;
}
