import { describe, expect, it } from 'vitest';
import { navScroll, startNavScroll, type NavScroll } from './nav-scroll';

const tall = 3000;

/** Feeds a run of scroll positions through the reducer, as the scroll events would. */
function scroll(from: NavScroll, positions: number[], maxY = tall): NavScroll {
	return positions.reduce((state, y) => navScroll(state, y, maxY), from);
}

const reading = (mode: NavScroll['mode'], y: number): NavScroll => ({ mode, anchor: y, lastY: y });

describe('navScroll', () => {
	it('minimizes on a scroll down past the top', () => {
		expect(scroll(reading('expanded', 380), [400]).mode).toBe('minimized');
	});

	it('expands on a scroll up', () => {
		expect(scroll(reading('minimized', 400), [380]).mode).toBe('expanded');
	});

	// Reading speed is a few pixels per frame; the distance counts over the
	// whole movement, not per scroll event.
	it('minimizes on a slow scroll down', () => {
		expect(scroll(reading('expanded', 400), [402, 404, 406, 408, 410]).mode).toBe('minimized');
	});

	it('expands on a slow scroll up', () => {
		expect(scroll(reading('minimized', 800), [798, 796, 794, 792, 790]).mode).toBe('expanded');
	});

	// A finger resting on the glass jitters by a pixel or two either way; that
	// must not make the bar flicker between its two shapes.
	it('keeps its shape while the position jitters', () => {
		expect(scroll(reading('expanded', 400), [403, 401, 404, 402, 405, 403]).mode).toBe('expanded');
		expect(scroll(reading('minimized', 400), [397, 399, 396, 398, 395, 397]).mode).toBe(
			'minimized'
		);
	});

	it('stays expanded near the top of the page', () => {
		expect(scroll(reading('expanded', 60), [100]).mode).toBe('expanded');
		expect(scroll(reading('minimized', 60), [100]).mode).toBe('expanded');
	});

	// At the end there is nothing left to read past the bar.
	it('expands at the end of the page', () => {
		expect(scroll(reading('minimized', tall - 40), [tall - 10]).mode).toBe('expanded');
	});

	it('never minimizes a page that does not scroll', () => {
		expect(scroll(startNavScroll(0), [0], 0).mode).toBe('expanded');
		expect(scroll(reading('minimized', 0), [0], 0).mode).toBe('expanded');
	});

	it('starts expanded wherever the page is', () => {
		expect(startNavScroll(900)).toEqual({ mode: 'expanded', anchor: 900, lastY: 900 });
	});
});
