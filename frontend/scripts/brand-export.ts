// Builds every logo file the browser loads from the masters in
// assets/brand/logo. Run `mise run //frontend:brand:export` after changing a
// master; `brand:check` fails when the committed files drift from it.
//
// The rasters come from @vite-pwa/assets-generator, run twice because the
// icon is responsive: L1 (the brush drawing) for the large icons, small-16
// (no face) for favicon.ico, where L1's brush would blur.
import { generateAssets } from '@vite-pwa/assets-generator/api/generate-assets';
import { instructions } from '@vite-pwa/assets-generator/api/instructions';
import sharp from 'sharp';
import { DARK, LIGHT } from './brand-palette';

const logo = new URL('../../assets/brand/logo/', import.meta.url);
const brand = new URL('../../assets/brand/', import.meta.url);

// The link preview (og:image) for the app, the docs site and the GitHub
// repository: the light stacked lockup, about 300px high, centered on the
// dark ground, at the 1280x640 that GitHub and most chat apps expect.
async function socialPreview(): Promise<Buffer> {
	const lockup = await Bun.file(
		new URL('lockups/rezepte-lockup-stacked-dark.svg', brand)
	).arrayBuffer();
	const art = await sharp(Buffer.from(lockup), { density: 300 })
		.resize({ height: 300 })
		.png()
		.toBuffer();
	return sharp({ create: { width: 1280, height: 640, channels: 3, background: DARK.ground } })
		.composite([{ input: art, gravity: 'center' }])
		.png({ compressionLevel: 9 })
		.toBuffer();
}
const statics = new URL('../static/', import.meta.url);

// The small-16 master plus a dark-scheme override. CSS beats the fill
// attributes on the two groups, so the light values stay the default.
export function favicon(master: string): string {
	const style = `<style>@media (prefers-color-scheme: dark){.ink{fill:${DARK.ink}}.sprout{fill:${DARK.sprout}}}</style>`;
	return master.replace(/<svg\b[^>]*>/, (open) => `${open}${style}`);
}

export function manifest(): string {
	const body = {
		name: 'Rezepte',
		short_name: 'Rezepte',
		display: 'browser',
		theme_color: LIGHT.ground,
		background_color: LIGHT.ground,
		icons: [
			{ src: '/icon-192.png', sizes: '192x192', type: 'image/png' },
			{ src: '/icon-512.png', sizes: '512x512', type: 'image/png' },
			{ src: '/icon-maskable-512.png', sizes: '512x512', type: 'image/png', purpose: 'maskable' }
		]
	};
	return `${JSON.stringify(body, null, '\t')}\n`;
}

type Preset = Parameters<typeof instructions>[0]['preset'];

async function rasterize(master: string, preset: Preset) {
	const file = new URL(master, logo);
	const plan = await instructions({
		imageResolver: async () => Buffer.from(await Bun.file(file).arrayBuffer()),
		imageName: master,
		preset,
		faviconPreset: '2023',
		htmlLinks: { xhtml: false, includeId: false },
		basePath: '/',
		resolveSvgName: (name) => name
	});
	await generateAssets(plan, true, statics.pathname);
}

const none = { sizes: [] as number[] };
const png = { compressionLevel: 9, quality: 90 };

async function main() {
	// L1 already carries its own padding, so only the maskable icon adds
	// some, to keep the onion inside the central safe zone.
	await rasterize('onion-l1.svg', {
		transparent: { sizes: [192, 512], padding: 0 },
		maskable: { sizes: [512], padding: 0.1, resizeOptions: { background: LIGHT.ground } },
		apple: { sizes: [180], padding: 0, resizeOptions: { background: LIGHT.ground } },
		png,
		assetName: (type, size): string =>
			({
				transparent: `icon-${size.width}.png`,
				maskable: `icon-maskable-${size.width}.png`,
				apple: 'apple-touch-icon.png'
			})[type]
	});
	await rasterize('onion-small-16.svg', {
		transparent: { sizes: [], padding: 0, favicons: [[32, 'favicon.ico']] },
		maskable: none,
		apple: none,
		png
	});

	const small16 = await Bun.file(new URL('onion-small-16.svg', logo)).text();
	await Bun.write(new URL('favicon.svg', statics), favicon(small16));
	await Bun.write(new URL('manifest.webmanifest', statics), manifest());

	const preview = await socialPreview();
	await Bun.write(new URL('og.png', statics), preview);
	await Bun.write(new URL('social-preview.png', brand), preview);
}

if (import.meta.main) await main();
