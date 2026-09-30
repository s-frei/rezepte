import type { Ingredient, IngredientGroup } from '$lib/api/recipes';
import { formatQuantityFor } from './scale';

/**
 * The ingredient list as plain text for the clipboard, at the chosen
 * servings. Two shapes: `shoppingList` is one ingredient per line and nothing
 * else, because reminder and to-do apps turn every pasted line into an item;
 * `fullList` is the list as the page shows it, for a message or a note.
 */

function line(ingredient: Ingredient, from: number, to: number, withNote: boolean): string {
	const text = [formatQuantityFor(ingredient.quantity, from, to), ingredient.unit, ingredient.name]
		.filter(Boolean)
		.join(' ');
	return withNote && ingredient.note ? `${text} (${ingredient.note})` : text;
}

/** Every ingredient not checked off, one per line, without groups or notes. */
export function shoppingList(
	groups: IngredientGroup[],
	from: number,
	to: number,
	isChecked: (groupIndex: number, ingredientIndex: number) => boolean
): string {
	return groups
		.flatMap((group, g) =>
			group.ingredients.filter((_, i) => !isChecked(g, i)).map((ing) => line(ing, from, to, false))
		)
		.join('\n');
}

/** Every ingredient with its note, each named group under its name. */
export function fullList(groups: IngredientGroup[], from: number, to: number): string {
	return groups
		.map((group) =>
			[group.name, ...group.ingredients.map((ing) => line(ing, from, to, true))]
				.filter(Boolean)
				.join('\n')
		)
		.join('\n\n');
}
