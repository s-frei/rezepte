import { expect, test } from 'bun:test';
import { readdirSync } from 'node:fs';
import sharp from 'sharp';
import { banner, horizontal, manifest, parseWordmark, stacked } from './brand-export';
import { BOOK, EDGE, GROUND, INK } from './brand-palette';

// brand:check never reads the mascot master: it sits in Git LFS, which CI
// does not fetch. It checks the committed outputs instead.
const brand = new URL('../../assets/brand/', import.meta.url);
const lockups = new URL('lockups/', brand);
const statics = new URL('../static/', import.meta.url);
const text = (url: URL) => Bun.file(url).text();

const colors = (svg: string) =>
	[...svg.matchAll(/#[0-9a-fA-F]{3,8}\b/g)].map((m) => m[0].toLowerCase());

test('the wordmark is one path of letters in the ink', async () => {
	const svg = await text(new URL('rezepte-wordmark.svg', lockups));
	expect(svg).toMatch(/^<svg xmlns="http:\/\/www.w3.org\/2000\/svg" viewBox="0 0 [\d.]+ [\d.]+">/);
	expect([...svg.matchAll(/<path\b/g)].length).toBe(1);
	expect(colors(svg)).toEqual([INK]);
	expect(svg).not.toMatch(/<(image|text|style|script)\b|transform=/);
});

test('the favicon book is a flat vector in its own palette', async () => {
	const svg = await text(new URL('icon/rezepte-book.svg', brand));
	const [, , , w, h] = svg.match(/viewBox="([\d.-]+) ([\d.-]+) ([\d.]+) ([\d.]+)"/) ?? [];
	expect(w).toBe(h); // square, so the favicon is not squashed
	for (const color of colors(svg)) expect(BOOK).toContain(color);
	expect(svg).not.toMatch(/<(image|text|style|script|foreignObject)\b|transform=|\sstyle=/);
});

const expected = [
	'rezepte-banner.svg',
	'rezepte-lockup-horizontal.svg',
	'rezepte-lockup-stacked.svg'
];

test('the lockups are the banner, the horizontal and the stacked one', () => {
	const found = readdirSync(lockups).filter(
		(f) => f.endsWith('.svg') && f !== 'rezepte-wordmark.svg'
	);
	expect(found.sort()).toEqual(expected);
});

// The embedded WebP differs per platform, so the lockups are checked by shape:
// a cream box, the mascot inlined (an <img> loads no external file), the
// wordmark in the ink, and nothing that could run.
for (const file of expected) {
	test(`${file} is a boxed, self-contained lockup`, async () => {
		const svg = await text(new URL(file, lockups));
		expect(svg).toMatch(/viewBox="0 0 [\d.]+ [\d.]+"/);
		expect(svg).toContain(`fill="${GROUND}" stroke="${EDGE}"`);
		expect([...svg.matchAll(/<image\b[^>]*href="data:image\/webp;base64,/g)].length).toBe(1);
		expect(svg).toContain(`fill="${INK}"`);
		expect(svg).not.toMatch(/<(script|style|foreignObject)\b|href="(?!data:)/);
	});
}

test('the layouts keep the wordmark inside the box', async () => {
	const word = parseWordmark(await text(new URL('rezepte-wordmark.svg', lockups)));
	const mascot = { href: 'data:image/webp;base64,', aspect: 1 };
	for (const svg of [horizontal(mascot, word), stacked(mascot, word), banner(mascot, word)]) {
		const [, width, height] = svg.match(/viewBox="0 0 ([\d.]+) ([\d.]+)"/)!.map(Number);
		for (const m of svg.matchAll(
			/<svg x="([\d.]+)" y="([\d.]+)" width="([\d.]+)" height="([\d.]+)"/g
		)) {
			const [x, y, w, h] = m.slice(1).map(Number);
			expect(x + w).toBeLessThanOrEqual(width);
			expect(y + h).toBeLessThanOrEqual(height);
		}
	}
});

test('the favicon is the book, and the manifest what the generator writes', async () => {
	expect(await text(new URL('favicon.svg', statics))).toBe(
		await text(new URL('icon/rezepte-book.svg', brand))
	);
	expect(await text(new URL('manifest.webmanifest', statics))).toBe(manifest());
});

test('manifest names the app and its icons', () => {
	const m = JSON.parse(manifest());
	expect(m).toMatchObject({
		name: 'Rezepte',
		short_name: 'Rezepte',
		display: 'browser',
		theme_color: GROUND,
		background_color: GROUND
	});
	expect(m.icons.map((i: { src: string }) => i.src)).toEqual([
		'/icon-192.png',
		'/icon-512.png',
		'/icon-maskable-512.png'
	]);
	expect(m.icons[2].purpose).toBe('maskable');
});

async function pixels(url: URL) {
	return sharp(Buffer.from(await Bun.file(url).arrayBuffer()))
		.removeAlpha()
		.raw()
		.toBuffer({ resolveWithObject: true });
}

const hexAt = ({ data, info }: Awaited<ReturnType<typeof pixels>>, x: number, y: number) => {
	const i = (y * info.width + x) * 3;
	return `#${[...data.subarray(i, i + 3)].map((c) => c.toString(16).padStart(2, '0')).join('')}`;
};

// Rasters are checked by size and ground only: sharp may anti-alias a pixel
// differently on another platform, so a byte comparison would fail CI.
test.each([
	['apple-touch-icon.png', 180],
	['icon-192.png', 192],
	['icon-512.png', 512],
	['icon-maskable-512.png', 512]
])('%s is a %dpx square on the mascot ground', async (name, size) => {
	const img = await pixels(new URL(name, statics));
	expect([img.info.width, img.info.height]).toEqual([size, size]);
	expect(hexAt(img, 0, 0)).toBe(GROUND);
	expect(hexAt(img, size - 1, size - 1)).toBe(GROUND);
});

test('favicon.ico is an icon file with a 32px image', async () => {
	const data = new Uint8Array(await Bun.file(new URL('favicon.ico', statics)).arrayBuffer());
	const view = new DataView(data.buffer);
	expect(view.getUint16(0, true)).toBe(0);
	expect(view.getUint16(2, true)).toBe(1); // type: icon
	expect(data[6]).toBe(32);
});

// The social preview, shared by the app (static/og.png), the docs site and
// the GitHub repository: the mascot and the wordmark on the cream ground.
test.each([['frontend/static/og.png'], ['assets/brand/social-preview.png']])(
	'%s is the 1280x640 social preview on the mascot ground',
	async (path) => {
		const img = await pixels(new URL(`../../${path}`, import.meta.url));
		expect([img.info.width, img.info.height]).toEqual([1280, 640]);
		expect(hexAt(img, 0, 0)).toBe(GROUND);
		expect(hexAt(img, 1279, 639)).toBe(GROUND);
		// The wordmark sits on the vertical middle, right of the mascot.
		const row = [...Array(img.info.width).keys()].map((x) => hexAt(img, x, 320));
		expect(row.slice(640)).toContain(INK);
	}
);
