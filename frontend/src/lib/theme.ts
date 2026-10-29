/**
 * Theme handling shared by the store (`theme.svelte.ts`) and the tests.
 * The inline script in `app.html` re-implements reading the stored
 * theme and `applyTheme` in plain JS so the choice is applied before first paint;
 * keep the key and the attribute name in sync with it.
 */
export type Theme = 'system' | 'light' | 'dark';

export const THEME_STORAGE_KEY = 'rezepte-theme';

export function isTheme(value: unknown): value is Theme {
	return value === 'system' || value === 'light' || value === 'dark';
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

/**
 * Points the `theme-color` metas in `app.html` at the chosen theme. Each
 * carries `data-scheme`; `system` hands them back to prefers-color-scheme,
 * an explicit choice turns its own meta on and the other off. Android
 * paints the status and navigation bars of the installed app from it.
 */
export function applyThemeColor(
	metas: Iterable<Pick<Element, 'getAttribute' | 'setAttribute'>>,
	theme: Theme
): void {
	for (const meta of metas) {
		const scheme = meta.getAttribute('data-scheme');
		const media =
			theme === 'system'
				? `(prefers-color-scheme: ${scheme})`
				: scheme === theme
					? 'all'
					: 'not all';
		meta.setAttribute('media', media);
	}
}
