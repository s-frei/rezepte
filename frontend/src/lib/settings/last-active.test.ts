import { describe, expect, it } from 'vitest';
import { lastActive } from './last-active';

const now = new Date(2026, 10, 6, 9, 30);
const daysAgo = (days: number, hour = 12) => new Date(2026, 10, 6 - days, hour).toISOString();

describe('lastActive', () => {
	it('counts calendar days, not 24-hour spans', () => {
		expect(lastActive(new Date(2026, 10, 6, 0, 5).toISOString(), now).text).toBe('Active today');
		expect(lastActive(daysAgo(1, 23), now).text).toBe('Active yesterday');
	});

	it('reads a date ahead of now as today', () => {
		expect(lastActive(daysAgo(-1), now).text).toBe('Active today');
	});

	it('says days up to six, then weeks below 60 days, then months', () => {
		expect(lastActive(daysAgo(3), now).text).toBe('Active 3 days ago');
		expect(lastActive(daysAgo(6), now).text).toBe('Active 6 days ago');
		expect(lastActive(daysAgo(7), now).text).toBe('Active 1 week ago');
		expect(lastActive(daysAgo(21), now).text).toBe('Active 3 weeks ago');
		expect(lastActive(daysAgo(59), now).text).toBe('Active 8 weeks ago');
		expect(lastActive(daysAgo(60), now).text).toBe('Active 2 months ago');
		expect(lastActive(daysAgo(125), now).text).toBe('Active 4 months ago');
	});

	it('calls 30 days or more dormant', () => {
		expect(lastActive(daysAgo(29), now).dormant).toBe(false);
		expect(lastActive(daysAgo(30), now).dormant).toBe(true);
	});

	it('says when nothing is recorded, without calling it dormant', () => {
		expect(lastActive(null, now)).toEqual({
			text: 'No activity recorded yet',
			dormant: false,
			state: 'none'
		});
	});

	it('marks below 7 days recent, up to 29 quiet, then dormant', () => {
		expect(lastActive(daysAgo(6), now).state).toBe('recent');
		expect(lastActive(daysAgo(7), now).state).toBe('quiet');
		expect(lastActive(daysAgo(29), now).state).toBe('quiet');
		expect(lastActive(daysAgo(30), now).state).toBe('dormant');
	});
});
