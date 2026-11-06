import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import { USER_COLORS } from './color';
import { tintThemeColor, USER_THEME_COLORS } from './theme-color';

const css = readFileSync(new URL('../../app.css', import.meta.url), 'utf8');

/** The value of `--color-user-<name>` in the block that `start` opens. */
function token(start: string, name: string): string | undefined {
	const block = css.slice(css.indexOf(start));
	return block.match(new RegExp(`--color-user-${name}:\\s*(#[0-9a-f]{6});`))?.[1];
}

describe('USER_THEME_COLORS', () => {
	it('mirrors every base tone in app.css, light and dark', () => {
		for (const color of USER_COLORS) {
			expect(USER_THEME_COLORS[color].light).toBe(token('@theme {', color));
			expect(USER_THEME_COLORS[color].dark).toBe(token(":root[data-theme='dark'] {", color));
		}
	});
});

function meta(scheme: string, content: string) {
	const attrs = new Map([
		['data-scheme', scheme],
		['content', content]
	]);
	return {
		getAttribute: (name: string) => attrs.get(name) ?? null,
		setAttribute: (name: string, value: string) => void attrs.set(name, value),
		content: () => attrs.get('content')
	};
}

describe('tintThemeColor', () => {
	it("takes each scheme's tone and gives the default back", () => {
		const metas = [meta('light', '#f4ede2'), meta('dark', '#1c1612')];
		tintThemeColor(metas, 'sage');
		expect(metas.map((m) => m.content())).toEqual(['#dbe7d9', '#25341f']);
		tintThemeColor(metas, null);
		expect(metas.map((m) => m.content())).toEqual(['#f4ede2', '#1c1612']);
	});

	it('stays on the default for a color it does not know', () => {
		const metas = [meta('light', '#f4ede2')];
		tintThemeColor(metas, 'chartreuse');
		expect(metas[0].content()).toBe('#f4ede2');
	});
});
