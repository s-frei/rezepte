// Copies the logo files the docs site shows into this package. Next cannot
// import from outside the package at build time, so the site carries copies,
// and scripts/brand-assets.test.ts (part of `check`) fails when one drifts
// from its source. Run `mise run //docs/user:brand-assets` after
// `mise run //frontend:brand:export` or after changing a lockup.
import { copyFile, mkdir } from 'node:fs/promises';

// [target in docs/user, source from the repository root]
export const COPIES: [string, string][] = [
	['public/brand/rezepte-lockup-compact.svg', 'assets/brand/lockups/rezepte-lockup-compact.svg'],
	['public/brand/rezepte-lockup-compact-dark.svg', 'assets/brand/lockups/rezepte-lockup-compact-dark.svg'],
	['public/brand/rezepte-lockup-horizontal.svg', 'assets/brand/lockups/rezepte-lockup-horizontal.svg'],
	['public/brand/rezepte-lockup-horizontal-dark.svg', 'assets/brand/lockups/rezepte-lockup-horizontal-dark.svg'],
	// The tab icon, through Next's app/icon convention.
	['app/icon.svg', 'frontend/static/favicon.svg'],
	// The link preview. Not app/opengraph-image: Next prefixes the base path
	// to that URL on top of metadataBase, which already ends in /rezepte/, and
	// publishes /rezepte/rezepte/…; lib/shared.ts points at ./og.png instead.
	['public/og.png', 'assets/brand/social-preview.png']
];

async function main() {
	const root = new URL('../../../', import.meta.url);
	const docs = new URL('../', import.meta.url);
	await mkdir(new URL('public/brand/', docs), { recursive: true });
	for (const [target, source] of COPIES) {
		await copyFile(new URL(source, root), new URL(target, docs));
	}
}

if (process.argv[1] === new URL(import.meta.url).pathname) await main();
