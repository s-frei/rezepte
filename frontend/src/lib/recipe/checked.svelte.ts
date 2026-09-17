import { SvelteSet } from 'svelte/reactivity';
import { belongsToRecipe, parseCheckedKeys, serializeCheckedKeys } from './checked';

const STORAGE_KEY = 'rezepte:checked-ingredients';

function readStorage(): string[] {
	if (typeof window === 'undefined') {
		return [];
	}
	return parseCheckedKeys(sessionStorage.getItem(STORAGE_KEY));
}

function writeStorage(): void {
	if (typeof window === 'undefined') {
		return;
	}
	sessionStorage.setItem(STORAGE_KEY, serializeCheckedKeys(keys));
}

// A single reactive set, shared across every open recipe in this tab - keys
// are built with `ingredientKey` from `./checked`
// (`${recipeId}:${groupIndex}:${ingredientIndex}`) so different recipes
// never collide. `SvelteSet` (rather than a plain `Set` inside `$state`)
// gives fine-grained reactivity for `.has()`/`.add()`/`.delete()`.
const keys = new SvelteSet<string>(readStorage());

/** Whether the ingredient row for `key` is checked in this session. */
export function isChecked(key: string): boolean {
	return keys.has(key);
}

/** Flips the checked state of the ingredient row for `key`. */
export function toggle(key: string): void {
	if (keys.has(key)) {
		keys.delete(key);
	} else {
		keys.add(key);
	}
	writeStorage();
}

/** Drops every checked key belonging to `recipeId` (e.g. once it's deleted). */
export function clear(recipeId: string): void {
	for (const key of [...keys]) {
		if (belongsToRecipe(key, recipeId)) {
			keys.delete(key);
		}
	}
	writeStorage();
}
