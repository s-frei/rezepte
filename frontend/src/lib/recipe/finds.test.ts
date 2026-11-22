import { describe, expect, test } from 'vitest';
import type { SearchFind } from '#lib/api/recipes.js';
import { clampNames, dedupeTextFinds, missingAfterTicks } from './finds.js';

const item = (name: string, key = name.toLowerCase()) => ({ name, key });

describe('missingAfterTicks', () => {
	test('a tick covers the ingredients it serves and leaves the rest', () => {
		const missing = [item('Rotweinessig'), item('Salz')];
		expect(missingAfterTicks(missing, [item('Essig')])).toEqual([item('Salz')]);
	});

	test('without ticks everything is still missing', () => {
		const missing = [item('Salz')];
		expect(missingAfterTicks(missing, [])).toEqual(missing);
	});
});

describe('clampNames', () => {
	const names = ['a', 'b', 'c', 'd', 'e', 'f', 'g'];

	test('keeps the names that fit and counts the rest', () => {
		expect(clampNames(names, 3)).toEqual({ shown: ['a', 'b', 'c'], more: 4 });
	});

	test('everything fits', () => {
		expect(clampNames(names, Infinity)).toEqual({ shown: names, more: 0 });
	});
});

describe('dedupeTextFinds', () => {
	const find = (field: SearchFind['field'], text: string, more = 0): SearchFind => ({
		field,
		items: [{ parts: [{ text, hit: true }] }],
		more
	});

	test('an ingredient find the pantry lines already name is dropped', () => {
		expect(dedupeTextFinds([find('ingredients', 'Senf')], ['Kartoffeln', 'Senf'])).toEqual([]);
	});

	test('an ingredient the pantry lines do not name keeps its line', () => {
		const finds = [find('ingredients', 'Wasser')];
		expect(dedupeTextFinds(finds, ['Senf'])).toEqual(finds);
	});

	test('a find with more matches than it lists keeps its line', () => {
		const finds = [find('ingredients', 'Senf', 2)];
		expect(dedupeTextFinds(finds, ['Senf'])).toEqual(finds);
	});

	test('finds in other fields are left alone', () => {
		const finds = [find('tags', 'Senf')];
		expect(dedupeTextFinds(finds, ['Senf'])).toEqual(finds);
	});
});
