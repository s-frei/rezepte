import { loadRecipeBySlug } from '$lib/recipe/load';
import type { PageLoad } from './$types';

/**
 * Loads the recipe for this slug. Runs in the browser only (`ssr = false`
 * is set for the whole app in the root layout), so it's a plain client
 * fetch rather than something SvelteKit needs to serialize across the
 * network.
 */
export const load: PageLoad = async ({ params }) => {
	return { recipe: await loadRecipeBySlug(params.slug) };
};
