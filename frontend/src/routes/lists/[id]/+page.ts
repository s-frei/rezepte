import { error } from '@sveltejs/kit';
import { ApiError } from '#lib/api/client.js';
import { getShoppingList } from '#lib/api/shopping.js';
import { m } from '#lib/paraglide/messages.js';
import type { PageLoad } from './$types';

/** One list; an unknown id, or another person's, is SvelteKit's 404 (the server never says which). */
export const load: PageLoad = async ({ params }) => {
	try {
		return { list: await getShoppingList(params.id) };
	} catch (err) {
		if (err instanceof ApiError && err.status === 404) error(404, m.not_found());
		throw err;
	}
};
