// Builds every logo file the app, the docs site and the README load from the
// masters in assets/brand. Run `mise run //frontend:brand:export` after
// changing a master; `brand:check` fails when the committed files drift from
// it.
//
// The mascot is a painting, so every lockup embeds it as a WebP inside the
// SVG, on a cream box of its own: a lockup looks the same in both themes and
// needs no dark variant. Only the favicon is vector, the book alone, because
// the figure turns to a smudge at 16px.
import { generateAssets } from '@vite-pwa/assets-generator/api/generate-assets';
import { instructions } from '@vite-pwa/assets-generator/api/instructions';
import sharp from 'sharp';
import { EDGE, GROUND } from './brand-palette';

const brand = new URL('../../assets/brand/', import.meta.url);
const lockups = new URL('lockups/', brand);
const statics = new URL('../static/', import.meta.url);

/** A wordmark or lockup SVG, reduced to what placing it needs. */
export type Art = { width: number; height: number; body: string };

export function parseWordmark(svg: string): Art {
	const [, width, height] = svg.match(/viewBox="0 0 ([\d.]+) ([\d.]+)"/) ?? [];
	const body = svg.replace(/^<svg[^>]*>|<\/svg>\s*$/g, '');
	return { width: Number(width), height: Number(height), body };
}

/** The mascot, placed by its height; its aspect comes from the master. */
export type Mascot = { href: string; aspect: number };

const f = (n: number) => Number(n.toFixed(1));

function place(word: Art, x: number, y: number, height: number): string {
	const width = (height * word.width) / word.height;
	return `<svg x="${f(x)}" y="${f(y)}" width="${f(width)}" height="${f(height)}" viewBox="0 0 ${word.width} ${word.height}">${word.body}</svg>`;
}

function image(m: Mascot, x: number, y: number, height: number): string {
	return `<image x="${f(x)}" y="${f(y)}" width="${f(height * m.aspect)}" height="${f(height)}" href="${m.href}"/>`;
}

// The box: cream with a hairline, so it still reads on the app's own cream.
function box(width: number, height: number, radius: number): string {
	const stroke = f(Math.max(height, width) * 0.004 + 0.6);
	return `<rect x="${stroke / 2}" y="${stroke / 2}" width="${f(width - stroke)}" height="${f(height - stroke)}" rx="${f(radius)}" fill="${GROUND}" stroke="${EDGE}" stroke-width="${stroke}"/>`;
}

const svg = (width: number, height: number, inner: string) =>
	`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${f(width)} ${f(height)}">${inner}</svg>\n`;

/** Mascot left of the wordmark: the top bar, the docs navigation, the share page. */
export function horizontal(m: Mascot, word: Art): string {
	const h = 100;
	const mascotWidth = 80 * m.aspect;
	const wordWidth = (44 * word.width) / word.height;
	const width = 10 + mascotWidth + 12 + wordWidth + 22;
	return svg(
		width,
		h,
		box(width, h, 24) + image(m, 10, 10, 80) + place(word, 10 + mascotWidth + 12, 29, 44)
	);
}

// The social preview: the mascot large on the left, the wordmark beside it,
// centered as a group on 1280x640.
const SOCIAL = { width: 1280, height: 640, mascot: 540, word: 125, gap: 48 };

function socialLayout(aspect: number, word: Art) {
	const mascotWidth = SOCIAL.mascot * aspect;
	const wordWidth = (SOCIAL.word * word.width) / word.height;
	const x = (SOCIAL.width - mascotWidth - SOCIAL.gap - wordWidth) / 2;
	return {
		mascot: { x, y: (SOCIAL.height - SOCIAL.mascot) / 2 + 20 },
		word: { x: x + mascotWidth + SOCIAL.gap, y: (SOCIAL.height - SOCIAL.word) / 2 }
	};
}

export function manifest(): string {
	const body = {
		name: 'Rezepte',
		short_name: 'Rezepte',
		display: 'browser',
		theme_color: GROUND,
		background_color: GROUND,
		icons: [
			{ src: '/icon-192.png', sizes: '192x192', type: 'image/png' },
			{ src: '/icon-512.png', sizes: '512x512', type: 'image/png' },
			{ src: '/icon-maskable-512.png', sizes: '512x512', type: 'image/png', purpose: 'maskable' }
		]
	};
	return `${JSON.stringify(body, null, '\t')}\n`;
}

