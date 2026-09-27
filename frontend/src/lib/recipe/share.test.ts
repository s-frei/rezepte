import { describe, expect, it } from 'vitest';
import { isStale, renewDelay } from './share';

const now = Date.parse('2026-09-27T12:00:00Z');
const inMinutes = (minutes: number) => new Date(now + minutes * 60_000).toISOString();

describe('renewDelay', () => {
	it('renews nothing without an expiry', () => {
		expect(renewDelay(null, now)).toBeNull();
	});

	it('renews two minutes before a 15-minute link runs out', () => {
		expect(renewDelay(inMinutes(15), now)).toBe(13 * 60_000);
	});

	it('waits at least a minute', () => {
		expect(renewDelay(new Date(now + 30_000).toISOString(), now)).toBe(60_000);
	});
});

describe('isStale', () => {
	it('is stale with less than two minutes left', () => {
		expect(isStale(new Date(now + 119_000).toISOString(), now)).toBe(true);
	});

	it('is stale once expired', () => {
		expect(isStale(inMinutes(-1), now)).toBe(true);
	});

	it('is fresh with five minutes left', () => {
		expect(isStale(inMinutes(5), now)).toBe(false);
	});

	it('never goes stale without an expiry', () => {
		expect(isStale(null, now)).toBe(false);
	});
});
