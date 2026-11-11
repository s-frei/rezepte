/**
 * Pure key-building and (de)serialization logic for the recipe detail
 * page's "checked ingredient" state. Split out from `checked.svelte.ts` so
 * it can be unit tested without a browser or `sessionStorage`.
 */

const STORAGE_KEY = 'rezepte:checked-ingredients';

/**
 * Builds the storage key for one ingredient row, unique across all recipes
 * so a single `sessionStorage` entry can hold checked state for every
 * recipe a user has open in the same tab.
 */
export function ingredientKey(
	recipeId: string,
	groupIndex: number,
	ingredientIndex: number
): string {
	return `${recipeId}:${groupIndex}:${ingredientIndex}`;
}

/**
 * True when `key` (as built by `ingredientKey`) belongs to `recipeId`.
 *
 * Matches on `${recipeId}:` rather than a plain `startsWith(recipeId)` so
 * one recipe id being a prefix of another's (e.g. `abc` vs. `abcd`) can't
 * cause a false match.
 */
export function belongsToRecipe(key: string, recipeId: string): boolean {
	return key.startsWith(`${recipeId}:`);
}

/**
 * Parses the `sessionStorage` value into a list of checked-ingredient keys.
 * Anything that isn't a JSON array of strings - missing, corrupt, or from
 * an incompatible earlier format - is treated as empty rather than thrown.
 */
export function parseCheckedKeys(raw: string | null): string[] {
	if (!raw) {
		return [];
	}
	try {
		const parsed: unknown = JSON.parse(raw);
		return Array.isArray(parsed)
			? parsed.filter((value): value is string => typeof value === 'string')
			: [];
	} catch {
		return [];
	}
}

/** Serializes a set of checked-ingredient keys for `sessionStorage`. */
export function serializeCheckedKeys(keys: Iterable<string>): string {
	return JSON.stringify(Array.from(keys));
}

/**
 * Reads the checked keys from `sessionStorage`. A browser that blocks site
 * data throws on the property itself, and outside the browser there is none;
 * either way nothing is checked, and the page still renders.
 */
export function loadCheckedKeys(): string[] {
	try {
		return parseCheckedKeys(sessionStorage.getItem(STORAGE_KEY));
	} catch {
		return [];
	}
}

/** Writes the checked keys; without storage the ticks last only this page. */
export function saveCheckedKeys(keys: Iterable<string>): void {
	try {
		sessionStorage.setItem(STORAGE_KEY, serializeCheckedKeys(keys));
	} catch {
		// No storage: the ticks stay on screen until the page goes.
	}
}
