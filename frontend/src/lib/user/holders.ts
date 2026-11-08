import type { PersonEntry } from '#lib/api/users.js';
import { m } from '#lib/paraglide/messages.js';
import { getLocale } from '#lib/paraglide/runtime.js';
import { compareByName } from '#lib/settings/people-groups.js';
import type { UserColor } from './color.js';

/**
 * Who else wears `color`: everyone in it but the person the picker edits,
 * in the people list's reading order. Sharing a color is allowed; this only
 * tells the picker who you would look like.
 */
export function colorHolders(
	people: PersonEntry[],
	color: UserColor,
	selfId: string
): PersonEntry[] {
	return people.filter((p) => p.color === color && p.id !== selfId).sort(compareByName);
}

/** The first word of a name, the way a household calls someone. */
export const firstWord = (name: string) => name.trim().split(/\s+/)[0];

/**
 * Each holder by first name, the way a household talks. Two holders with the
 * same first name are both named in full, or the line would say "Anna, Anna".
 */
export function holderNames(holders: Pick<PersonEntry, 'displayName'>[]): string[] {
	const firsts = holders.map((p) => firstWord(p.displayName));
	return holders.map((p, i) =>
		firsts.indexOf(firsts[i]) === firsts.lastIndexOf(firsts[i]) ? firsts[i] : p.displayName.trim()
	);
}

/** Names joined the way the locale says "A, B and C". */
export const joinNames = (names: string[]) =>
	new Intl.ListFormat(getLocale(), { type: 'conjunction' }).format(names);

/** The visible sentence after the color's name: up to three names, then a count. */
export function holdersText(holders: PersonEntry[]): string {
	const names = holderNames(holders);
	if (names.length === 0) return m.color_holders_none();
	if (names.length <= 3) return m.color_holders_some({ names: joinNames(names) });
	return m.color_holders_more({ names: `${names[0]}, ${names[1]}`, count: names.length - 2 });
}

/** The same sentence for screen readers: every holder, by full display name. */
export function holdersFullText(holders: PersonEntry[]): string {
	if (holders.length === 0) return m.color_holders_none();
	return m.color_holders_some({ names: joinNames(holders.map((p) => p.displayName.trim())) });
}
