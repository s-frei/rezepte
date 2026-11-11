#!/usr/bin/env bun
//MISE description="Fail when a commit on this branch that develop lacks breaks the commit message rules"
// Only develop..HEAD: develop is never rewritten, so a violation that already
// landed stays and must not fail every later check. See
// docs/memory/content/conventions/commits.mdx.
import { env, git, gitOk } from '../../lib/release/git';
import { isExempt, violations } from '../../lib/guards/commits';

const cwd = process.cwd();
const skip = (why: string) => {
	console.log(`commits:check: skipped, ${why}`);
	process.exit(0);
};

if (git(['branch', '--show-current'], cwd) === 'develop') skip('on develop itself');
if (!gitOk(['rev-parse', '--verify', '--quiet', 'develop^{commit}'], cwd)) skip('there is no develop ref here');
// A shallow clone (CI checks out one commit) cuts the range at its graft, so
// develop..HEAD would list commits that are long on develop.
if (git(['rev-parse', '--is-shallow-repository'], cwd) === 'true') skip('the clone is shallow');

const log = Bun.spawnSync(['git', 'log', '--no-merges', '--format=%h%x00%B%x00', 'develop..HEAD'], { cwd, env }).stdout.toString();
const parts = log.split('\0');
let checked = 0;
let failed = 0;
for (let i = 0; i + 1 < parts.length; i += 2) {
	const sha = parts[i].trim();
	const message = parts[i + 1];
	if (isExempt(message)) continue;
	checked++;
	for (const v of violations(message)) {
		console.error(`commits:check: ${sha} ${message.split('\n')[0]}\n  ${v}`);
		failed++;
	}
}
if (failed) process.exit(1);
console.log(`commits:check: ${checked} commit(s) on this branch follow the rules`);
