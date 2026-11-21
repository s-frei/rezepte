import { describe, expect, it } from 'vitest';
import type { IngredientGroup } from '#lib/api/recipes.js';
import { pickedRefs, preselected } from './add';

const groups = [
	{ name: 'A', ingredients: [{}, {}] },
	{ name: 'B', ingredients: [{}] }
] as unknown as IngredientGroup[];

describe('add to list', () => {
	it('preselects what is not checked on the recipe', () => {
		const picked = preselected('r1', groups, (k) => k === 'r1:0:1');
		expect([...picked]).toEqual(['0:0', '1:0']);
	});
	it('orders refs by recipe position', () => {
		expect(pickedRefs(groups, new Set(['1:0', '0:1', '0:0']))).toEqual([
			{ group: 0, index: 0 },
			{ group: 0, index: 1 },
			{ group: 1, index: 0 }
		]);
	});
});
