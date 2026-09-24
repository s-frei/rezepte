// The few git calls the release tasks make. Kept apart from version.ts and
// changelog.ts so those stay pure and testable without a repository.
import path from 'node:path';

export function fail(message: string): never {
	console.error(`release: ${message}`);
	process.exit(1);
}

// Every call names its repository by cwd. A GIT_DIR, GIT_WORK_TREE or
// GIT_INDEX_FILE inherited from the caller - `git rebase --exec` and git hooks
// export them - would silently point it at another one instead.
export const env = { ...process.env, GIT_DIR: undefined, GIT_WORK_TREE: undefined, GIT_INDEX_FILE: undefined };

function run(args: string[], cwd: string, stdout: 'pipe' | 'ignore', stderr: 'pipe' | 'ignore') {
	return Bun.spawnSync(['git', ...args], { cwd, env, stdout, stderr });
}

export function git(args: string[], cwd: string): string {
	const r = run(args, cwd, 'pipe', 'pipe');
	if (r.exitCode !== 0) throw new Error(`git ${args.join(' ')}: ${r.stderr.toString().trim()}`);
	return r.stdout.toString().trim();
}

export function gitOk(args: string[], cwd: string): boolean {
	return run(args, cwd, 'ignore', 'ignore').exitCode === 0;
}

// The main checkout, wherever this runs from: --git-common-dir always points
// at its .git, the same trick mise/tasks/worktree/new.sh uses.
export function mainCheckout(cwd: string): string {
	return path.dirname(path.resolve(cwd, git(['rev-parse', '--git-common-dir'], cwd)));
}

// The nearest version tag, or with releasesOnly the nearest that is not a
// prerelease (no hyphen).
export function lastTag(cwd: string, ref = 'HEAD', releasesOnly = false): string | null {
	const exclude = releasesOnly ? ['--exclude', 'v*-*'] : [];
	const r = run(['describe', '--tags', '--abbrev=0', '--match', 'v[0-9]*.[0-9]*.[0-9]*', ...exclude, ref], cwd, 'pipe', 'ignore');
	return r.exitCode === 0 ? r.stdout.toString().trim() : null;
}

export function subjectsSince(tag: string | null, ref: string, cwd: string): string[] {
	const out = git(['log', '--format=%s', tag ? `${tag}..${ref}` : ref], cwd);
	return out ? out.split('\n') : [];
}

// Why the printed `git push --atomic origin develop main <tag>` would be
// rejected, or null: the release has to contain everything origin already
// has on both branches. Only as fresh as the last fetch.
export function remoteBlocker(root: string, branch: string): string | null {
	const has = (ref: string) => gitOk(['rev-parse', '--verify', '--quiet', ref], root);
	const contains = (ref: string) => gitOk(['merge-base', '--is-ancestor', ref, branch], root);
	if (has('refs/remotes/origin/develop') && !contains('refs/remotes/origin/develop')) {
		return `origin/develop has commits ${branch} lacks; pull them into develop and rebase the release branch`;
	}
	if (has('refs/remotes/origin/main') && !contains('refs/remotes/origin/main')) {
		return `origin/main is not an ancestor of ${branch}; main has diverged, sort that out by hand`;
	}
	return null;
}

export function tagBlocker(root: string, version: string): string | null {
	return gitOk(['rev-parse', '--verify', '--quiet', `refs/tags/${version}`], root) ? `the tag ${version} already exists` : null;
}

// Why main cannot be fast-forwarded to the release branch, or null. Checked
// before develop moves: `git fetch . develop:main` refuses a diverged main and
// a main checked out anywhere, and failing there would leave develop carrying
// a release that main and the tag do not.
export function mainBlocker(root: string, branch: string): string | null {
	if (!gitOk(['rev-parse', '--verify', '--quiet', 'refs/heads/main'], root)) return null;
	if (!gitOk(['merge-base', '--is-ancestor', 'main', branch], root)) {
		return `local main is not an ancestor of ${branch}; main has diverged, sort that out by hand`;
	}
	let worktree = '';
	for (const line of git(['worktree', 'list', '--porcelain'], root).split('\n')) {
		if (line.startsWith('worktree ')) worktree = line.slice('worktree '.length);
		if (line === 'branch refs/heads/main') {
			return `main is checked out in ${worktree}; switch that worktree to another branch first`;
		}
	}
	return null;
}
