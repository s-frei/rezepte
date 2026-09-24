// Rounds the overview screenshots for the README. GitHub strips CSS from a
// README, so the corners and the border the docs site draws around every
// <Screenshot> (rounded-xl, 1px fd-border) are baked into the picture here.
// Runs after `mise run //docs/user:screenshots`, and on its own as
// `mise run //docs/user:readme-images`.
import { mkdir, readFile, writeFile } from 'node:fs/promises';
import sharp from 'sharp';

// The docs site's fd-border in each scheme (app/global.css).
export const BORDER = { light: '#e3d7c3', dark: '#3b322a' } as const;

// A 1440px screenshot is shown about 760px wide on the docs site, where
// rounded-xl is 18px and the border 1px; scaled to the picture that is
// about 32px and 2px.
const RADIUS = 32;
const STROKE = 2;

export async function rounded(png: ArrayBuffer | Buffer, scheme: keyof typeof BORDER): Promise<Buffer> {
	const image = sharp(png);
	const { width = 0, height = 0 } = await image.metadata();
	const inset = STROKE / 2;
	const shape = (paint: string) =>
		Buffer.from(
			`<svg xmlns="http://www.w3.org/2000/svg" width="${width}" height="${height}">` +
				`<rect x="${inset}" y="${inset}" width="${width - STROKE}" height="${height - STROKE}" rx="${RADIUS - inset}" ${paint}/></svg>`
		);
	const bordered = await image
		.composite([{ input: shape(`fill="none" stroke="${BORDER[scheme]}" stroke-width="${STROKE}"`) }])
		.png()
		.toBuffer();
	const mask = Buffer.from(
		`<svg xmlns="http://www.w3.org/2000/svg" width="${width}" height="${height}"><rect width="${width}" height="${height}" rx="${RADIUS}" fill="#fff"/></svg>`
	);
	return sharp(bordered)
		.ensureAlpha()
		.composite([{ input: mask, blend: 'dest-in' }])
		.png({ compressionLevel: 9 })
		.toBuffer();
}

// node:fs rather than Bun.file: Next type-checks every script in this
// package, and its tsconfig carries no Bun types.
async function main() {
	const target = new URL('../../../assets/readme/', import.meta.url);
	await mkdir(target, { recursive: true });
	for (const scheme of ['light', 'dark'] as const) {
		const name = `overview-desktop-${scheme}.png`;
		const source = await readFile(new URL(`../public/screenshots/${name}`, import.meta.url));
		await writeFile(new URL(name, target), await rounded(source, scheme));
	}
}

if (process.argv[1] === new URL(import.meta.url).pathname) await main();
