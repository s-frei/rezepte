import { describe, expect, it } from 'vitest';
import { dimensionOf, fromBase, toBase } from './measures';

describe('measures', () => {
	it('knows metric weights, volumes and pieces, case and spaces ignored', () => {
		expect(dimensionOf(' KG ')).toBe('mass');
		expect(dimensionOf('dl')).toBe('volume');
		expect(dimensionOf('')).toBe('count');
		expect(dimensionOf('Bund')).toBeUndefined();
	});

	it('moves an amount through the base unit', () => {
		expect(toBase(1.5, 'kg')).toBe(1500);
		expect(toBase(3, 'Bund')).toBe(3);
		expect(fromBase(1500, 'mass')).toEqual({ quantity: 1.5, unit: 'kg' });
		expect(fromBase(250, 'volume')).toEqual({ quantity: 250, unit: 'ml' });
	});
});
