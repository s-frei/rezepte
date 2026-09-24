import { expect, test } from 'bun:test';
import { readdirSync } from 'node:fs';
import { favicon, manifest } from './brand-export';
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

const lockups = new URL('../../assets/brand/lockups/', import.meta.url);
const variants = ['', '-dark', '-mono', '-mono-dark'];
const lockupFiles = ['wordmark', 'lockup-horizontal', 'lockup-stacked', 'lockup-compact'].flatMap(
	(base) => variants.map((v) => `rezepte-${base}${v}.svg`)
);

test('there are sixteen wordmark and lockup files', () => {
	const found = readdirSync(lockups)
		.filter((f) => f.endsWith('.svg'))
		.sort();
	expect(found).toEqual([...lockupFiles].sort());
});

for (const file of lockupFiles) {
	test(`${file} is a clean wordmark or lockup`, async () => {
		const svg = await Bun.file(new URL(file, lockups)).text();
		// Starting at 0 0 lets an <img> or inline SVG size it by height alone.
		expect(svg).toMatch(/viewBox="0 0 [\d.]+ [\d.]+"/);
		expect(svg).not.toMatch(/<(image|text|style|script|foreignObject|metadata)\b/);
		expect(svg).toContain('<g class="wordmark"');

		const colors = [...svg.matchAll(/#[0-9a-fA-F]{3,8}\b/g)].map((m) => m[0].toLowerCase());
		for (const color of colors) expect(ALLOWED).toContain(color);

		const palette = file.includes('-dark') ? DARK : LIGHT;
		const mono = file.includes('-mono');
		expect(svg).toContain(`<g class="ink" fill="${palette.ink}"`);
		// The wordmark alone has no sprout; every lockup carries the icon's.
		const groups = file.includes('wordmark') ? 1 : 2;
		if (groups === 2) {
			expect(svg).toContain(`<g class="sprout" fill="${mono ? palette.ink : palette.sprout}"`);
		}
		expect([...svg.matchAll(/\s(fill|stroke|color|stop-color)=/g)].length).toBe(groups);
		expect(svg).not.toMatch(/\s(style|opacity|fill-opacity)=|<(linearGradient|radialGradient)\b/);
	});
}

const sample = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512"><g class="sprout" fill="${LIGHT.sprout}"><path d="M0 0h1v1z"/></g><g class="ink" fill="${LIGHT.ink}"><path d="M1 1h1v1z"/></g></svg>`;

test('favicon switches ink and sprout to the dark values on a dark scheme', () => {
	const svg = favicon(sample);
	expect(svg).toContain('@media (prefers-color-scheme: dark)');
	expect(svg).toContain(`.ink{fill:${DARK.ink}}`);
	expect(svg).toContain(`.sprout{fill:${DARK.sprout}}`);
	expect(svg).toContain(`fill="${LIGHT.ink}"`); // light stays the default
	expect(svg.indexOf('<style>')).toBeGreaterThan(svg.indexOf('<svg'));
});

test('manifest names the app and its icons', () => {
	const m = JSON.parse(manifest());
	expect(m).toMatchObject({
		name: 'Rezepte',
		short_name: 'Rezepte',
		display: 'browser',
		theme_color: LIGHT.ground,
		background_color: LIGHT.ground
	});
	expect(m.icons.map((i: { src: string }) => i.src)).toEqual([
		'/icon-192.png',
		'/icon-512.png',
		'/icon-maskable-512.png'
	]);
	expect(m.icons[2].purpose).toBe('maskable');
});

const statics = new URL('../static/', import.meta.url);
const bytes = async (name: string) =>
	new Uint8Array(await Bun.file(new URL(name, statics)).arrayBuffer());

test('committed favicon.svg and manifest match the generator', async () => {
	const small16 = await Bun.file(new URL('onion-small-16.svg', logo)).text();
	expect(await Bun.file(new URL('favicon.svg', statics)).text()).toBe(favicon(small16));
	expect(await Bun.file(new URL('manifest.webmanifest', statics)).text()).toBe(manifest());
});

// Rasters are checked by format and size only: sharp may anti-alias a pixel
// differently on another platform, so a byte comparison would fail CI.
test.each([
	['apple-touch-icon.png', 180],
	['icon-192.png', 192],
	['icon-512.png', 512],
	['icon-maskable-512.png', 512]
])('%s is a %dpx png', async (name, size) => {
	const data = await bytes(name);
	const view = new DataView(data.buffer);
	expect([...data.slice(1, 4)]).toEqual([0x50, 0x4e, 0x47]);
	expect([view.getUint32(16), view.getUint32(20)]).toEqual([size, size]);
});

test('favicon.ico is an icon file with a 32px image', async () => {
	const data = await bytes('favicon.ico');
	const view = new DataView(data.buffer);
	expect(view.getUint16(0, true)).toBe(0);
	expect(view.getUint16(2, true)).toBe(1); // type: icon
	expect(data[6]).toBe(32);
});
