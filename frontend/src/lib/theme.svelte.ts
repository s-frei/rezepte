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
	applyTheme(document.documentElement, next);
	applyThemeColor(document.querySelectorAll('meta[name="theme-color"]'), next);
	save(THEME_STORAGE_KEY, next === 'system' ? null : next);
}
