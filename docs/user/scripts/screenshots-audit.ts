// Cross-checks the places a screenshot name lives:
//   <Screenshot name="…"/> in content/**/*.mdx, `shot: '…'` in the landing
//   page's tiles (app/page.tsx), test('…') in screenshots/*.spec.ts, and PNGs
//   in public/screenshots. A name used in a page without a test fails the
//   check; missing or unused PNGs are only reported, because PNGs can only be
//   produced on a machine where Chromium starts.
import { readdirSync, readFileSync } from 'node:fs';
import path from 'node:path';

const VARIANTS = ['desktop-light', 'desktop-dark', 'mobile-light', 'mobile-dark'];

function walk(dir: string, ext: string): string[] {
	return readdirSync(dir, { recursive: true, encoding: 'utf8' }).filter((f) => f.endsWith(ext)).map((f) => path.join(dir, f));
}

const referenced = new Map<string, string[]>();
for (const f of walk('content', '.mdx')) {
	for (const m of readFileSync(f, 'utf8').matchAll(/<Screenshot\s[^>]*?name="([^"]+)"/g)) {
		referenced.set(m[1], [...(referenced.get(m[1]) ?? []), f]);
	}
}
// The landing page builds its image paths from these names itself, so a
// renamed test would otherwise leave a broken picture on the front page.
for (const m of readFileSync('app/page.tsx', 'utf8').matchAll(/\bshot: '([^']+)'/g)) {
	referenced.set(m[1], [...(referenced.get(m[1]) ?? []), 'app/page.tsx']);
}
// A test that skips one project (`test.skip(!isMobile(testInfo), …)` for a
// phone-only screen, `test.skip(isMobile(testInfo), …)` for a desktop-only
// one) never writes the other project's PNGs, so they are not expected.
// Only these two exact skips narrow the set; anything else expects all four.
const tested = new Map<string, string[]>();
for (const f of walk('screenshots', '.spec.ts')) {
	const src = readFileSync(f, 'utf8');
	const tests = [...src.matchAll(/test\('([^']+)'/g)];
	tests.forEach((m, i) => {
		const body = src.slice(m.index, tests[i + 1]?.index ?? src.length);
		const only = /test\.skip\(\s*!isMobile\(/.test(body)
			? 'mobile'
			: /test\.skip\(\s*isMobile\(/.test(body)
				? 'desktop'
				: null;
		tested.set(m[1], only ? VARIANTS.filter((v) => v.startsWith(only)) : VARIANTS);
	});
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
let expected = 0;
for (const [name, variants] of tested) {
	expected += variants.length;
	for (const v of variants) if (!pngs.has(`${name}-${v}.png`)) missing++;
	if (!referenced.has(name)) console.warn(`test('${name}') is not referenced by any page`);
}
for (const png of pngs) {
	const name = png.replace(/-(desktop|mobile)-(light|dark)\.png$/, '');
	if (!tested.has(name)) console.warn(`unused screenshot file: ${png}`);
}
if (missing > 0)
	console.warn(
		`${missing} of ${expected} screenshot files are missing; pages show placeholder frames (run: mise run //docs/user:screenshots)`
	);
if (failed) process.exit(1);
console.log(`screenshots: ${referenced.size} referenced, ${tested.size} tested, ${pngs.size} files on disk`);
