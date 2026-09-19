// Fails the build if a generated page requests a root-relative URL that does not
// carry the deployed base path (/rezepte). This is the gate Critical 1 (ADR 0015)
// was missing: `//docs/user:check` was lint + the screenshot-name audit + `next
// build`, and none of those serve the export and follow a link, so a src that
// 404s under the base path passed every existing check and two task reviews.
//
// React/Next also emit one root-relative link that is a resource *hint*, not a
// resource load - `<link rel="preconnect" href="/">` - independent of basePath
// and not a broken asset. It is stripped before scanning so it is not mistaken
// for one. Everything else that fetches something (script, img, stylesheet,
// anchor) must carry the base path.
//
// Deliberately narrow: this asserts the base path only, it is not a link checker.
import { existsSync, readdirSync, readFileSync } from 'node:fs';
import path from 'node:path';
import nextConfig from '../next.config.mjs';

// Taken from the config rather than repeated here: the base path has exactly
// one owner, and a second literal would go stale silently. Importing the config
// regenerates `.source/` as a side effect - gitignored, and the build does it
// anyway.
const BASE_PATH = nextConfig.basePath ?? '';

function walk(dir: string): string[] {
	return readdirSync(dir, { withFileTypes: true }).flatMap((e) => {
		const p = path.join(dir, e.name);
		if (e.isDirectory()) return walk(p);
		return e.name.endsWith('.html') ? [p] : [];
	});
}

if (!existsSync('out')) {
	throw new Error('out/ does not exist - run `mise run //docs/user:build` first');
}

let checked = 0;
let failed = false;
for (const file of walk('out')) {
	const html = readFileSync(file, 'utf8').replace(
		/<link[^>]*\brel="(?:preconnect|dns-prefetch)"[^>]*>/g,
		''
	);
	for (const m of html.matchAll(/\b(?:src|href)="(\/[^"]*)"/g)) {
		const value = m[1];
		// `//host/path` is protocol-relative - an absolute URL on another origin
		// that merely looks root-relative. The base path does not apply to it.
		if (value.startsWith('//')) continue;
		checked++;
		if (value === BASE_PATH || value.startsWith(`${BASE_PATH}/`)) continue;
		console.error(`${file}: root-relative path "${value}" is missing the "${BASE_PATH}" base path`);
		failed = true;
	}
}
if (checked === 0) throw new Error('no src/href attributes found under out/ - did the build run first?');
if (failed) process.exit(1);
console.log(`base-path audit: ${checked} root-relative src/href attributes all carry "${BASE_PATH}"`);
