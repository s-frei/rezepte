import type { PersonEntry, UserRole } from '$lib/api/users';
import { getLocale } from '$lib/paraglide/runtime';
import { roleRank } from '$lib/roles';

/** One role's share of the people list, the people already in reading order. */
export type PeopleGroup<T extends PersonEntry> = { role: UserRole; people: T[] };

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

/** Display name first, what the row shows; the username and the id keep the order total. */
function compareByName(a: PersonEntry, b: PersonEntry): number {
	const compare = nameCollator().compare;
	return (
		compare(a.displayName, b.displayName) ||
		byCodePoint(a.displayName, b.displayName) ||
		compareUsernames(a.username, b.username) ||
		byCodePoint(a.id, b.id)
	);
}

/**
 * The people list's order: one group per role that somebody holds, owner
 * first, then admins, then members (`roleRank`), each group ordered by name
 * the way the reader's locale reads it. The group heading names the role, so
 * the rows need not; a role nobody holds gets no empty heading. Returns new
 * arrays and leaves `people` alone.
 */
export function groupPeople<T extends PersonEntry>(people: T[]): PeopleGroup<T>[] {
	const byRole = new Map<UserRole, T[]>();
	for (const person of [...people].sort(compareByName)) {
		const group = byRole.get(person.role);
		if (group) group.push(person);
		else byRole.set(person.role, [person]);
	}
	return [...byRole.entries()]
		.sort(([a], [b]) => roleRank(a) - roleRank(b))
		.map(([role, members]) => ({ role, people: members }));
}
