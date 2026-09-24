import { describe, expect, test } from 'bun:test';
import { belowSuggestion, classificationBase, classify, compareVersions, parseVersion, suggest, validate } from './version';

describe('classify', () => {
	test('sorts subjects into breaking, features and other', () => {
		const c = classify([
			'refactor(api)!: say username where a login name is meant',
			'feat!: drop the legacy import',
			'feat(service): name operations by their route',
			'feat: link ingredients from steps',
			'fix(i18n): keep <html lang> in step',
			'docs: explain releases',
			'chore(mise): move task bodies'
		]);
		expect(c.breaking).toEqual(['refactor(api)!: say username where a login name is meant', 'feat!: drop the legacy import']);
		expect(c.features).toEqual(['feat(service): name operations by their route', 'feat: link ingredients from steps']);
		expect(c.other).toEqual(['fix(i18n): keep <html lang> in step', 'docs: explain releases', 'chore(mise): move task bodies']);
	});

	test('a subject that only mentions an exclamation mark is not breaking', () => {
		expect(classify(['fix: handle "Hello!" in titles']).breaking).toEqual([]);
	});

	test('an empty range classifies to nothing', () => {
		expect(classify([])).toEqual({ breaking: [], features: [], other: [] });
	});
});

describe('parseVersion', () => {
	test('reads a release and a prerelease', () => {
		expect(parseVersion('v1.2.3')).toEqual({ major: 1, minor: 2, patch: 3 });
		expect(parseVersion('v1.2.3-rc.1')).toEqual({ major: 1, minor: 2, patch: 3, prerelease: 'rc.1' });
	});

	test('rejects anything else', () => {
		for (const bad of ['1.2.3', 'v1.2', 'v1.2.3.4', 'v01.2.3x', 'va.b.c', '']) {
			expect(parseVersion(bad)).toBeNull();
		}
	});
});

describe('compareVersions', () => {
	const v = (s: string) => parseVersion(s)!;
	test('orders by major, minor, patch', () => {
		expect(compareVersions(v('v2.0.0'), v('v1.9.9'))).toBeGreaterThan(0);
		expect(compareVersions(v('v1.2.0'), v('v1.10.0'))).toBeLessThan(0);
		expect(compareVersions(v('v1.2.3'), v('v1.2.3'))).toBe(0);
	});

	test('a prerelease comes before its release', () => {
		expect(compareVersions(v('v1.1.0-rc1'), v('v1.1.0'))).toBeLessThan(0);
		expect(compareVersions(v('v1.1.0-rc1'), v('v1.1.0-rc2'))).toBeLessThan(0);
	});
});

describe('suggest', () => {
	// suggest(nearest tag, last release tag, commits since the last release)
	const none = { breaking: [], features: [], other: [] };
	test('the first release is v1.0.0', () => {
		expect(suggest(null, null, { ...none, other: ['chore: init'] })).toBe('v1.0.0');
		expect(suggest(null, null, none)).toBe('v1.0.0');
	});

	test('breaking bumps major, feat minor, the rest patch', () => {
		expect(suggest('v1.4.2', 'v1.4.2', { ...none, breaking: ['x!: y'], features: ['feat: z'] })).toBe('v2.0.0');
		expect(suggest('v1.4.2', 'v1.4.2', { ...none, features: ['feat: z'], other: ['fix: a'] })).toBe('v1.5.0');
		expect(suggest('v1.4.2', 'v1.4.2', { ...none, other: ['fix: a'] })).toBe('v1.4.3');
	});

	test('after a prerelease the suggestion is the release it previews', () => {
		expect(suggest('v1.1.0-rc1', 'v1.0.0', { ...none, features: ['feat: a'], other: ['fix: b'] })).toBe('v1.1.0');
		expect(suggest('v2.0.0-rc1', 'v1.4.2', { ...none, breaking: ['x!: y'] })).toBe('v2.0.0');
		expect(suggest('v1.0.0-rc1', null, { ...none, other: ['chore: init'] })).toBe('v1.0.0');
	});

	test('a release candidate with no commits after it can still be promoted', () => {
		expect(suggest('v1.1.0-rc1', 'v1.0.0', { ...none, features: ['feat: in the candidate'] })).toBe('v1.1.0');
	});

	test('a change after the candidate that needs a bigger step outranks it', () => {
		expect(suggest('v1.5.0-rc1', 'v1.4.0', { ...none, breaking: ['feat!: drop the old API'], features: ['feat: x'] })).toBe(
			'v2.0.0'
		);
		expect(suggest('v1.4.2-rc1', 'v1.4.1', { ...none, features: ['feat: x'] })).toBe('v1.5.0');
	});

	test('nothing since the last tag is an error', () => {
		expect(() => suggest('v1.0.0', 'v1.0.0', none)).toThrow('nothing to release since v1.0.0');
	});
});

describe('classificationBase', () => {
	test('a release covers everything since the last release, release candidates included', () => {
		expect(classificationBase('v2.0.0', 'v2.0.0-rc1', 'v1.4.2')).toBe('v1.4.2');
	});

	test('a release candidate covers only what came since the nearest tag', () => {
		expect(classificationBase('v2.0.0-rc2', 'v2.0.0-rc1', 'v1.4.2')).toBe('v2.0.0-rc1');
	});

	test('without any release the whole history counts', () => {
		expect(classificationBase('v1.0.0', 'v1.0.0-rc1', null)).toBeNull();
		expect(classificationBase('v1.0.0', null, null)).toBeNull();
	});
});

describe('belowSuggestion', () => {
	test('warns when the chosen version is a smaller step than the commits ask for', () => {
		expect(belowSuggestion('v1.5.0', 'v2.0.0')).toBe(
			'v1.5.0 is a smaller step than the commits since the last release ask for (v2.0.0) - readers going by semver will expect less change than it carries'
		);
	});

	test('says nothing when the chosen version matches or goes further', () => {
		expect(belowSuggestion('v2.0.0', 'v2.0.0')).toBeNull();
		expect(belowSuggestion('v3.0.0', 'v2.0.0')).toBeNull();
	});

	test('a candidate for the suggested release is not a smaller step', () => {
		expect(belowSuggestion('v2.0.0-rc1', 'v2.0.0')).toBeNull();
	});
});

describe('validate', () => {
	test('accepts a greater version', () => {
		expect(validate('v1.0.0', null)).toBeNull();
		expect(validate('v1.1.0', 'v1.0.0')).toBeNull();
		expect(validate('v1.1.0-rc1', 'v1.0.0')).toBeNull();
	});

	test('rejects a malformed version', () => {
		expect(validate('1.1.0', 'v1.0.0')).toBe('"1.1.0" is not a version like v1.2.3 or v1.2.3-rc1');
	});

	test('rejects a version that is not greater than the last tag', () => {
		expect(validate('v1.0.0', 'v1.0.0')).toBe('v1.0.0 is not greater than the last release v1.0.0');
		expect(validate('v0.9.0', 'v1.0.0')).toBe('v0.9.0 is not greater than the last release v1.0.0');
	});
});
