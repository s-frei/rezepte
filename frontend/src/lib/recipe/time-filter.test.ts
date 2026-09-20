import { describe, expect, it } from 'vitest';
import { TIME_STOPS, snapMaxMinutes, timeStopIndex, timeStopMinutes } from './time-filter';

describe('TIME_STOPS', () => {
	it('ends on the off stop', () => {
		expect(TIME_STOPS[TIME_STOPS.length - 1]).toBe(0);
	});

	it('rises to the last limit', () => {
		const limits = TIME_STOPS.slice(0, -1);
		expect(limits).toEqual([...limits].sort((a, b) => a - b));
	});
});

describe('snapMaxMinutes', () => {
	it('keeps a value that is already a stop', () => {
		for (const stop of TIME_STOPS) {
			expect(snapMaxMinutes(stop)).toBe(stop);
		}
	});

	it('reads anything at or below zero as off', () => {
		expect(snapMaxMinutes(0)).toBe(0);
		expect(snapMaxMinutes(-5)).toBe(0);
	});

	it('reads a non-number as off', () => {
		expect(snapMaxMinutes(Number.NaN)).toBe(0);
		expect(snapMaxMinutes(Number.POSITIVE_INFINITY)).toBe(0);
	});

	// Upwards, never down: a bound rounded down would hide recipes the caller
	// asked to see, while rounding up only adds a few they did not.
	it('rounds a value between two stops up to the next one', () => {
		expect(snapMaxMinutes(1)).toBe(15);
		expect(snapMaxMinutes(16)).toBe(30);
		expect(snapMaxMinutes(37)).toBe(45);
		expect(snapMaxMinutes(61)).toBe(90);
		expect(snapMaxMinutes(121)).toBe(180);
	});

	it('rounds a fraction up like any other value between stops', () => {
		expect(snapMaxMinutes(37.5)).toBe(45);
		expect(snapMaxMinutes(14.5)).toBe(15);
	});

	// Past the last limit there is no tighter bound left to offer, and "no
	// bound at all" is the one that keeps every recipe the caller asked for.
	it('reads a value past the last limit as off', () => {
		expect(snapMaxMinutes(241)).toBe(0);
		expect(snapMaxMinutes(1440)).toBe(0);
	});
});

describe('timeStopIndex', () => {
	it('finds the index of each stop', () => {
		TIME_STOPS.forEach((stop, index) => {
			expect(timeStopIndex(stop)).toBe(index);
		});
	});

	it('puts off at the far end', () => {
		expect(timeStopIndex(0)).toBe(TIME_STOPS.length - 1);
	});

	it('snaps a value between two stops to the index of the next one', () => {
		expect(timeStopIndex(37)).toBe(timeStopIndex(45));
	});
});

describe('timeStopMinutes', () => {
	it('reads back the stop at an index', () => {
		TIME_STOPS.forEach((stop, index) => {
			expect(timeStopMinutes(index)).toBe(stop);
		});
	});

	it('reads an index outside the scale as off', () => {
		expect(timeStopMinutes(-1)).toBe(0);
		expect(timeStopMinutes(TIME_STOPS.length)).toBe(0);
		expect(timeStopMinutes(1.5)).toBe(0);
	});
});
