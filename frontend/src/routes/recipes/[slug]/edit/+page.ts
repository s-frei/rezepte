import { error } from '@sveltejs/kit';
import { ApiError } from '$lib/api/client';
import { getRecipeBySlug } from '$lib/api/recipes';
import { m } from '$lib/paraglide/messages';
import type { PageLoad } from './$types';

/** Loads the recipe to edit. Client-side only, like the detail page's load. */
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
