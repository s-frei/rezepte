import { expect, test } from 'bun:test';
import { isFixup, violations } from './commits';

test('accepts the documented shapes', () => {
	expect(violations('feat(editor): add a unit picker')).toEqual([]);
	expect(violations('build: check commit messages')).toEqual([]);
	expect(violations('ci(release): pin the runner\n\nWhy it matters.')).toEqual([]);
	expect(violations('refactor(api)!: say username\n\nBreaking: clients must send `username`.')).toEqual([]);
});

test('rejects an unknown type or a missing prefix', () => {
	expect(violations('feature: add it')).toHaveLength(1);
	expect(violations('Add it')).toHaveLength(1);
	expect(violations('fix(ui):no space')).toHaveLength(1);
});

test('rejects a title over 70 characters', () => {
	expect(violations(`fix: ${'x'.repeat(66)}`)[0]).toContain('71 characters');
	expect(violations(`fix: ${'x'.repeat(65)}`)).toEqual([]);
});

test('a breaking change needs its Breaking line', () => {
	expect(violations('feat!: drop the flag')).toHaveLength(1);
	expect(violations('feat!: drop the flag\n\nBreaking:no space')).toHaveLength(1);
});

test('rejects tool trailers', () => {
	expect(violations('fix: x\n\nCo-Authored-By: Bot <b@b>')).toHaveLength(1);
	expect(violations('fix: x\n\n🤖 Generated with [Claude Code](https://claude.com)')).toHaveLength(1);
	expect(violations('fix: x\n\nThe output is regenerated with sqlc.')).toEqual([]);
});

test('recognizes fixups', () => {
	expect(isFixup('fixup! feat: x')).toBe(true);
	expect(isFixup('feat: fixup! x')).toBe(false);
});
