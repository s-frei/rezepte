import { describe, expect, it } from 'vitest';
import { splitColumns } from './ingredient-columns';

const group = (name: string | null, count: number) => ({
	name,
	ingredients: Array.from({ length: count }, (_, i) => i)
});

describe('splitColumns', () => {
	it('splits between groups when that balances the sides', () => {
		// 1+6 = 7 rows against 1+5 = 6 rows.
		expect(splitColumns([group('Rouladen', 6), group('Sauce', 5)])).toEqual([
			[{ group: 0, from: 0, to: 6, heading: true }],
			[{ group: 1, from: 0, to: 5, heading: true }]
		]);
	});

	it('picks the most balanced group boundary', () => {
		const [left, right] = splitColumns([group('A', 2), group('B', 2), group('C', 3)]);
		expect(left.map((p) => p.group)).toEqual([0, 1]);
		expect(right.map((p) => p.group)).toEqual([2]);
	});

	it('splits a list without groups at its middle, left taking the odd row', () => {
		expect(splitColumns([group(null, 7)])).toEqual([
			[{ group: 0, from: 0, to: 4, heading: false }],
			[{ group: 0, from: 4, to: 7, heading: false }]
		]);
	});

	it('splits inside the long group without repeating its heading', () => {
		// The boundary leaves 3 against 11 rows; B broken after its third ingredient gives 7 against 7.
		expect(splitColumns([group('A', 2), group('B', 10)])).toEqual([
			[
				{ group: 0, from: 0, to: 2, heading: true },
				{ group: 1, from: 0, to: 3, heading: true }
			],
			[{ group: 1, from: 3, to: 10, heading: false }]
		]);
	});

	it('splits inside the group that crosses the middle', () => {
		const [left, right] = splitColumns([group('A', 5), group('B', 5), group('C', 5)]);
		expect(left.at(-1)).toEqual({ group: 1, from: 0, to: 2, heading: true });
		expect(right[0]).toEqual({ group: 1, from: 2, to: 5, heading: false });
	});

	it('keeps a short list whole in the left column', () => {
		// A heading and four ingredients: five rows, one short of a split.
		expect(splitColumns([group('Sauce', 4)])).toEqual([
			[{ group: 0, from: 0, to: 4, heading: true }],
			[]
		]);
		expect(splitColumns([group(null, 6)])[1]).toHaveLength(1);
	});

	it('never splits when the caller asks for one column', () => {
		expect(splitColumns([group('A', 6), group('B', 5)], Infinity)[1]).toEqual([]);
	});

	it('keeps a single ingredient in the left column', () => {
		expect(splitColumns([group(null, 1)])).toEqual([
			[{ group: 0, from: 0, to: 1, heading: false }],
			[]
		]);
	});
});
