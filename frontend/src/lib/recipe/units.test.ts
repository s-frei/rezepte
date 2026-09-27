import { describe, expect, test } from 'vitest';
import { unitOptions } from './units';

const units = ['g', 'kg', 'ml', 'l', 'EL', 'TL', 'Stück', 'Prise'];

describe('unitOptions', () => {
	test('an opened field lists every unit, whatever it already holds', () => {
		expect(unitOptions(units, 'EL', false)).toEqual(units);
		expect(unitOptions(units, 'Dose', false)).toEqual(units);
	});

	test('typing narrows the list to the units that contain the text', () => {
		expect(unitOptions(units, 'g', true)).toEqual(['g', 'kg']);
		expect(unitOptions(units, 'l', true)).toEqual(['ml', 'l', 'EL', 'TL']);
	});

	test('the match ignores case and surrounding space', () => {
		expect(unitOptions(units, ' st ', true)).toEqual(['Stück']);
		expect(unitOptions(units, 'el', true)).toEqual(['EL']);
	});

	test('a cleared field lists everything again', () => {
		expect(unitOptions(units, '  ', true)).toEqual(units);
	});

	test('a unit of its own matches nothing', () => {
		expect(unitOptions(units, 'Bund', true)).toEqual([]);
	});
});
