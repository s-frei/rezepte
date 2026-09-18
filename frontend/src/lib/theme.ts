/**
 * Theme handling shared by the store (`theme.svelte.ts`) and the tests.
 * The inline script in `app.html` re-implements `readStoredTheme` +
 * `applyTheme` in plain JS so the choice is applied before first paint;
 * keep the key and the attribute name in sync with it.
 */
export type Theme = 'system' | 'light' | 'dark';

export const THEME_STORAGE_KEY = 'rezepte-theme';

export function isTheme(value: unknown): value is Theme {
	return value === 'system' || value === 'light' || value === 'dark';
}

/** Reads the stored choice; anything missing, unknown or unreadable counts as `system`. */
export function readStoredTheme(storage: Pick<Storage, 'getItem'> | null | undefined): Theme {
	try {
		const raw = storage?.getItem(THEME_STORAGE_KEY);
		return isTheme(raw) ? raw : 'system';
	} catch {
		return 'system';
	}
}

/**
 * Mirrors a theme onto `<html>`: `light`/`dark` set `data-theme`, `system`
 * removes it so the CSS falls back to `prefers-color-scheme` (see app.css).
 */
export function applyTheme(
	root: Pick<HTMLElement, 'setAttribute' | 'removeAttribute'>,
	theme: Theme
): void {
	if (theme === 'system') {
		root.removeAttribute('data-theme');
	} else {
		root.setAttribute('data-theme', theme);
	}
}
