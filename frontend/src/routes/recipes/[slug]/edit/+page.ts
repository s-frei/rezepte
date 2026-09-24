import { redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';
import { loadRecipeBySlug } from '$lib/recipe/load';
import type { PageLoad } from './$types';

/**
 * Loads the recipe to edit. Client-side only, like the detail page's load.
 * Someone who may not edit it (a link, a bookmark, the back button) lands on
 * the recipe instead of an editor that would refuse to save.
 */
export const load: PageLoad = async ({ params }) => {
	const recipe = await loadRecipeBySlug(params.slug);
	if (!recipe.canEdit) {
		redirect(307, resolve('/recipes/[slug]', { slug: recipe.slug }));
	}
	return { recipe };
};
