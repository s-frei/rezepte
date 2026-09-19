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
import { readdirSync, readFileSync } from 'node:fs';
import path from 'node:path';

const BASE_PATH = '/rezepte';

function walk(dir: string): string[] {
	return readdirSync(dir, { withFileTypes: true }).flatMap((e) => {
		const p = path.join(dir, e.name);
		if (e.isDirectory()) return walk(p);
		return e.name.endsWith('.html') ? [p] : [];
	});
}

let checked = 0;
let failed = false;
for (const file of walk('out')) {
	const html = readFileSync(file, 'utf8').replace(
		/<link[^>]*\brel="(?:preconnect|dns-prefetch)"[^>]*>/g,
		''
	);
	for (const m of html.matchAll(/\b(?:src|href)="(\/[^"]*)"/g)) {
		checked++;
		const value = m[1];
		if (value === BASE_PATH || value.startsWith(`${BASE_PATH}/`)) continue;
		console.error(`${file}: root-relative path "${value}" is missing the "${BASE_PATH}" base path`);
		failed = true;
	}
}
if (checked === 0) throw new Error('no src/href attributes found under out/ - did the build run first?');
if (failed) process.exit(1);
console.log(`base-path audit: ${checked} root-relative src/href attributes all carry "${BASE_PATH}"`);
