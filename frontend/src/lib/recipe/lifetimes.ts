import { m } from '$lib/paraglide/messages';
import type { ShareLifetime } from '$lib/api/shares';

/** The five lifetimes a public link can have, longest last; `null` is permanent. */
const ALL: ShareLifetime[] = [1, 7, 30, 365, null];

/**
 * The lifetimes a member may pick when creating a public link, capped at the
 * instance's maximum (`publicShareMaxDays`). `null` (permanent) is itself
 * the longest lifetime there is, so it appears only when there is no
 * maximum at all - the create route rejects it with a 422 otherwise.
 */
export function lifetimeOptions(maxDays: number | null): ShareLifetime[] {
	if (maxDays === null) {
		return ALL;
	}
	return ALL.filter((days): days is 1 | 7 | 30 | 365 => days !== null && days <= maxDays);
}

/**
 * A lifetime as the stable text value a `Select` can hold - "permanent"
 * alongside "1", "7", "30", "365" - and back. Days round-trip through these
 * instead of a second parallel list, so the option order can never drift
 * from `lifetimeOptions`. Shared by every surface that lets someone pick a
 * lifetime: the share dialog and the owner's default/maximum controls.
 */
export const lifetimeKey = (days: ShareLifetime): string =>
	days === null ? 'permanent' : String(days);
export const lifetimeFromKey = (key: string): ShareLifetime =>
	key === 'permanent' ? null : (Number(key) as 1 | 7 | 30 | 365);

/**
 * The label for a lifetime, in the active language - one set of copy keys
 * for every surface that names one (the share dialog, the owner's default
 * and maximum controls), so a translation never has to be written twice.
 */
export function lifetimeLabel(days: ShareLifetime): string {
	switch (days) {
		case 1:
			return m.public_share_lifetime_1();
		case 7:
			return m.public_share_lifetime_7();
		case 30:
			return m.public_share_lifetime_30();
		case 365:
			return m.public_share_lifetime_365();
		default:
			return m.public_share_lifetime_permanent();
	}
}
