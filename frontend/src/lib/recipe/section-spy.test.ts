import { describe, expect, it } from 'vitest';
import { currentSection, isAtBottom, nextBand } from './section-spy';

const order = ['basics', 'images', 'ingredients', 'steps', 'editing'];

describe('currentSection', () => {
	it('picks the topmost of every section in the band, not just the latest change', () => {
		expect(currentSection(order, new Set(['steps', 'ingredients']), false, 'basics')).toBe(
			'ingredients'
		);
	});

	it('keeps the current section while nothing is in the band', () => {
		expect(currentSection(order, new Set(), false, 'images')).toBe('images');
	});

	it('hands the last section the nav once the page cannot scroll further', () => {
		// The short last section never reaches the band under the top bar,
		// because the page ends before it gets there.
		expect(currentSection(order, new Set(['steps']), true, 'steps')).toBe('editing');
	});

	it('ignores ids it does not know', () => {
		expect(currentSection(order, new Set(['elsewhere']), false, 'steps')).toBe('steps');
	});
});

describe('isAtBottom', () => {
	it('is true when the viewport reaches the end of the page', () => {
		expect(isAtBottom({ innerHeight: 720, scrollY: 1280, scrollHeight: 2000 })).toBe(true);
	});

	it('allows for sub-pixel rounding', () => {
		expect(isAtBottom({ innerHeight: 720, scrollY: 1278.5, scrollHeight: 2000 })).toBe(true);
	});

	it('is false with page left below the viewport', () => {
		expect(isAtBottom({ innerHeight: 720, scrollY: 1000, scrollHeight: 2000 })).toBe(false);
	});

	it('is false for a page too short to scroll', () => {
		// Nothing was scrolled to, so the first section keeps the nav.
		expect(isAtBottom({ innerHeight: 720, scrollY: 0, scrollHeight: 700 })).toBe(false);
	});
});

describe('nextBand', () => {
	it('keeps sections a callback does not mention', () => {
		const band = nextBand(new Set(['ingredients']), [{ id: 'steps', isIntersecting: true }]);
		expect([...band]).toEqual(['ingredients', 'steps']);
	});

	it('drops sections that left the band', () => {
		const band = nextBand(new Set(['ingredients', 'steps']), [
			{ id: 'ingredients', isIntersecting: false }
		]);
		expect([...band]).toEqual(['steps']);
	});

	it('leaves the band it was given alone', () => {
		const before = new Set(['basics']);
		nextBand(before, [{ id: 'basics', isIntersecting: false }]);
		expect([...before]).toEqual(['basics']);
	});
});
