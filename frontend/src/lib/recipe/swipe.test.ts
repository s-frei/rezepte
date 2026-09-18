import { describe, expect, it } from 'vitest';
import { SWIPE_THRESHOLD, swipeDirection } from './swipe';

describe('swipeDirection', () => {
	it('is 50px by default', () => {
		expect(SWIPE_THRESHOLD).toBe(50);
	});

	it('reads a leftward drag as "next"', () => {
		expect(swipeDirection(200, 140)).toBe('next');
		expect(swipeDirection(200, 150)).toBe('next'); // exactly the threshold counts
	});

	it('reads a rightward drag as "previous"', () => {
		expect(swipeDirection(100, 170)).toBe('previous');
		expect(swipeDirection(100, 150)).toBe('previous');
	});

	it('ignores anything shorter than the threshold', () => {
		expect(swipeDirection(100, 130)).toBeNull();
		expect(swipeDirection(100, 51)).toBeNull();
		expect(swipeDirection(100, 100)).toBeNull();
	});

	it('takes a custom threshold', () => {
		expect(swipeDirection(100, 80, 10)).toBe('next');
		expect(swipeDirection(100, 80, 30)).toBeNull();
	});
});
