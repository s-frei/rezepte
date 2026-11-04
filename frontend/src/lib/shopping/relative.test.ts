import { describe, expect, it } from 'vitest';
import { relativeTime } from './relative';

describe('relativeTime', () => {
	const now = Date.parse('2026-11-03T12:00:00Z');
	it('picks the largest whole unit', () => {
		expect(relativeTime('2026-11-03T11:55:00Z', now)).toMatch(/5/);
		expect(relativeTime('2026-11-01T12:00:00Z', now)).toMatch(/2/);
	});
	it('never breaks between number and unit', () => {
		expect(relativeTime('2026-11-03T11:55:00Z', now)).not.toContain(' ');
	});
	it('calls under a minute now', () => {
		expect(relativeTime('2026-11-03T11:59:50Z', now)).toBe('now');
	});
});
