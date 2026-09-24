import { afterEach, beforeEach, describe, expect, test } from 'bun:test';
import { mkdtempSync, realpathSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { mainBlocker, remoteBlocker, tagBlocker } from './git';

// A throwaway repository per test: develop with two commits and a release
// branch on top, the shape release:finish sees.
let dir: string;

// An inherited GIT_DIR (a git hook running `mise run check`) would point these
// commands at the real repository instead of the throwaway one.
const env = { ...process.env, GIT_DIR: undefined, GIT_INDEX_FILE: undefined, GIT_WORK_TREE: undefined };

function sh(...args: string[]) {
	const r = Bun.spawnSync(['git', '-c', 'commit.gpgsign=false', '-c', 'tag.gpgsign=false', '-c', 'user.name=t', '-c', 'user.email=t@t', ...args], {
		cwd: dir,
		env,
		stdout: 'pipe',
		stderr: 'pipe'
	});
	if (r.exitCode !== 0) throw new Error(r.stderr.toString());
}

beforeEach(() => {
	// realpath: macOS hands out /var/..., which git reports as /private/var/...
	dir = realpathSync(mkdtempSync(path.join(tmpdir(), 'release-git-')));
	sh('init', '-q', '-b', 'develop');
	sh('commit', '-q', '--allow-empty', '-m', 'one');
	sh('commit', '-q', '--allow-empty', '-m', 'two');
	sh('branch', 'release/v1.0.0');
});

afterEach(() => rmSync(dir, { recursive: true, force: true }));

describe('mainBlocker', () => {
	test('no local main is fine - release:finish creates it', () => {
		expect(mainBlocker(dir, 'release/v1.0.0')).toBeNull();
	});

	test('a main behind the release is fine', () => {
		sh('branch', 'main', 'HEAD~1');
		expect(mainBlocker(dir, 'release/v1.0.0')).toBeNull();
	});

	test('a main with commits of its own blocks', () => {
		sh('checkout', '-q', '-b', 'main', 'HEAD~1');
		sh('commit', '-q', '--allow-empty', '-m', 'hotfix');
		sh('checkout', '-q', 'develop');
		expect(mainBlocker(dir, 'release/v1.0.0')).toBe('local main is not an ancestor of release/v1.0.0; main has diverged, sort that out by hand');
	});

	test('a main checked out in a worktree blocks', () => {
		sh('branch', 'main', 'HEAD~1');
		sh('worktree', 'add', '-q', path.join(dir, 'wt'), 'main');
		expect(mainBlocker(dir, 'release/v1.0.0')).toBe(
			`main is checked out in ${path.join(dir, 'wt')}; switch that worktree to another branch first`
		);
	});
});

describe('an inherited git environment', () => {
	// `git rebase --exec` and git hooks export GIT_DIR to everything they start;
	// the release tasks must still work on the repository they are pointed at.
	// A child process, because that is how the variable arrives in real use.
	test('does not redirect the calls away from the given directory', () => {
		sh('checkout', '-q', '-b', 'main', 'HEAD~1');
		sh('commit', '-q', '--allow-empty', '-m', 'hotfix');
		sh('checkout', '-q', 'develop');
		const script = `import { mainBlocker } from ${JSON.stringify(path.join(import.meta.dir, 'git.ts'))};
console.log(mainBlocker(${JSON.stringify(dir)}, 'release/v1.0.0'));`;
		const r = Bun.spawnSync([process.execPath, '-e', script], {
			env: { ...process.env, GIT_DIR: path.join(tmpdir(), 'not-a-repository'), GIT_WORK_TREE: tmpdir() },
			stdout: 'pipe',
			stderr: 'pipe'
		});
		expect(r.stdout.toString().trim()).toBe(
			'local main is not an ancestor of release/v1.0.0; main has diverged, sort that out by hand'
		);
	});
});

describe('remoteBlocker', () => {
	test('no remote-tracking branches is fine', () => {
		expect(remoteBlocker(dir, 'release/v1.0.0')).toBeNull();
	});

	test('remote branches the release already contains are fine', () => {
		sh('update-ref', 'refs/remotes/origin/develop', 'HEAD');
		sh('update-ref', 'refs/remotes/origin/main', 'HEAD~1');
		expect(remoteBlocker(dir, 'release/v1.0.0')).toBeNull();
	});

	test('a commit on origin/develop the release lacks blocks', () => {
		sh('checkout', '-q', '-b', 'elsewhere');
		sh('commit', '-q', '--allow-empty', '-m', 'pushed from another machine');
		sh('update-ref', 'refs/remotes/origin/develop', 'HEAD');
		sh('checkout', '-q', 'develop');
		expect(remoteBlocker(dir, 'release/v1.0.0')).toBe(
			'origin/develop has commits release/v1.0.0 lacks; pull them into develop and rebase the release branch'
		);
	});

	test('a diverged origin/main blocks', () => {
		sh('checkout', '-q', '-b', 'elsewhere', 'HEAD~1');
		sh('commit', '-q', '--allow-empty', '-m', 'hotfix on main');
		sh('update-ref', 'refs/remotes/origin/main', 'HEAD');
		sh('checkout', '-q', 'develop');
		expect(remoteBlocker(dir, 'release/v1.0.0')).toBe(
			'origin/main is not an ancestor of release/v1.0.0; main has diverged, sort that out by hand'
		);
	});
});

describe('tagBlocker', () => {
	test('a free tag is fine', () => {
		expect(tagBlocker(dir, 'v1.0.0')).toBeNull();
	});

	test('an existing tag blocks', () => {
		sh('tag', 'v1.0.0', 'HEAD~1');
		expect(tagBlocker(dir, 'v1.0.0')).toBe('the tag v1.0.0 already exists');
	});
});
