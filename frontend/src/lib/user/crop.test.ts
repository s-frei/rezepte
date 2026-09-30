import { describe, expect, it } from 'vitest';
import { centered, clampView, toCrop, zoomAt, MAX_ZOOM } from './crop';

// A 400x200 landscape image in a 100 px frame: at zoom 1 its short side
// (200) fills the frame, so one frame pixel is two image pixels.
const W = 400;
const H = 200;
const F = 100;

describe('crop math', () => {
	it('starts centered and maps to the centered square', () => {
		const view = centered(W, H, F);
		expect(view).toEqual({ zoom: 1, x: -50, y: 0 });
		expect(toCrop(W, H, F, view)).toEqual({ x: 0.25, y: 0, size: 1 });
	});

	it('keeps the image covering the frame', () => {
		expect(clampView(W, H, F, { zoom: 1, x: 10, y: 10 })).toEqual({ zoom: 1, x: 0, y: 0 });
		expect(clampView(W, H, F, { zoom: 1, x: -500, y: -500 })).toEqual({ zoom: 1, x: -100, y: 0 });
	});

	it('bounds the zoom', () => {
		expect(clampView(W, H, F, { zoom: 0.5, x: 0, y: 0 }).zoom).toBe(1);
		expect(clampView(W, H, F, { zoom: 9, x: 0, y: 0 }).zoom).toBe(MAX_ZOOM);
	});

	it('zooms around the pointer', () => {
		const view = zoomAt(W, H, F, centered(W, H, F), 2, 50, 50);
		// The image point under the frame center stays under it.
		expect(view).toEqual({ zoom: 2, x: -150, y: -50 });
		expect(toCrop(W, H, F, view)).toEqual({ x: 0.375, y: 0.25, size: 0.5 });
	});

	it('never crops past the edge', () => {
		const view = clampView(W, H, F, { zoom: 2, x: -1000, y: -1000 });
		const crop = toCrop(W, H, F, view);
		expect(crop.x * W + crop.size * H).toBeCloseTo(W);
		expect(crop.y * H + crop.size * H).toBeCloseTo(H);
	});
});
