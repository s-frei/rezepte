import type { ReadonlyURL } from '$app/state';
import { load, save } from '#lib/storage.js';
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
	url: ReadonlyURL,
	remembered: Sort | null
): ListQuery {
	return url.searchParams.has('sort') || remembered === null
		? parsed
		: { ...parsed, sort: remembered };
}

/** The remembered order, or `null` for the default or nothing usable. */
export function readSort(): Sort | null {
	const raw = load(SORT_STORAGE_KEY);
	return raw !== null && raw !== 'updated' && isSort(raw) ? raw : null;
}

/** Remembers a chosen order; the default is stored as no entry at all. */
export function writeSort(sort: Sort): void {
	save(SORT_STORAGE_KEY, sort === 'updated' ? null : sort);
}
