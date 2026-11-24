import { describe, expect, it } from 'vitest';
import { cardRatio, focusPosition, isCentered, settle } from './focus';

/** The two percentages of an `object-position`. */
const parts = (pos: string) => pos.split(' ').map((p) => parseFloat(p));

describe('focusPosition', () => {
	it('puts a portrait point at the center of a wider box', () => {
		const [x, y] = parts(focusPosition({ x: 0.5, y: 0.66 }, 0.75, 4 / 3));
		expect(x).toBe(50);
		// Visible 0.5625: (0.66 - 0.28125) / 0.4375 = 0.8657.
		expect(y).toBeCloseTo(86.6, 1);
	});

	it('stops at the edge', () => {
		expect(focusPosition({ x: 0.5, y: 0.95 }, 0.75, 4 / 3)).toBe('50% 100%');
		expect(focusPosition({ x: 0.36, y: 0.5 }, 1.339, 1)).toBe('0% 50%');
	});

	it('centers a centered point, an uncropped box and a missing focus', () => {
		expect(focusPosition({ x: 0.5, y: 0.5 }, 0.75, 4 / 3)).toBe('50% 50%');
		expect(focusPosition({ x: 0.2, y: 0.9 }, 1.5, 1.5)).toBe('50% 50%');
		expect(focusPosition(null, 0.75, 4 / 3)).toBe('50% 50%');
		expect(focusPosition(undefined, 0.75, 4 / 3)).toBe('50% 50%');
	});
});

describe('settle', () => {
	it('moves a portrait point only vertically, to the middle of what every shape shows', () => {
		const f = settle({ x: 0.3, y: 0.8 }, 0.75, [4 / 3, 0.8, 1]);
		expect(f.x).toBe(0.5);
		expect(f.y).toBeCloseTo(0.71875, 6);
	});

	it('moves a near-square point on both axes', () => {
		const f = settle({ x: 0.2, y: 0.2 }, 1.1, [4 / 3, cardRatio(1.1), 1]);
		// The square cuts the sides (visible 1/1.1), the recipe page top and
		// bottom (visible 1.1/(4/3) = 0.825); both stop at the edge.
		expect(f.x).toBeCloseTo(1 / 2.2, 4);
		expect(f.y).toBeCloseTo(0.4125, 4);
	});
});

describe('cardRatio', () => {
	it('keeps the photo between 4:5 and 16:9', () => {
		expect(cardRatio(0.5625)).toBe(0.8);
		expect(cardRatio(2)).toBe(16 / 9);
		expect(cardRatio(1.2)).toBe(1.2);
	});
});

describe('isCentered', () => {
	it('takes a rounding step as the center, a percent off as not', () => {
		expect(isCentered({ x: 0.5, y: 0.504 })).toBe(true);
		expect(isCentered({ x: 0.5, y: 0.51 })).toBe(false);
		expect(isCentered({ x: 0.49, y: 0.5 })).toBe(false);
	});
});
