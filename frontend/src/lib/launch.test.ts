import { describe, expect, it } from 'vitest';
import { isFreshLaunch } from './launch';

describe('isFreshLaunch', () => {
	it('the launch that loaded the page is not fresh', () => {
		expect(isFreshLaunch('https://h/import?url=a', 'https://h/import?url=a')).toBe(false);
	});
	it('a later launch is fresh', () => {
		expect(isFreshLaunch('https://h/import?url=b', 'https://h/import?url=a')).toBe(true);
	});
});
