import type { PersonEntry, UserRole } from '#lib/api/users.js';
import { m } from '#lib/paraglide/messages.js';
import { rowPermissions, type RowPermissions } from './row-permissions';

/** Whether the viewer may act on a person at all, and so open their page. */
export function canOpenPerson(p: RowPermissions): boolean {
	return p.changeRole || p.manageAccount || p.editProfile || p.toggleSharing;
}

/**
 * What a typed `/settings/users/[username]` shows. The own account lives on
 * the profile page, and a person the viewer may not act on has nothing to
 * show beyond their row in the list, so both lead away instead of to an
 * empty page.
 */
export function personPageTarget(
	viewer: { id: string; role: UserRole },
	target: PersonEntry | undefined
):
	| { kind: 'show'; permissions: RowPermissions }
	| { kind: 'redirect'; to: '/settings' | '/settings/users' }
	| { kind: 'missing' } {
	if (!target) return { kind: 'missing' };
	if (target.id === viewer.id) return { kind: 'redirect', to: '/settings' };
	const permissions = rowPermissions(viewer.role, target, false);
	if (!canOpenPerson(permissions)) return { kind: 'redirect', to: '/settings/users' };
	return { kind: 'show', permissions };
}

/** "No recipes", "1 recipe", "3 recipes": the tile and the person page say it alike. */
export function recipesText(count: number): string {
	if (count === 0) return m.users_recipes_none();
	return count === 1 ? m.users_recipes_count_one() : m.users_recipes_count({ count });
}

/** The same for shopping lists. */
export function listsText(count: number): string {
	if (count === 0) return m.users_lists_none();
	return count === 1 ? m.users_lists_count_one() : m.users_lists_count({ count });
}
