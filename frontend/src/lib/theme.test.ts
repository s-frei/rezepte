import { describe, expect, it } from 'vitest';
import { applyTheme, applyThemeColor } from './theme';

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

describe('applyThemeColor', () => {
	function metas() {
		return ['light', 'dark'].map((scheme) => {
			const attrs: Record<string, string> = { 'data-scheme': scheme };
			return {
				attrs,
				getAttribute: (name: string) => attrs[name] ?? null,
				setAttribute: (name: string, value: string) => {
					attrs[name] = value;
				}
			};
		});
	}
	const media = (list: ReturnType<typeof metas>) => list.map((meta) => meta.attrs.media);

	it('turns on only the chosen theme and hands system back to the media query', () => {
		const list = metas();
		applyThemeColor(list, 'dark');
		expect(media(list)).toEqual(['not all', 'all']);
		applyThemeColor(list, 'light');
		expect(media(list)).toEqual(['all', 'not all']);
		applyThemeColor(list, 'system');
		expect(media(list)).toEqual(['(prefers-color-scheme: light)', '(prefers-color-scheme: dark)']);
	});
});
