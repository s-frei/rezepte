import { expect, test } from 'bun:test';
import { flaky } from './flaky-summary';

test('lists flaky tests from nested suites and skips the rest', () => {
	const spec = (title: string, status: string) => ({
		file: 'a.test.ts',
		line: 3,
		title,
		tests: [{ status, projectName: 'desktop' }]
	});
	const suites = [
		{
			specs: [spec('steady', 'expected')],
			suites: [{ specs: [spec('wobbly', 'flaky'), spec('broken', 'unexpected')] }]
		}
	];
	expect(flaky(suites)).toEqual(['a.test.ts:3 › wobbly [desktop]']);
});
