import { describe, expect, it } from 'vitest';
import { lifetimeOptions } from './lifetimes';

describe('lifetimeOptions', () => {
	it('offers all five lifetimes without a maximum', () => {
		expect(lifetimeOptions(null)).toEqual([1, 7, 30, 365, null]);
	});

	it('caps at 30 days, dropping the year and permanent', () => {
		expect(lifetimeOptions(30)).toEqual([1, 7, 30]);
	});

	it('caps at 1 day, leaving only itself', () => {
		expect(lifetimeOptions(1)).toEqual([1]);
	});

	it('caps at 365 days, keeping every finite lifetime', () => {
		expect(lifetimeOptions(365)).toEqual([1, 7, 30, 365]);
	});
});
