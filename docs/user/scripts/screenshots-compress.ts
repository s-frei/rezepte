// Quantizes the docs screenshots and the README images to a 256-color palette
// (libimagequant inside sharp): about a third of the bytes, and nothing the eye
// sees, photos included. The git history keeps every version of every picture,
// so this is what the repository grows by per refresh.
// Runs last in `mise run //docs/user:screenshots`, and on its own as
// `mise run //docs/user:screenshots:compress`.
//
// A file that already holds a palette is skipped, so a run touches only what
// Playwright or readme-images just wrote. The same render quantizes to the same
// bytes, but Chromium's renders are not pixel-stable: two runs of the same code
// differ in about a third of the files, and a different palette then reshuffles
// the dithering across the picture. Invisible, and still a third of the bytes
// the same churn costs without a palette.
import { readdir, readFile, writeFile } from 'node:fs/promises';
import sharp from 'sharp';
import { README_SHOTS } from './readme-images';

/** Quality 90 keeps photos and anti-aliased text clean; lower starts to band gradients. */
const PALETTE = { palette: true, quality: 90, effort: 10, compressionLevel: 9 } as const;

/** PNG color type 3 (indexed) sits at byte 25, in the IHDR chunk. */
export const isPalette = (png: Buffer) => png[25] === 3;

export const toPalette = (png: Buffer) => sharp(png).png(PALETTE).toBuffer();

async function main() {
	const screenshots = new URL('../public/screenshots/', import.meta.url);
	const readme = new URL('../../../assets/readme/', import.meta.url);
	const files = [
		...(await readdir(screenshots)).filter((n) => n.endsWith('.png')).map((n) => new URL(n, screenshots)),
		...README_SHOTS.flatMap((s) => ['light', 'dark'].map((scheme) => new URL(`${s}-${scheme}.png`, readme)))
	];
	let before = 0;
	let after = 0;
	// All at once: sharp queues the work on libuv's thread pool, one image per core.
	await Promise.all(
		files.map(async (file) => {
			const png = await readFile(file);
			if (isPalette(png)) return;
			const out = await toPalette(png);
			await writeFile(file, out);
			before += png.length;
			after += out.length;
		})
	);
	console.log(`screenshots-compress: ${(before / 1048576).toFixed(1)} MB -> ${(after / 1048576).toFixed(1)} MB`);
}

if (process.argv[1] === new URL(import.meta.url).pathname) await main();
