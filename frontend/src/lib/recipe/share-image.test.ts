import { describe, expect, it } from 'vitest';
import { MAX_CANVAS_PIXELS, MAX_CANVAS_SIDE, pickScale } from './share-image';

describe('pickScale', () => {
	it('renders a short card at 3x', () => {
		expect(pickScale(420, 1500)).toBe(3);
	});

	it('lowers the scale for a tall card', () => {
		const scale = pickScale(420, 10_000);
		expect(scale).toBeLessThan(3);
		expect(scale).toBeGreaterThan(1);
	});

	it('never goes below 1, however tall the card', () => {
		expect(pickScale(420, 60_000)).toBe(1);
	});

	it('keeps every realistic card under the canvas cap', () => {
		for (let height = 400; height <= 38_000; height += 97) {
			const scale = pickScale(420, height);
			expect(420 * scale * height * scale).toBeLessThanOrEqual(MAX_CANVAS_PIXELS);
		}
	});

	it('keeps a very tall card under the longest side a canvas may have', () => {
		for (let height = 20_000; height <= MAX_CANVAS_SIDE; height += 997) {
			expect(height * pickScale(420, height)).toBeLessThanOrEqual(MAX_CANVAS_SIDE);
		}
	});
});
