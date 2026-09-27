import { getPublicRecipe } from '$lib/api/public';
import type { PageLoad } from './$types';

/**
 * Loads the recipe behind this public link, or null when the token serves
 * nothing. Runs in the browser only (`ssr = false` is set for the whole app
 * in the root layout). Never throws into SvelteKit's error page: a stranger
 * with a dead link gets this route's own friendly "not available" state.
 */
export const load: PageLoad = async ({ params }) => {
	return { token: params.token, recipe: await getPublicRecipe(params.token) };
};
