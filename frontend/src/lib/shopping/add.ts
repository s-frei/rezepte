import type { IngredientGroup } from '#lib/api/recipes.js';
import type { ShoppingSummary } from '#lib/api/shopping.js';
import { ingredientKey } from '#lib/recipe/checked.js';

/** Selection keys use the same `group:index` shape as the recipe page's checks. */
export const pickKey = (group: number, index: number) => `${group}:${index}`;

/** Everything not yet ticked on the recipe page: what is still to buy. */
export function preselected(
	recipeId: string,
	groups: IngredientGroup[],
	isChecked: (key: string) => boolean
): Set<string> {
	const picked = new Set<string>();
	groups.forEach((group, g) =>
		group.ingredients.forEach((_, i) => {
			if (!isChecked(ingredientKey(recipeId, g, i))) picked.add(pickKey(g, i));
		})
	);
	return picked;
}

/** The request body's ingredient refs, in recipe order. */
export function pickedRefs(picked: Set<string>) {
	return [...picked]
		.map((key) => key.split(':').map(Number))
		.map(([group, index]) => ({ group, index }))
		.sort((a, b) => a.group - b.group || a.index - b.index);
}

/** The remembered list if it still exists, else the first - the server sends newest first. */
export function defaultList(lists: ShoppingSummary[], lastId: string | null): string {
	return lists.find((l) => l.id === lastId)?.id ?? lists[0]?.id ?? '';
}
