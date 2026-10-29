import { describe, expect, it } from 'vitest';
import { initialFor, tintFor } from './placeholder';

describe('tintFor', () => {
	it('is stable for the same id', () => {
		expect(tintFor('abc')).toBe(tintFor('abc'));
		expect(tintFor('11111111-1111-1111-1111-111111111111')).toBe(
			tintFor('11111111-1111-1111-1111-111111111111')
		);
	});

	it('always returns one of the three tint classes', () => {
		const ids = ['', 'a', 'recipe-1', '11111111-1111-1111-1111-111111111111'];
		for (const id of ids) {
			expect(['bg-tint-1', 'bg-tint-2', 'bg-tint-3']).toContain(tintFor(id));
		}
	});

	it('reaches all three tints across a spread of ids', () => {
		const seen = new Set(Array.from({ length: 30 }, (_, i) => tintFor(`recipe-${i}`)));
		expect(seen).toEqual(new Set(['bg-tint-1', 'bg-tint-2', 'bg-tint-3']));
	});
});

describe('initialFor', () => {
	it.each([
		['Pesto', 'P'],
		['🍝 Spaghetti aglio e olio', 'S'],
		['„Omas“ Kartoffelsuppe', 'O'],
		['3-Minuten-Ei', '3'],
		['ölkuchen', 'Ö'],
		['🍝', '?'],
		['', '?']
	])('%s -> %s', (title, initial) => {
		expect(initialFor(title)).toBe(initial);
	});
});
