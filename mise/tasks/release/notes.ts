#!/usr/bin/env bun
//MISE description="Print the GitHub release body for a version from its changelog page"
//USAGE arg "<version>" help="The released version, like v1.2.0; its page is docs/user/content/changelog/<version>.mdx"
//USAGE flag "--url" help="Print only the URL of the version's changelog page"
// release:publish writes this to the notes file of `gh release create`. See
// docs/memory/content/architecture/releases.mdx.
import { CHANGELOG_DIR, pageUrl, parseFrontmatter, releaseBody } from '../../lib/release/changelog';
import { fail } from '../../lib/release/git';
import { parseVersion } from '../../lib/release/version';

const argv = Bun.argv.slice(2);
const version = argv.find((a) => !a.startsWith('-'));
const unknown = argv.find((a) => a.startsWith('-') && a !== '--url');
if (unknown || !version || !parseVersion(version)) fail('usage: mise run release:notes vX.Y.Z [--url]');
if (argv.includes('--url')) {
	console.log(pageUrl(version));
	process.exit(0);
}

// mise runs a root file task from the repository root.
const file = Bun.file(`${CHANGELOG_DIR}/${version}.mdx`);
if (!(await file.exists())) fail(`${CHANGELOG_DIR}/${version}.mdx does not exist`);
const fm = parseFrontmatter(await file.text());
if ('error' in fm) fail(`${version}.mdx: ${fm.error}`);
if (!fm.data.description) fail(`${version}.mdx has no description`);
process.stdout.write(releaseBody(version, fm.data));
