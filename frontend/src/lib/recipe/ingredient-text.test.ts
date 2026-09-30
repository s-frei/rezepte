import { describe, expect, it, vi } from 'vitest';
import type { IngredientGroup } from '$lib/api/recipes';
import { fullList, shoppingList } from './ingredient-text';

vi.mock('$lib/paraglide/runtime', () => ({
	getLocale: () => 'en',
	experimentalStaticLocale: undefined
}));

const groups: IngredientGroup[] = [
	{
		name: 'Dough',
		ingredients: [
			{ quantity: 250, unit: 'g', name: 'Flour', note: 'sifted' },
			{ quantity: 2, unit: null, name: 'Eggs', note: null }
		]
	},
	{
		name: 'Filling',
		ingredients: [
			{ quantity: null, unit: null, name: 'Salt', note: 'a pinch' },
			{ quantity: 0.5, unit: 'tsp', name: 'Cinnamon', note: null }
		]
	}
];

const none = () => false;

describe('shoppingList', () => {
	it('lists every open ingredient flat, without group names or notes', () => {
		expect(shoppingList(groups, 4, 4, none)).toBe('250 g Flour\n2 Eggs\nSalt\n½ tsp Cinnamon');
	});

	it('leaves out what is checked', () => {
		const checked = (g: number, i: number) => g === 0 && i === 0;
		expect(shoppingList(groups, 4, 4, checked)).toBe('2 Eggs\nSalt\n½ tsp Cinnamon');
	});

	it('scales the quantities to the chosen servings', () => {
		expect(shoppingList(groups, 4, 8, none)).toBe('500 g Flour\n4 Eggs\nSalt\n1 tsp Cinnamon');
	});

	it('is empty when everything is checked', () => {
		expect(shoppingList(groups, 4, 4, () => true)).toBe('');
	});
});

describe('fullList', () => {
	it('keeps group names and notes, a blank line between groups', () => {
		expect(fullList(groups, 4, 4)).toBe(
			'Dough\n250 g Flour (sifted)\n2 Eggs\n\nFilling\nSalt (a pinch)\n½ tsp Cinnamon'
		);
	});

	it('writes an unnamed group without a heading', () => {
		const plain: IngredientGroup[] = [
			{ name: null, ingredients: [{ quantity: 1, unit: null, name: 'Onion', note: null }] }
		];
		expect(fullList(plain, 2, 2)).toBe('1 Onion');
	});
});
