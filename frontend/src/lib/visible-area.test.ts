import { describe, expect, test } from 'vitest';
import { measureVisibleArea } from './visible-area.js';

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
