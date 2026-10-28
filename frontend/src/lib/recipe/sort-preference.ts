import { browser } from '$app/environment';
import { isSort, type ListQuery, type Sort } from './query';

/**
 * The overview's order is remembered per device, like the theme and the
 * servings: only the browser needs it, so it has no place on the account.
 * Only a choice made with the sort control is stored - a link that names an
 * order applies it without becoming the reader's own.
 */
export const SORT_STORAGE_KEY = 'rezepte-sort';

/**
 * The order the overview opens with: the address's own when it names one,
 * the remembered one otherwise. Pure, so the rule is testable without a
 * browser; `readSort` supplies `remembered`.
 */
export function applyRememberedSort(
	parsed: ListQuery,
	url: URL,
	remembered: Sort | null
): ListQuery {
	return url.searchParams.has('sort') || remembered === null
		? parsed
		: { ...parsed, sort: remembered };
}

/** The remembered order, or `null` for the default or nothing usable. */
export function readSort(): Sort | null {
	if (!browser) {
		return null;
	}
	try {
		const raw = window.localStorage.getItem(SORT_STORAGE_KEY);
		return raw !== null && raw !== 'updated' && isSort(raw) ? raw : null;
	} catch {
		// `window.localStorage` itself throws when site data is blocked.
		return null;
	}
}

/** Remembers a chosen order; the default is stored as no entry at all. */
export function writeSort(sort: Sort): void {
	if (!browser) {
		return;
	}
	try {
		if (sort === 'updated') {
			window.localStorage.removeItem(SORT_STORAGE_KEY);
		} else {
			window.localStorage.setItem(SORT_STORAGE_KEY, sort);
		}
	} catch {
		// Storage blocked or full: the order still applies to this page view.
	}
}
