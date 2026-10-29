import { browser } from '$app/environment';

/**
 * Per-device choices in `localStorage`. Reaching for `window.localStorage`
 * itself throws in a browser that blocks site data, so the guard sits around
 * the property access, not just around `getItem`; a blocked or full storage
 * leaves the choice standing for this page view only.
 */
export function load(key: string): string | null {
	if (!browser) {
		return null;
	}
	try {
		return window.localStorage.getItem(key);
	} catch {
		return null;
	}
}

/** Stores `value` under `key`; `null` removes the entry. */
export function save(key: string, value: string | null): void {
	if (!browser) {
		return;
	}
	try {
		if (value === null) {
			window.localStorage.removeItem(key);
		} else {
			window.localStorage.setItem(key, value);
		}
	} catch {
		// Storage blocked or full: the choice still applies to this page view.
	}
}
