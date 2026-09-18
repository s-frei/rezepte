import { loadRecipeBySlug } from '$lib/recipe/load';
import type { PageLoad } from './$types';

/** Cooking mode needs the whole recipe (steps, ingredients, servings), like the detail page. */
export const load: PageLoad = async ({ params }) => {
	return { recipe: await loadRecipeBySlug(params.slug) };
};
