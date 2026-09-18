import { describe, expect, it } from 'vitest';
import { applyTheme, readStoredTheme, THEME_STORAGE_KEY } from './theme';

function storageWith(value: string | null) {
	return { getItem: (key: string) => (key === THEME_STORAGE_KEY ? value : null) };
}

describe('readStoredTheme', () => {
	it('returns the stored theme', () => {
		expect(readStoredTheme(storageWith('dark'))).toBe('dark');
		expect(readStoredTheme(storageWith('light'))).toBe('light');
	});
	it('falls back to system for missing, unknown or unreadable values', () => {
		expect(readStoredTheme(storageWith(null))).toBe('system');
		expect(readStoredTheme(storageWith('blue'))).toBe('system');
		expect(readStoredTheme(null)).toBe('system');
		expect(
			readStoredTheme({
				getItem: () => {
					throw new Error('blocked');
				}
			})
		).toBe('system');
	});
});

describe('applyTheme', () => {
	it('sets data-theme for explicit choices and removes it for system', () => {
		const calls: string[] = [];
		const root = {
			setAttribute: (name: string, value: string) => calls.push(`set ${name}=${value}`),
			removeAttribute: (name: string) => calls.push(`remove ${name}`)
		};
		applyTheme(root, 'dark');
		applyTheme(root, 'light');
		applyTheme(root, 'system');
		expect(calls).toEqual(['set data-theme=dark', 'set data-theme=light', 'remove data-theme']);
	});
});
