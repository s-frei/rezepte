import { describe, expect, test } from 'vitest';
import type { SearchFind } from '#lib/api/recipes.js';
import {
	clampNames,
	dedupeTextFinds,
	longNames,
	missingAfterTicks,
	missingParts
} from './finds.js';

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

describe('missingParts', () => {
	const hits = new Map([
		[
			'Rotweinessig',
			[
				{ text: 'Rotwein', hit: true },
				{ text: 'essig', hit: false }
			]
		]
	]);

	test('marks the hovered ingredient whole and keeps the text hits of the rest', () => {
		expect(missingParts([item('Salz'), item('Rotweinessig')], hits, 'salz')).toEqual([
			{ parts: [{ text: 'Salz', hit: true }] },
			{ parts: hits.get('Rotweinessig') }
		]);
	});

	test('without a hover nothing new is marked', () => {
		expect(missingParts([item('Salz')], hits, null)).toEqual([
			{ parts: [{ text: 'Salz', hit: false }] }
		]);
	});
});

describe('longNames', () => {
	// One pixel per character.
	const measure = (text: string) => text.length;

	test('the first name against the room after the icon, the rest against a full line', () => {
		// "Rinderrouladen," is 15 wide: too wide for 14 after the icon, but
		// the same 15 fits a 20 line in second place.
		expect(
			longNames(['Rinderrouladen', 'Rinderrouladen'], { first: 14, line: 20 }, measure)
		).toEqual([true, false]);
	});

	test('counts the comma on all but the last name', () => {
		// "Pfeffer," is 8, too wide for 8 with the margin; the last "Pfeffer" is 7 and fits.
		expect(longNames(['Pfeffer', 'Pfeffer'], { first: 8, line: 8 }, measure)).toEqual([
			true,
			false
		]);
	});

	test('a name within 1px of its room counts as too wide', () => {
		expect(longNames(['Senf'], { first: 4, line: 20 }, measure)).toEqual([true]);
		expect(longNames(['Senf'], { first: 5, line: 20 }, measure)).toEqual([false]);
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
