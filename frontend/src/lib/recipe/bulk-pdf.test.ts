import { describe, expect, it } from 'vitest';
import type { Recipe } from '../api/recipes';
import { bulkPdfRequest } from './bulk-pdf';
import type { PrintOptions } from './print-options.svelte';

const recipe = (id: string, servings: number) =>
	({
		id,
		title: id,
		description: '',
		servings,
		prepMinutes: null,
		cookMinutes: null,
		sourceUrl: null,
		sourceName: null,
		tags: [],
		ingredientGroups: [
			{ name: null, ingredients: [{ quantity: 400, unit: 'g', name: 'Flour', note: null }] }
		],
		steps: []
	}) as unknown as Recipe;

const options: PrintOptions = {
	format: 'a4',
	photo: true,
	footer: false,
	bundle: 'book',
	servings: null
};
const now = new Date(2026, 10, 5, 23, 30);
const flour = (r: { ingredientGroups: { ingredients: { quantity: unknown }[] }[] }) =>
	r.ingredientGroups[0].ingredients[0].quantity;

describe('bulkPdfRequest', () => {
	it('builds a cookbook at each recipe’s own servings, dated by the local day', () => {
		const body = bulkPdfRequest([recipe('a', 4), recipe('b', 2)], options, 'https://x.test', now);
		expect(body.options).toEqual({
			bundle: 'book',
			paper: 'a4',
			photo: true,
			footer: false
		});
		expect(body.origin).toBe('https://x.test');
		expect(body.recipes.map((r) => r.id)).toEqual(['a', 'b']);
		expect(body.recipes.map(flour)).toEqual([
			{ value: 400, display: '400' },
			{ value: 400, display: '400' }
		]);
		expect(body.book?.date).toBe('2026-11-05');
		expect(body.book?.dateDisplay).toBe('November 2026');
		expect(body.book?.title).toBe('Recipes');
		expect(body.book?.countDisplay).toBe('2 recipes');
	});

	it('scales every recipe to the override and sends a zip without a book', () => {
		const body = bulkPdfRequest(
			[recipe('a', 4), recipe('b', 2)],
			{ ...options, format: 'a5', bundle: 'zip', servings: 2 },
			'https://x.test',
			now
		);
		expect(body.options).toEqual({ bundle: 'zip', paper: 'a5', photo: true, footer: false });
		expect(body.book).toBeNull();
		expect(body.recipes.map((r) => flour(r))).toEqual([
			{ value: 200, display: '200' },
			{ value: 400, display: '400' }
		]);
	});
});
