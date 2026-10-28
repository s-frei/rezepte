import { describe, expect, it } from 'vitest';
import { launchFilter } from './launch';

describe('launchFilter', () => {
	it('the launch that loaded the page is not fresh', () => {
		const fresh = launchFilter('https://h/import?url=a');
		expect(fresh('https://h/import?url=a')).toBe(false);
	});
	it('a later launch is fresh', () => {
		const fresh = launchFilter('https://h/import?url=a');
		expect(fresh('https://h/import?url=b')).toBe(true);
	});
	it('sharing the same link again later is fresh', () => {
		const fresh = launchFilter('https://h/import?url=a');
		expect(fresh('https://h/import?url=a')).toBe(false);
		expect(fresh('https://h/import?url=a')).toBe(true);
	});
});
