import { describe, expect, it } from 'vitest';
import { docked } from './nav-scroll';

// A page that does not scroll stays at y=0 and therefore keeps the bar floating.
describe('docked', () => {
	it('floats at the top of a page', () => {
		expect(docked(0)).toBe(false);
	});

	it('floats up to the threshold', () => {
		expect(docked(120)).toBe(false);
	});

	it('docks once reading starts', () => {
		expect(docked(121)).toBe(true);
	});
});
