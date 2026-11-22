import { describe, expect, test } from 'vitest';
import { fitListHeight, measureVisibleArea } from './visible-area.js';

describe('measureVisibleArea', () => {
	test.each([
		[
			'no keyboard',
			{ top: 0, bottom: 780 },
			{ offsetTop: 0, height: 780 },
			false,
			{ top: 0, bottom: 0 }
		],
		[
			'keyboard, iOS panned',
			{ top: 0, bottom: 780 },
			{ offsetTop: 120, height: 480 },
			true,
			{ top: 120, bottom: 180 }
		],
		[
			'Chrome toolbar box',
			{ top: -56, bottom: 724 },
			{ offsetTop: 0, height: 424 },
			true,
			{ top: 56, bottom: 300 }
		],
		[
			'stuck iOS viewport, no field',
			{ top: 0, bottom: 780 },
			{ offsetTop: 3, height: 756 },
			false,
			{ top: 0, bottom: 0 }
		],
		[
			'never negative',
			{ top: 0, bottom: 780 },
			{ offsetTop: 0, height: 800 },
			true,
			{ top: 0, bottom: 0 }
		],
		[
			'rounded',
			{ top: 0.4, bottom: 780.4 },
			{ offsetTop: 120.6, height: 480.2 },
			true,
			{ top: 120, bottom: 180 }
		]
	])('%s', (_name, probe, viewport, typing, expected) => {
		expect(measureVisibleArea(probe, viewport, typing)).toEqual(expected);
	});
});

describe('fitListHeight', () => {
	// Five 44px options from 0, ending at 220.
	const rows = [0, 44, 88, 132, 176].map((top) => ({ top, height: 44 }));
	// A 64px text row (two lines) and then 44px options.
	const mixed = [
		{ top: 0, height: 64 },
		{ top: 64, height: 44 },
		{ top: 108, height: 44 },
		{ top: 152, height: 44 }
	];
	test.each([
		['everything fits', 300, rows, 300],
		['the last row cut at 55%', 200, rows, 200],
		['the last row at its 48% lower bound', 198, rows, 198],
		['the last row under 48% moves the cut up a row', 197, rows, 156],
		['the last row at 32% moves the cut up a row', 190, rows, 156],
		['never under two rows', 50, rows, 88],
		['no rows, short', 50, [], 88],
		['no rows, tall', 300, [], 300],
		['mixed heights, cut at 55% of a 44px row', 140, mixed, 132]
	])('%s', (_name, room, list, expected) => {
		expect(fitListHeight(room, list)).toBe(expected);
	});
});
