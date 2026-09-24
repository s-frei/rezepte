// The command-line arguments of the release tasks, and the git steps
// release:finish takes once every check has passed. Kept as data so that
// --dry-run prints exactly the list the real run executes.

export type Args = { dryRun: boolean; version: string | undefined };

export function parseArgs(argv: string[]): Args {
	const args: Args = { dryRun: false, version: undefined };
	for (const a of argv) {
		if (a === '--dry-run') args.dryRun = true;
		else if (a.startsWith('-')) throw new Error(`unknown option ${a}`);
		else args.version = a;
	}
	return args;
}

export type Step = { args: string[]; cwd: string };

export function finishSteps(r: { root: string; here: string; branch: string; version: string; description: string }): Step[] {
	return [
		// develop, then main, both fast-forward only; fetch updates main without
		// checking it out and refuses anything but a fast-forward.
		{ args: ['merge', '--ff-only', '--quiet', r.branch], cwd: r.root },
		{ args: ['fetch', '.', 'develop:main'], cwd: r.root },
		{ args: ['tag', '-a', r.version, '-m', r.description, 'main'], cwd: r.root },
		{ args: ['worktree', 'remove', r.here], cwd: r.root },
		{ args: ['branch', '-d', r.branch], cwd: r.root }
	];
}

// Single quotes keep $, backticks and spaces literal; a single quote itself
// closes, escapes and reopens.
const quote = (s: string) => (/^[\w@%+=:,./-]+$/.test(s) ? s : `'${s.replaceAll("'", "'\\''")}'`);

export function formatStep(step: Step): string {
	return ['git', ...step.args].map(quote).join(' ');
}
