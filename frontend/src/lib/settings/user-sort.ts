import type { UserAccount } from '$lib/api/users';
import { getLocale } from '$lib/paraglide/runtime';
import { roleRank } from '$lib/roles';

export type UserSortColumn = 'name' | 'role';
export type SortDirection = 'asc' | 'desc';
export type UserSort = { column: UserSortColumn; direction: SortDirection };

export const DEFAULT_USER_SORT: UserSort = { column: 'name', direction: 'asc' };

// The list's order is the client's to decide. The API sorts with SQLite's
// `COLLATE NOCASE`, which folds ASCII only and files "Änna" after "Zoe", so a
// name the browser would put second arrives last. One comparator here, for the
// list and for a row that was just added, keeps both orders the same.
//
// The locale comes from Paraglide rather than a literal, so a second locale
// sorts by its own rules without anyone having to find this line.
let collator: Intl.Collator | undefined;
let collatorLocale: string | undefined;

function nameCollator(): Intl.Collator {
	const locale = getLocale();
	if (!collator || collatorLocale !== locale) {
		collator = new Intl.Collator(locale, { numeric: true });
		collatorLocale = locale;
	}
	return collator;
}

/** Ordering of last resort: identical under every locale and every option. */
function byCodePoint(a: string, b: string): number {
	if (a === b) return 0;
	return a < b ? -1 : 1;
}

/**
 * Compares two usernames the way the reader's locale reads them, and never
 * calls two different names equal: a tie would leave their order to however
 * the two rows happened to arrive, which is the bug this module exists for.
 * `numeric: true` makes "user2" precede "user10" and, on its own, would call
 * "user2" and "user02" equal; the collator's own strength does the same to
 * "Anna" and "Änna" in a locale that ignores accents.
 */
export function compareUsernames(a: string, b: string): number {
	return nameCollator().compare(a, b) || byCodePoint(a, b);
}

/**
 * Sorts a copy; ties in the role column fall back to the display name, then
 * the username, then the id.
 *
 * The row's large primary line is the display name, so the list sorts on it
 * rather than on the login name - a household whose display names differ
 * from their usernames would otherwise read as unsorted. Display names are
 * deliberately not unique, so the username is still needed as a tiebreaker
 * to keep the comparator total: without it, two people sharing a display
 * name would leave their relative order to however the two rows happened to
 * arrive, which is the bug `compareUsernames` above exists for in the first
 * place.
 */
export function sortUsers(users: UserAccount[], sort: UserSort): UserAccount[] {
	// Resolved once per sort rather than once per comparison.
	const compare = nameCollator().compare;
	const factor = sort.direction === 'desc' ? -1 : 1;
	return [...users].sort((a, b) => {
		if (sort.column === 'role') {
			const byRank = roleRank(a.role) - roleRank(b.role);
			if (byRank !== 0) return byRank * factor;
		}
		const byDisplayName =
			compare(a.displayName, b.displayName) || byCodePoint(a.displayName, b.displayName);
		if (byDisplayName !== 0) return byDisplayName * factor;
		const byUsername = compare(a.username, b.username) || byCodePoint(a.username, b.username);
		return (byUsername || byCodePoint(a.id, b.id)) * factor;
	});
}

/** Clicking the active column reverses it; any other column starts ascending. */
export function nextSort(current: UserSort, column: UserSortColumn): UserSort {
	if (current.column !== column) return { column, direction: 'asc' };
	return { column, direction: current.direction === 'asc' ? 'desc' : 'asc' };
}
