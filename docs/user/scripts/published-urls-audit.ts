// Fails the build when an address someone outside this site links to is gone
// from the export. They are linked from places a docs change never touches:
// every release's page and its Markdown view, named in the GitHub release
// notes (mise/lib/release/changelog.ts); the docs pages the app links to
// (frontend/src/lib/docs.ts); and every docs link in README.md and
// CONTRIBUTING.md, anchors included, since GitHub shows those to everyone who
// finds the project. Moving content around is free as long as these stay put;
// see docs/memory/content/architecture/docs.mdx.
import { existsSync, readdirSync, readFileSync } from 'node:fs';
import path from 'node:path';

const SITE = 'https://s-frei.github.io/rezepte/';
const CHANGELOG = 'content/(releases)/changelog';

if (!existsSync('out')) throw new Error('out/ does not exist - run `mise run //docs/user:build` first');

// Paths below the site root, as the export lays them out (trailingSlash: true).
const required = new Map<string, string>();
for (const file of readdirSync(CHANGELOG).filter((f) => /^v.+\.mdx$/.test(f))) {
	const version = file.slice(0, -'.mdx'.length);
	required.set(`changelog/${version}/index.html`, `release ${version}`);
	required.set(`llms.mdx/changelog/${version}/content.md`, `release ${version}, Markdown view`);
}
// A link's anchor, if it has one, must be a heading id on that page.
const anchors = new Map<string, string>();
const site = SITE.replaceAll('.', '\\.');
const appLinks = readFileSync(path.join('..', '..', 'frontend', 'src', 'lib', 'docs.ts'), 'utf8');
for (const [, rest] of appLinks.matchAll(new RegExp(`'${site}([^'#]*)'`, 'g'))) {
	required.set(`${rest}index.html`, 'linked from frontend/src/lib/docs.ts');
}
for (const doc of ['README.md', 'CONTRIBUTING.md']) {
	const text = readFileSync(path.join('..', '..', doc), 'utf8');
	for (const [, rest, anchor] of text.matchAll(new RegExp(`${site}([^\\s)#\\]]*)(?:#([\\w-]+))?`, 'g'))) {
		// A file such as llms.txt is linked as itself; a page by its folder.
		const file = /\.\w+$/.test(rest) ? rest : `${rest}index.html`;
		required.set(file, `linked from ${doc}`);
		if (anchor) anchors.set(`${file}#${anchor}`, `linked from ${doc}`);
	}
}

let failed = false;
for (const [file, why] of required) {
	if (existsSync(path.join('out', file))) continue;
	console.error(`${SITE}${file.replace(/index\.html$/, '')} (${why}) is missing from the export`);
	failed = true;
}
for (const [link, why] of anchors) {
	const [file, anchor] = link.split('#');
	const html = path.join('out', file);
	if (!existsSync(html) || readFileSync(html, 'utf8').includes(`id="${anchor}"`)) continue;
	console.error(`${SITE}${file.replace(/index\.html$/, '')}#${anchor} (${why}): the page has no such heading`);
	failed = true;
}
if (failed) process.exit(1);
console.log(`published-urls audit: ${required.size} linked addresses and ${anchors.size} anchors all exported`);
