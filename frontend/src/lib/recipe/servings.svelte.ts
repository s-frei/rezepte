import { browser } from '$app/environment';
import { clampServings, parseServings, servingsKey } from './servings';

function readStored(recipeId: string): number | null {
	if (!browser) {
		return null;
	}
	try {
		return parseServings(window.localStorage.getItem(servingsKey(recipeId)));
	} catch {
		// `window.localStorage` itself throws when site data is blocked.
		return null;
	}
}

function writeStored(recipeId: string, value: number | null): void {
	if (!browser) {
		return;
	}
	try {
		if (value === null) {
			window.localStorage.removeItem(servingsKey(recipeId));
		} else {
			window.localStorage.setItem(servingsKey(recipeId), String(value));
		}
	} catch {
		// Storage blocked or full: the choice still applies to this page view.
	}
}

/**
 * The servings a user wants for one recipe. `base` is the recipe's stored
 * `servings` (the baseline every quantity is written for); `value` starts
 * from `localStorage` when a choice was made earlier, otherwise from `base`.
 * Only a choice that differs from the baseline is persisted, so a recipe
 * that was never scaled leaves no key behind.
 */
export class Servings {
	readonly recipeId: string;
	readonly base: number;
	value = $state(1);

	constructor(recipeId: string, base: number) {
		this.recipeId = recipeId;
		this.base = base;
		this.value = readStored(recipeId) ?? base;
	}

	/** True while the list shows scaled quantities. */
	get scaled(): boolean {
		return this.value !== this.base;
	}

	set(next: number): void {
		const clamped = clampServings(next);
		this.value = clamped;
		writeStored(this.recipeId, clamped === this.base ? null : clamped);
	}

	reset(): void {
		this.set(this.base);
	}
}

/** One store per (recipe, page); the detail page and cook mode never share an instance, only the storage key. */
export function createServings(recipeId: string, base: number): Servings {
	return new Servings(recipeId, base);
}

/** Forgets the choice for a recipe (called when the recipe is deleted). */
export function clearServings(recipeId: string): void {
	writeStored(recipeId, null);
}
