// Cross-checks the three places a screenshot name lives:
//   <Screenshot name="…"/> in content/**/*.mdx, test('…') in screenshots/*.spec.ts,
//   and PNGs in public/screenshots. A name used in MDX without a test fails
//   the check; missing or unused PNGs are only reported, because PNGs can
//   only be produced on a machine where Chromium starts.
import { readdirSync, readFileSync } from 'node:fs';
import path from 'node:path';

const VARIANTS = ['desktop-light', 'desktop-dark', 'mobile-light', 'mobile-dark'];

function walk(dir: string, ext: string): string[] {
	return readdirSync(dir, { withFileTypes: true }).flatMap((e) => {
		const p = path.join(dir, e.name);
		if (e.isDirectory()) return walk(p, ext);
		return e.name.endsWith(ext) ? [p] : [];
	});
}

const referenced = new Map<string, string[]>();
for (const f of walk('content', '.mdx')) {
	for (const m of readFileSync(f, 'utf8').matchAll(/<Screenshot\s[^>]*?name="([^"]+)"/g)) {
		referenced.set(m[1], [...(referenced.get(m[1]) ?? []), f]);
	}
}
const tested = new Set<string>();
for (const f of walk('screenshots', '.spec.ts')) {
	for (const m of readFileSync(f, 'utf8').matchAll(/test\('([^']+)'/g)) tested.add(m[1]);
}
const pngs = new Set(readdirSync('public/screenshots').filter((f) => f.endsWith('.png')));

let failed = false;
for (const [name, files] of referenced) {
	if (!/^[a-z0-9-]+$/.test(name)) {
		console.error(`invalid screenshot name "${name}" in ${files.join(', ')} (use a-z, 0-9, -)`);
		failed = true;
	}
	if (!tested.has(name)) {
		console.error(`"${name}" is referenced in ${files.join(', ')} but has no test('${name}') in screenshots/`);
		failed = true;
	}
}
let missing = 0;
for (const name of tested) {
	for (const v of VARIANTS) if (!pngs.has(`${name}-${v}.png`)) missing++;
	if (!referenced.has(name)) console.warn(`test('${name}') is not referenced by any page`);
}
for (const png of pngs) {
	const name = png.replace(/-(desktop|mobile)-(light|dark)\.png$/, '');
	if (!tested.has(name)) console.warn(`unused screenshot file: ${png}`);
}
if (missing > 0)
	console.warn(
		`${missing} of ${tested.size * VARIANTS.length} screenshot files are missing; pages show placeholder frames (run: mise run //docs/user:screenshots)`
	);
if (failed) process.exit(1);
console.log(`screenshots: ${referenced.size} referenced, ${tested.size} tested, ${pngs.size} files on disk`);
