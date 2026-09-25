#!/usr/bin/env bun
//MISE description="Start a release: suggest the version, open release/vX.Y.Z in a worktree, draft its changelog page"
//MISE raw=true
//USAGE arg "[version]" help="The version to release, like v1.2.0 or v1.2.0-rc1; without it the task suggests one and asks"
//USAGE flag "--dry-run" help="Show the suggestion, the branch and worktree and the draft page; create nothing"
// raw: the task asks for confirmation on stdin. See
// docs/memory/content/howtos/release.mdx.
import path from 'node:path';
import { CHANGELOG_DIR, draftPage, insertIntoMeta } from '../../lib/release/changelog';
import { env, fail, git, gitOk, lastTag, mainCheckout, subjectsSince } from '../../lib/release/git';
import { type Args, parseArgs } from '../../lib/release/steps';
import { belowSuggestion, classificationBase, classify, suggest, validate } from '../../lib/release/version';

let args: Args;
try {
	args = parseArgs(Bun.argv.slice(2));
} catch (e) {
	fail((e as Error).message);
}

const root = mainCheckout(process.cwd());
if (git(['branch', '--show-current'], root) !== 'develop') fail(`${root} is not on develop`);
if (git(['status', '--porcelain'], root) !== '') fail(`${root} has uncommitted changes`);

if (gitOk(['fetch', '--quiet', 'origin'], root) && !gitOk(['merge-base', '--is-ancestor', 'origin/develop', 'develop'], root)) {
	console.warn('release: develop is behind origin/develop - pull first unless that is intended');
}

// The nearest tag may be a release candidate; the version is suggested from
// everything since the last actual release, so a breaking change that landed
// after a candidate still asks for a major step.
const tag = lastTag(root, 'develop');
const release = lastTag(root, 'develop', true);
const sinceRelease = classify(subjectsSince(release, 'develop', root));
console.log(`Last release: ${release ?? 'none'}${tag !== release ? ` (nearest tag ${tag})` : ''}`);
console.log(
	`Since then:   ${sinceRelease.breaking.length} breaking, ${sinceRelease.features.length} feat, ${sinceRelease.other.length} other`
);
for (const s of sinceRelease.breaking) console.log(`  ! ${s}`);

let suggestion: string;
try {
	suggestion = suggest(tag, release, sinceRelease);
} catch (e) {
	fail((e as Error).message);
}

// A dry run takes the suggestion rather than asking.
let version = args.version;
if (!version && !args.dryRun) {
	const answer = prompt(`Version [${suggestion}]:`)?.trim();
	version = answer || suggestion;
}
version ??= suggestion;
const invalid = validate(version, tag);
if (invalid) fail(invalid);
const smaller = belowSuggestion(version, suggestion);
if (smaller) console.warn(`release: warning: ${smaller}`);

const name = `release-${version}`;
const branch = `release/${version}`;
const dir = path.join(root, '.claude', 'worktrees', name);

// A release's page covers everything since the last release, candidates
// included; a candidate's page only what came since the nearest tag.
const base = classificationBase(version, tag, release);
const covered = base === release ? sinceRelease : classify(subjectsSince(base, 'develop', root));
const today = new Date().toISOString().slice(0, 10);
const draft = draftPage(version, today, base, covered);

if (args.dryRun) {
	const taken = gitOk(['rev-parse', '--verify', '--quiet', `refs/heads/${branch}`], root)
		? `\n  but the branch ${branch} already exists - the real run would stop here`
		: '';
	console.log(`
Dry run - nothing was created. release:start would:
  open ${dir}
  on a new branch ${branch} off develop${taken}
  run mise run setup there
  add ${version} to ${CHANGELOG_DIR}/meta.json
  write ${CHANGELOG_DIR}/${version}.mdx:

${draft}`);
	process.exit(0);
}

const created = Bun.spawnSync(['mise', 'run', 'worktree:new', name, branch], { cwd: root, env, stdio: ['inherit', 'inherit', 'inherit'] });
if (created.exitCode !== 0) fail('worktree:new failed');
// A fresh worktree has no node_modules; release:finish runs the full gate there.
const setup = Bun.spawnSync(['mise', 'run', 'setup'], { cwd: dir, env, stdio: ['inherit', 'inherit', 'inherit'] });
if (setup.exitCode !== 0) fail(`mise run setup failed in ${dir}`);

await Bun.write(path.join(dir, CHANGELOG_DIR, `${version}.mdx`), draft);
const metaPath = path.join(dir, CHANGELOG_DIR, 'meta.json');
await Bun.write(metaPath, insertIntoMeta(await Bun.file(metaPath).text(), version));

// The main checkout stays on develop on purpose: release:finish fast-forwards
// it there. Say so, or the release looks like it never started.
console.log(`
Release ${version} is open in its own worktree:
  ${path.relative(root, dir)}   (branch ${branch})
This checkout stays on develop - open that folder to work on the release.

Next, in that worktree:
  1. Write ${CHANGELOG_DIR}/${version}.mdx - highlights, the rest, and
     the upgrade notice if anything is breaking. Remove the DRAFT line.
  2. Commit it:  docs(changelog): write the ${version} release notes
  3. Run:  mise run release:finish
`);
