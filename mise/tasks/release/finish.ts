#!/usr/bin/env bun
//MISE description="Finish a release: check it, fast-forward develop and main, tag it - locally, nothing is pushed"
//USAGE flag "--dry-run" help="Run every check except mise run check, list the git commands and the push; change nothing"
// Runs inside the release worktree that release:start opened. See
// docs/memory/content/howtos/release.mdx.
import { CHANGELOG_DIR, checkPage, parseFrontmatter } from '../../lib/release/changelog';
import { env, fail, git, gitOk, lastTag, mainBlocker, mainCheckout, remoteBlocker, subjectsSince, tagBlocker } from '../../lib/release/git';
import { finishSteps, formatStep, parseArgs } from '../../lib/release/steps';
import { classificationBase, classify, parseVersion } from '../../lib/release/version';

let dryRun = false;
try {
	const args = parseArgs(Bun.argv.slice(2));
	if (args.version) fail('release:finish takes no version - it reads it from the release branch');
	dryRun = args.dryRun;
} catch (e) {
	fail((e as Error).message);
}

// mise runs a root file task from the root of the worktree it was called in.
const here = process.cwd();
const branch = git(['branch', '--show-current'], here);
const version = branch.startsWith('release/') ? branch.slice('release/'.length) : '';
if (!parseVersion(version)) fail(`run this in a release worktree; ${here} is on ${branch || 'a detached HEAD'}`);
if (git(['status', '--porcelain'], here) !== '') fail('the release worktree has uncommitted changes');

const root = mainCheckout(here);
if (root === here) fail('run this in the release worktree, not the main checkout');
if (git(['branch', '--show-current'], root) !== 'develop') fail(`${root} is not on develop`);
if (git(['status', '--porcelain'], root) !== '') fail(`${root} has uncommitted changes`);

// 1. The page is finished.
const pagePath = `${CHANGELOG_DIR}/${version}.mdx`;
const page = Bun.file(pagePath);
if (!(await page.exists())) fail(`${pagePath} does not exist`);
const src = await page.text();
// Counted from the last release, so a breaking change that shipped in a
// release candidate still demands the final release's upgrade notice.
const base = classificationBase(version, lastTag(here), lastTag(here, 'HEAD', true));
const hasBreaking = classify(subjectsSince(base, 'HEAD', here)).breaking.length > 0;
const problems = checkPage(src, hasBreaking);
if (problems.length) fail(`${pagePath} is not ready:\n  - ${problems.join('\n  - ')}`);
const fm = parseFrontmatter(src);
if ('error' in fm) fail(fm.error); // checkPage already reported it; narrows the type
const description = fm.data.description ?? '';

// 2. Everything that would stop the steps after the gate, checked before
//    it, so a doomed release fails in a second rather than after minutes -
//    and never after develop has already moved. Run again right before
//    step 4, because the gate takes long enough for main to get checked out
//    or for origin to move.
function blocker(): string | null {
	if (!gitOk(['merge-base', '--is-ancestor', 'develop', branch], root)) {
		return `develop has moved on since ${branch} started. In the release worktree:
  git rebase develop && mise run release:finish`;
	}
	gitOk(['fetch', '--quiet', 'origin'], root);
	return tagBlocker(root, version) ?? mainBlocker(root, branch) ?? remoteBlocker(root, branch);
}
const before = blocker();
if (before) fail(before);

const steps = finishSteps({ root, here, branch, version, description });
// --atomic: all three refs or none. Without it a rejected develop would still
// let main and the tag through, and the tag starts the release workflow.
const push = `git push --atomic origin develop main ${version}`;

if (dryRun) {
	console.log(`
${pagePath} is ready and nothing blocks ${version}.
Skipped in a dry run: mise run check, and the same checks again after it.

release:finish would run, in ${root}:
${steps.map((st) => `  ${formatStep(st)}`).join('\n')}

and then leave the publishing to you:
  ${push}
`);
	process.exit(0);
}

// 3. The gate is green - and nothing changed while it ran.
const check = Bun.spawnSync(['mise', 'run', 'check'], { cwd: here, env, stdio: ['inherit', 'inherit', 'inherit'] });
if (check.exitCode !== 0) fail('mise run check failed');
const after = blocker();
if (after) fail(after);

// 4. Fast-forward develop and main, tag, clean up. A step that fails stops
//    the rest; the message says what already happened and what is left.
for (const [i, st] of steps.entries()) {
	try {
		git(st.args, st.cwd);
	} catch (e) {
		const done = steps.slice(0, i).map((d) => `  ${formatStep(d)}`);
		const left = steps.slice(i).map((d) => `  ${formatStep(d)}`);
		fail(`${(e as Error).message}
${done.length ? `Already done:\n${done.join('\n')}\n` : 'Nothing has changed yet.\n'}Left to do, in ${root}, once the cause is fixed:
${left.join('\n')}`);
	}
}

console.log(`
${version} is tagged on main, locally. Publish it with:

  ${push}
`);
