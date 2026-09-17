import { error } from '@sveltejs/kit';
import { ApiError } from '$lib/api/client';
import { getRecipeBySlug } from '$lib/api/recipes';
import { m } from '$lib/paraglide/messages';
import type { PageLoad } from './$types';

/**
 * Loads the recipe for this slug. Runs in the browser only (`ssr = false`
 * is set for the whole app in the root layout), so it's a plain client
 * fetch rather than something SvelteKit needs to serialise across the
 * network.
 */
export const load: PageLoad = async ({ params }) => {
	try {
		return { recipe: await getRecipeBySlug(params.slug) };
	} catch (err) {
		if (err instanceof ApiError && err.status === 404) {
			error(404, m.not_found());
		}
		throw err;
	}
};
