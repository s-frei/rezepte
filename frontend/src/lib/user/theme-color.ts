import { isUserColor, type UserColor } from './color';

/**
 * Each pair's base tone, `--color-user-<name>` in app.css, light and dark.
 * The browser reads `theme-color` from a meta's content, which cannot hold
 * a CSS variable, so the values are mirrored here the way app.html mirrors
 * `--color-background`; theme-color.test.ts holds them equal to app.css.
 */
export const USER_THEME_COLORS: Record<UserColor, { light: string; dark: string }> = {
	amber: { light: '#f0dfc4', dark: '#3f2f1e' },
	clay: { light: '#f3dbcd', dark: '#43291f' },
	rose: { light: '#f5d9da', dark: '#43262a' },
	plum: { light: '#e9dcea', dark: '#372a3d' },
	sage: { light: '#dbe7d9', dark: '#25341f' },
	olive: { light: '#e6e3cf', dark: '#33301f' },
	teal: { light: '#d5e5e4', dark: '#1f3433' },
	slate: { light: '#dedfe4', dark: '#2b2f36' }
};

type Meta = Pick<Element, 'getAttribute' | 'setAttribute'>;

/**
 * Tints the browser's bar (`theme-color`) in a person's pair, or gives each
 * meta in app.html its own value back for `null`. Each meta keeps its
 * `data-scheme`; its first content is kept in `data-default` the first time.
 */
export function tintThemeColor(metas: Iterable<Meta>, color: string | null): void {
	for (const meta of metas) {
		const scheme = meta.getAttribute('data-scheme') === 'dark' ? 'dark' : 'light';
		let fallback = meta.getAttribute('data-default');
		if (fallback === null) {
			fallback = meta.getAttribute('content') ?? '';
			meta.setAttribute('data-default', fallback);
		}
		meta.setAttribute('content', isUserColor(color) ? USER_THEME_COLORS[color][scheme] : fallback);
	}
}
