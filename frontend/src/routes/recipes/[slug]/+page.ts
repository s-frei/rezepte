import { listComments } from '$lib/api/comments';
import { getMyPublicShare } from '$lib/api/shares';
import { getSettings } from '$lib/api/settings';
import { loadRecipeBySlug } from '$lib/recipe/load';
import type { PageLoad } from './$types';

/**
 * Loads the recipe for this slug, plus what the public-share dialog needs
 * to decide its own visibility: the household's settings (is sharing on,
 * what lifetimes) and the caller's own link for this recipe, if any.
 *
 * Settings, the share and the comments all fail soft (`null`) rather than failing the
 * whole page - a recipe is still worth showing without them, just without
 * the "Share publicly…" menu item. Runs in the browser only (`ssr =
 * false` is set for the whole app in the root layout), so this is a plain
 * client fetch rather than something SvelteKit needs to serialize across
 * the network.
 */
export const load: PageLoad = async ({ params }) => {
	const settingsPromise = getSettings().catch(() => null);
	const recipe = await loadRecipeBySlug(params.slug);
	const [settings, share, comments] = await Promise.all([
		settingsPromise,
		getMyPublicShare(recipe.id).catch(() => null),
		listComments(recipe.id).catch(() => null)
	]);
	return { recipe, settings, share, comments };
};
