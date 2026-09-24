import { expect, test } from 'bun:test';
import { readdirSync } from 'node:fs';
import { ALLOWED, DARK, LIGHT } from './brand-palette';

const logo = new URL('../../assets/brand/logo/', import.meta.url);
const masters = readdirSync(logo)
	.filter((f) => f.endsWith('.svg'))
	.sort();

test('there are twelve masters', () => {
	const expected = ['onion-l1', 'onion-small', 'onion-small-16']
		.flatMap((base) => ['', '-dark', '-mono', '-mono-dark'].map((v) => `${base}${v}.svg`))
		.sort();
	expect(masters).toEqual(expected);
});

test('the logo tokens in app.css match the palette in every theme block', async () => {
	const css = await Bun.file(new URL('../src/app.css', import.meta.url)).text();
	const values = (token: string) =>
		[...css.matchAll(new RegExp(`--color-logo-${token}:\\s*(#[0-9a-f]{6})`, 'g'))].map((m) => m[1]);
	for (const token of ['ink', 'sprout', 'ground'] as const) {
		// @theme, then the explicit dark block, then the system dark block.
		expect(values(token)).toEqual([LIGHT[token], DARK[token], DARK[token]]);
	}
});

for (const file of masters) {
	test(`${file} is a clean master`, async () => {
		const svg = await Bun.file(new URL(file, logo)).text();
		expect(svg).toContain('viewBox="0 0 512 512"');
		expect(svg).not.toMatch(/<(image|text|style|script|foreignObject|metadata)\b/);

		const colors = [...svg.matchAll(/#[0-9a-fA-F]{3,8}\b/g)].map((m) => m[0].toLowerCase());
		for (const color of colors) expect(ALLOWED).toContain(color);

		const palette = file.includes('-dark') ? DARK : LIGHT;
		const mono = file.includes('-mono');
		expect(svg).toContain(`<g class="ink" fill="${palette.ink}"`);
		expect(svg).toContain(`<g class="sprout" fill="${mono ? palette.ink : palette.sprout}"`);
		// Colors live on the two groups only, so the app can recolor them. Count
		// paint attributes too, since a named or rgb() color is not a hex match.
		expect(colors.length).toBe(2);
		expect([...svg.matchAll(/\s(fill|stroke|color|stop-color)=/g)].length).toBe(2);
		expect(svg).not.toMatch(/\s(style|opacity|fill-opacity)=|<(linearGradient|radialGradient)\b/);
	});
}
