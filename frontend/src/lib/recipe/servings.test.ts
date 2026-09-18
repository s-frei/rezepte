import { describe, expect, it } from 'vitest';
import { clampServings, parseServings, SERVINGS_MAX, SERVINGS_MIN, servingsKey } from './servings';

describe('servingsKey', () => {
	it('prefixes the recipe id', () => {
		expect(servingsKey('0192abcd')).toBe('rezepte-servings:0192abcd');
	});
});

describe('clampServings', () => {
	it('keeps values inside the range', () => {
		expect(clampServings(1)).toBe(1);
		expect(clampServings(4)).toBe(4);
		expect(clampServings(99)).toBe(99);
	});

	it('clamps to the bounds', () => {
		expect(clampServings(0)).toBe(SERVINGS_MIN);
		expect(clampServings(-3)).toBe(SERVINGS_MIN);
		expect(clampServings(100)).toBe(SERVINGS_MAX);
		expect(clampServings(Number.POSITIVE_INFINITY)).toBe(SERVINGS_MAX);
	});

	it('rounds to whole servings and treats NaN as the minimum', () => {
		expect(clampServings(2.6)).toBe(3);
		expect(clampServings(Number.NaN)).toBe(SERVINGS_MIN);
	});
});

describe('parseServings', () => {
	it('parses a stored integer inside the range', () => {
		expect(parseServings('6')).toBe(6);
		expect(parseServings('99')).toBe(99);
	});

	it('returns null for anything else', () => {
		expect(parseServings(null)).toBeNull();
		expect(parseServings('')).toBeNull();
		expect(parseServings('abc')).toBeNull();
		expect(parseServings('0')).toBeNull();
		expect(parseServings('100')).toBeNull();
		expect(parseServings('2.5')).toBeNull();
	});
});
