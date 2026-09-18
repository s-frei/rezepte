import { browser } from '$app/environment';
import { applyTheme, readStoredTheme, THEME_STORAGE_KEY, type Theme } from './theme';

function initialTheme(): Theme {
	if (!browser) {
		return 'system';
	}
	try {
		return readStoredTheme(window.localStorage);
	} catch {
		// Reaching for `window.localStorage` throws outright in a browser that
		// blocks site data, so the guard sits around the property access and
		// not just around `getItem` - this runs while the module evaluates.
		return 'system';
	}
}

/** The user's theme choice. Read `theme.value`; change it with `setTheme`. */
export const theme = $state<{ value: Theme }>({ value: initialTheme() });

export function setTheme(next: Theme): void {
	theme.value = next;
	if (!browser) {
		return;
	}
	applyTheme(document.documentElement, next);
	try {
		if (next === 'system') {
			window.localStorage.removeItem(THEME_STORAGE_KEY);
		} else {
			window.localStorage.setItem(THEME_STORAGE_KEY, next);
		}
	} catch {
		// Storage blocked or full: the choice still applies to this page view.
	}
}
