import { browser } from '$app/environment';
import { load, save } from './storage';
import { applyTheme, applyThemeColor, isTheme, THEME_STORAGE_KEY, type Theme } from './theme';

const stored = load(THEME_STORAGE_KEY);

/** The user's theme choice. Read `theme.value`; change it with `setTheme`. */
export const theme = $state<{ value: Theme }>({ value: isTheme(stored) ? stored : 'system' });

export function setTheme(next: Theme): void {
	theme.value = next;
	if (!browser) {
		return;
	}
	// The new colors open as a circle from the control that chose them
	// (app.css): every surface switching in one frame flashed.
	const root = document.documentElement;
	const apply = () => {
		applyTheme(root, next);
		applyThemeColor(document.querySelectorAll('meta[name="theme-color"]'), next);
	};
	if (
		'startViewTransition' in document &&
		!window.matchMedia('(prefers-reduced-motion: reduce)').matches
	) {
		const from = document.activeElement?.getBoundingClientRect();
		if (from && from.width > 0) {
			root.style.setProperty('--theme-x', `${from.left + from.width / 2}px`);
			root.style.setProperty('--theme-y', `${from.top + from.height / 2}px`);
		}
		root.dataset.themeTransition = '';
		void document
			.startViewTransition(apply)
			.finished.catch(() => {})
			.finally(() => {
				delete root.dataset.themeTransition;
				root.style.removeProperty('--theme-x');
				root.style.removeProperty('--theme-y');
			});
	} else {
		apply();
	}
	save(THEME_STORAGE_KEY, next === 'system' ? null : next);
}
