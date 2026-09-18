import { error } from '@sveltejs/kit';
import { ApiError } from '$lib/api/client';
import { getRecipeBySlug, type Recipe } from '$lib/api/recipes';
import { m } from '$lib/paraglide/messages';

/**
 * The `load` body shared by the detail, edit and cook routes: fetch by slug
 * (client-side only - `ssr = false` is set for the whole app) and turn an
 * unknown slug into SvelteKit's 404, which the root `+error.svelte` renders.
 * Any other failure propagates.
 */
export async function loadRecipeBySlug(slug: string): Promise<Recipe> {
	try {
		return await getRecipeBySlug(slug);
	} catch (err) {
		if (err instanceof ApiError && err.status === 404) {
			error(404, m.not_found());
		}
		throw err;
	}
}