async function readLfs(path: string): Promise<Buffer> {
	const file = Bun.file(new URL(path, brand));
	const head = await file.slice(0, 40).text();
	if (head.startsWith('version https://git-lfs')) {
		throw new Error(
			`assets/brand/${path} is a Git LFS pointer; run \`git lfs pull --include="assets/brand/**"\``
		);
	}
	return Buffer.from(await file.arrayBuffer());
}

// The login screen's backdrop: the kitchen scene, wide enough to cover a
// 1280px window on a 2x screen.
async function kitchen(): Promise<Buffer> {
	const master = await readLfs('scenes/kitchen.jpg');
	return sharp(master).resize({ width: 2560 }).webp({ quality: 78, effort: 6 }).toBuffer();
}

async function webp(master: Buffer, height: number): Promise<Buffer> {
	return sharp(master)
		.resize({ height })
		.webp({ quality: 82, alphaQuality: 90, effort: 6 })
		.toBuffer();
}

async function embedded(master: Buffer, aspect: number, height: number): Promise<Mascot> {
	return {
		href: `data:image/webp;base64,${(await webp(master, height)).toString('base64')}`,
		aspect
	};
}

// The home screen icons: the mascot on its own ground, filling `share` of the
// square. The maskable icon keeps it inside the central safe zone.
async function icon(master: Buffer, size: number, share: number): Promise<Buffer> {
	const art = await sharp(master)
		.resize({ width: Math.round(size * share), height: Math.round(size * share), fit: 'inside' })
		.toBuffer();
	return sharp({ create: { width: size, height: size, channels: 3, background: GROUND } })
		.composite([{ input: art, gravity: 'center' }])
		.png({ compressionLevel: 9, palette: true, quality: 92 })
		.toBuffer();
}

async function socialPreview(
	master: Buffer,
	aspect: number,
	wordmark: string,
	word: Art
): Promise<Buffer> {
	const at = socialLayout(aspect, word);
	const art = await sharp(master).resize({ height: SOCIAL.mascot }).toBuffer();
	const letters = await sharp(Buffer.from(wordmark), { density: 400 })
		.resize({ height: SOCIAL.word })
		.toBuffer();
	return sharp({
		create: { width: SOCIAL.width, height: SOCIAL.height, channels: 3, background: GROUND }
	})
		.composite([
			{ input: art, left: Math.round(at.mascot.x), top: Math.round(at.mascot.y) },
			{ input: letters, left: Math.round(at.word.x), top: Math.round(at.word.y) }
		])
		.png({ compressionLevel: 9, palette: true, quality: 92 })
		.toBuffer();
}

// favicon.ico, through @vite-pwa/assets-generator, which writes ICO files.
async function faviconIco(book: string) {
	const plan = await instructions({
		imageResolver: async () => Buffer.from(book),
		imageName: 'rezepte-book.svg',
		preset: {
			transparent: { sizes: [], padding: 0, favicons: [[32, 'favicon.ico']] },
			maskable: { sizes: [] },
			apple: { sizes: [] }
		},
		faviconPreset: '2023',
		htmlLinks: { xhtml: false, includeId: false },
		basePath: '/',
		resolveSvgName: (name) => name
	});
	await generateAssets(plan, true, statics.pathname);
}

async function main() {
	const master = await readLfs('mascot/rezepte-mascot.png');
	const { width = 1, height = 1 } = await sharp(master).metadata();
	const aspect = width / height;
	const wordmark = await Bun.file(new URL('rezepte-wordmark.svg', lockups)).text();
	const word = parseWordmark(wordmark);

	await Bun.write(
		new URL('rezepte-lockup-horizontal.svg', lockups),
		horizontal(await embedded(master, aspect, 240), word)
	);
	await Bun.write(new URL('rezepte-mascot.webp', lockups), await webp(master, 144));
	await Bun.write(new URL('rezepte-kitchen.webp', lockups), await kitchen());

	const book = await Bun.file(new URL('icon/rezepte-book.svg', brand)).text();
	await Bun.write(new URL('favicon.svg', statics), book);
	await faviconIco(book);
	await Bun.write(new URL('apple-touch-icon.png', statics), await icon(master, 180, 0.84));
	await Bun.write(new URL('icon-192.png', statics), await icon(master, 192, 0.84));
	await Bun.write(new URL('icon-512.png', statics), await icon(master, 512, 0.84));
	await Bun.write(new URL('icon-maskable-512.png', statics), await icon(master, 512, 0.66));
	await Bun.write(new URL('manifest.webmanifest', statics), manifest());

	const preview = await socialPreview(master, aspect, wordmark, word);
	await Bun.write(new URL('og.png', statics), preview);
	await Bun.write(new URL('social-preview.png', brand), preview);
}

if (import.meta.main) await main();
