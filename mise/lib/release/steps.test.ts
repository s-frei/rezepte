import { describe, expect, test } from 'bun:test';
import { finishSteps, formatStep, parseArgs } from './steps';

describe('parseArgs', () => {
	test('reads the flag and the version in any order', () => {
		expect(parseArgs(['--dry-run', 'v1.2.0'])).toEqual({ dryRun: true, version: 'v1.2.0' });
		expect(parseArgs(['v1.2.0', '--dry-run'])).toEqual({ dryRun: true, version: 'v1.2.0' });
	});

	test('both are optional', () => {
		expect(parseArgs([])).toEqual({ dryRun: false, version: undefined });
	});

	test('an unknown flag is an error, not a version', () => {
		expect(() => parseArgs(['--dryrun'])).toThrow('unknown option --dryrun');
	});
});

describe('finishSteps', () => {
	const steps = finishSteps({
		root: '/repo',
		here: '/repo/.claude/worktrees/release-v2.0.0',
		branch: 'release/v2.0.0',
		version: 'v2.0.0',
		description: 'The old API is gone.'
	});

	test('fast-forwards develop, then main, tags, then cleans up - all in the main checkout', () => {
		expect(steps.map((s) => s.args)).toEqual([
			['merge', '--ff-only', '--quiet', 'release/v2.0.0'],
			['fetch', '.', 'develop:main'],
			['tag', '-a', 'v2.0.0', '-m', 'The old API is gone.', 'main'],
			['worktree', 'remove', '/repo/.claude/worktrees/release-v2.0.0'],
			['branch', '-d', 'release/v2.0.0']
		]);
		expect(new Set(steps.map((s) => s.cwd))).toEqual(new Set(['/repo']));
	});

	test('prints as a git command a person could run', () => {
		expect(formatStep(steps[2])).toBe("git tag -a v2.0.0 -m 'The old API is gone.' main");
		expect(formatStep(steps[0])).toBe('git merge --ff-only --quiet release/v2.0.0');
	});

	test('quotes what a shell would otherwise expand', () => {
		const [, , tag] = finishSteps({ root: '/r', here: '/r/w', branch: 'release/v1.0.0', version: 'v1.0.0', description: "It's $HOME `x`" });
		expect(formatStep(tag)).toBe(`git tag -a v1.0.0 -m 'It'\\''s $HOME \`x\`' main`);
	});
});
