import { describe, expect, test } from 'vitest';
import { parseUnitList, unitOptions } from './units';

describe('parseUnitList', () => {
	test('splits on commas and keeps a unit with a space whole', () => {
		expect(parseUnitList('tsp, tbsp, fl oz,oz')).toEqual(['tsp', 'tbsp', 'fl oz', 'oz']);
	});

	test('drops the empty entries a stray comma leaves', () => {
		expect(parseUnitList(' g,, kg, ')).toEqual(['g', 'kg']);
	});
});

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
