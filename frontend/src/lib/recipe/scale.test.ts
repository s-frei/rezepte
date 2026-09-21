import { describe, expect, it, vi } from 'vitest';
import { formatQuantityFor, formatScaled, roundScaled, scaleQuantity } from './scale';

// formatScaled/formatQuantityFor call through to format.ts's formatQuantity,
// which is locale-aware; pin the locale explicitly rather than relying on
// Paraglide's baseLocale default, so this file states its assumption instead
// of inheriting it.
vi.mock('$lib/paraglide/runtime', () => ({
	getLocale: () => 'en',
	experimentalStaticLocale: undefined
}));

describe('scaleQuantity', () => {
	it.each([
		[200, 4, 8, 400],
		[200, 4, 6, 300],
		[3, 4, 1, 0.75],
		[0.5, 4, 2, 0.25],
		[150, 4, 3, 112.5]
	])('scales %d from %d to %d servings', (quantity, from, to, expected) => {
		expect(scaleQuantity(quantity, from, to)).toBeCloseTo(expected, 10);
	});

	it('returns the stored value untouched when nothing changes', () => {
		// No multiplication at all, so no floating-point drift on the baseline.
		expect(scaleQuantity(1.33, 3, 3)).toBe(1.33);
	});

	it('returns the stored value for a non-positive base or target', () => {
		expect(scaleQuantity(200, 0, 8)).toBe(200);
		expect(scaleQuantity(200, 4, 0)).toBe(200);
	});
});

describe('roundScaled', () => {
	it.each([
		// below 1: snap within 0.04 to 0, ¼, ⅓, ½, ⅔, ¾, 1 - else one decimal
		[0.24, 0.25],
		// Exactly 0.04 away from ¼ - binary error must not push it out of the window.
		[0.29, 0.25],
		[0.3, 1 / 3],
		[0.5, 0.5],
		[0.66, 2 / 3],
		[0.74, 0.75],
		[0.97, 1],
		[0.4, 0.4],
		[0.17, 0.2],
		[0.12, 0.1],
		[0.04, 0],
		// 1 to 10: thirds within 0.04 snap first, everything else to the nearest quarter
		[1.1, 1],
		[1.15, 1.25],
		[1.33, 1 + 1 / 3],
		[1.66, 1 + 2 / 3],
		[2.6, 2.5],
		[2.9, 3],
		[7.375, 7.5],
		[9.99, 10],
		// from 10: whole numbers, no separators
		[10.4, 10],
		[12.5, 13],
		[333.33, 333],
		[1250, 1250],
		[1333.4, 1333],
		// exact values pass through
		[0, 0],
		[2, 2],
		[400, 400]
	])('rounds %f to %f', (input, expected) => {
		expect(roundScaled(input)).toBeCloseTo(expected, 10);
	});

	it('keeps the sign', () => {
		expect(roundScaled(-1.5)).toBe(-1.5);
		expect(roundScaled(-12.4)).toBe(-12);
	});
});

describe('formatScaled', () => {
	it.each([
		[0.25, '¼'],
		[0.29, '¼'],
		[1 / 3, '⅓'],
		[0.4, '0.4'],
		[0.17, '0.2'],
		[2.5, '2 ½'],
		[1.33, '1 ⅓'],
		[2.66, '2 ⅔'],
		[1.15, '1 ¼'],
		[12.5, '13'],
		[1250, '1250'],
		[400, '400']
	])('formats %f as %s', (input, expected) => {
		expect(formatScaled(input)).toBe(expected);
	});
});

describe('formatQuantityFor', () => {
	it('renders null as an empty string', () => {
		expect(formatQuantityFor(null, 4, 8)).toBe('');
	});

	it('renders the exact stored value at the baseline, without rounding', () => {
		expect(formatQuantityFor(1.33, 3, 3)).toBe('1 ⅓');
		expect(formatQuantityFor(1.45, 4, 4)).toBe('1.45');
	});

	it.each([
		[200, 4, 8, '400'],
		[1.33, 3, 6, '2 ⅔'],
		[1, 4, 6, '1 ½'],
		[2, 4, 3, '1 ½'],
		[1, 4, 3, '¾'],
		[150, 4, 3, '113'],
		[1, 4, 1, '¼']
	])('scales %s from %d to %d as %s', (quantity, from, to, expected) => {
		expect(formatQuantityFor(quantity, from, to)).toBe(expected);
	});
});
