import { describe, expect, it } from 'vitest';
import { MAX_CANVAS_PIXELS, pickScale } from './share-image';

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
});
