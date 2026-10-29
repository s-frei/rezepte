import { describe, expect, it } from 'vitest';
import { placeCards } from './masonry';

describe('placeCards', () => {
	it('puts each card into the shortest column', () => {
		expect(placeCards([100, 50, 80], 2, 10)).toEqual([
			{ column: 0, top: 0 },
			{ column: 1, top: 0 },
			{ column: 1, top: 60 }
		]);
	});

	it('prefers the leftmost column within the tolerance', () => {
		expect(placeCards([100, 92, 50], 2, 10, 16)[2]).toEqual({ column: 0, top: 110 });
	});

	it('keeps earlier placements when more cards are appended', () => {
		const heights = [120, 80, 200, 90, 150, 60];
		expect(placeCards(heights.slice(0, 3), 3, 20)).toEqual(placeCards(heights, 3, 20).slice(0, 3));
	});

	it('stacks every card in one column', () => {
		expect(placeCards([10, 20, 30], 1, 5)).toEqual([
			{ column: 0, top: 0 },
			{ column: 0, top: 15 },
			{ column: 0, top: 40 }
		]);
	});

	it('places a list taller than 10 000px without clamping', () => {
		const placed = placeCards(new Array(300).fill(190), 2, 10);
		expect(placed.at(-1)!.top).toBeGreaterThan(10_000);
	});
});
