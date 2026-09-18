/**
 * Storage rules for the servings a user picked on a recipe. Split from
 * `servings.svelte.ts` (the rune store) so they can be unit tested without
 * a browser, like `theme.ts` next to `theme.svelte.ts`.
 */

/** The editor's bounds (`form.ts` validates 1-99); the stepper never leaves them. */
export const SERVINGS_MIN = 1;
export const SERVINGS_MAX = 99;

/** One `localStorage` entry per recipe: `rezepte-servings:<recipeId>` → "6". */
export const SERVINGS_STORAGE_PREFIX = 'rezepte-servings:';

export function servingsKey(recipeId: string): string {
	return `${SERVINGS_STORAGE_PREFIX}${recipeId}`;
}

/** Whole servings within [1, 99]; `NaN` counts as the minimum. */
export function clampServings(value: number): number {
	if (Number.isNaN(value)) {
		return SERVINGS_MIN;
	}
	return Math.min(SERVINGS_MAX, Math.max(SERVINGS_MIN, Math.round(value)));
}

/** A stored value is only trusted when it is a whole number inside the range. */
export function parseServings(raw: string | null): number | null {
	if (raw === null || !/^\d{1,2}$/.test(raw)) {
		return null;
	}
	const value = Number(raw);
	return value >= SERVINGS_MIN && value <= SERVINGS_MAX ? value : null;
}
