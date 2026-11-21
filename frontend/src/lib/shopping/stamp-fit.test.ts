import { describe, expect, it } from 'vitest';
import { fitStamps } from './stamp-fit';

const s = (full: number, bare = full) => ({ full, bare });
const GAP = 6;
const MORE = 30;

describe('fitStamps', () => {
	it('shows every stamp whole when they fit', () => {
		expect(fitStamps([s(100, 60), s(100, 60)], 300, 1, GAP, MORE)).toEqual({
			shown: 2,
			bare: false,
			summary: false
		});
	});

	it('drops every detail at once when that shows more stamps', () => {
		// Whole: 100 + 6 + 100 > 180, so one plus "+1"; bare: 60 + 6 + 60 fits both.
		expect(fitStamps([s(100, 60), s(100, 60)], 180, 1, GAP, MORE)).toEqual({
			shown: 2,
			bare: true,
			summary: false
		});
	});

	it('takes a free line before it drops a detail', () => {
		// Whole, the second stamp starts line 2 and both show; no detail goes.
		expect(fitStamps([s(150, 100), s(150, 100)], 200, 2, GAP, MORE)).toEqual({
			shown: 2,
			bare: false,
			summary: false
		});
	});

	it('keeps the details when going bare would show no more', () => {
		// Lines of 200: whole [120] [120 +n]; bare [100] [100 +n] - the same two.
		expect(fitStamps([s(120, 100), s(120, 100), s(120, 100)], 200, 2, GAP, MORE)).toEqual({
			shown: 2,
			bare: false,
			summary: false
		});
	});

	it('moves a stamp behind the "+n" when it would not fit beside it', () => {
		// One line: all three do not fit, and [100 60 +n] is 202 > 200; so [100 +n].
		expect(fitStamps([s(100), s(60), s(190)], 200, 1, GAP, MORE)).toEqual({
			shown: 1,
			bare: false,
			summary: false
		});
	});

	it('collapses into one summary when not even the first bare stamp fits', () => {
		// A narrow card: "Getränke für das Sommerfest" bare is 190, the row 145.
		expect(fitStamps([s(260, 190), s(120, 100), s(80)], 145, 2, GAP, MORE)).toEqual({
			shown: 0,
			bare: false,
			summary: true
		});
	});

	it('collapses when the first stamp leaves no room for its "+n"', () => {
		expect(fitStamps([s(200, 170), s(90)], 190, 1, GAP, MORE).summary).toBe(true);
	});

	it('shows a single stamp bare when only its name fits', () => {
		expect(fitStamps([s(200, 150)], 160, 1, GAP, MORE)).toEqual({
			shown: 1,
			bare: true,
			summary: false
		});
		expect(fitStamps([s(200, 170)], 160, 1, GAP, MORE).summary).toBe(true);
	});

	it('shows nothing for no stamps', () => {
		expect(fitStamps([], 100, 2, GAP, MORE)).toEqual({ shown: 0, bare: false, summary: false });
	});
});
