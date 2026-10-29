import { describe, expect, it } from 'vitest';
import { applyTheme } from './theme';

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
