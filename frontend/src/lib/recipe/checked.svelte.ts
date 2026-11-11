import { SvelteSet } from 'svelte/reactivity';
import { belongsToRecipe, loadCheckedKeys, saveCheckedKeys } from './checked';

// A single reactive set, shared across every open recipe in this tab - keys
// are built with `ingredientKey` from `./checked`
// (`${recipeId}:${groupIndex}:${ingredientIndex}`) so different recipes
// never collide. `SvelteSet` (rather than a plain `Set` inside `$state`)
// gives fine-grained reactivity for `.has()`/`.add()`/`.delete()`.
const keys = new SvelteSet<string>(loadCheckedKeys());

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
	saveCheckedKeys(keys);
}

/** Drops every checked key belonging to `recipeId` (e.g. once it's deleted). */
export function clear(recipeId: string): void {
	for (const key of [...keys]) {
		if (belongsToRecipe(key, recipeId)) {
			keys.delete(key);
		}
	}
	saveCheckedKeys(keys);
}
