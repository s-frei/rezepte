import { error } from '@sveltejs/kit';
import { ApiError } from '#lib/api/client.js';
import { listPlans } from '#lib/api/mealplan.js';
import { getShoppingList } from '#lib/api/shopping.js';
import { m } from '#lib/paraglide/messages.js';
import type { PageLoad } from './$types';

/**
 * One list; an unknown id, or another person's, is SvelteKit's 404 (the
 * server never says which). A list holding planned entries also brings the
 * reader's plans, so "aus der Woche" links only to a plan they are in;
 * they are asked for alongside the list, so the link costs no round trip.
 */
export const load: PageLoad = async ({ params }) => {
	const plans = myPlans();
	try {
		const list = await getShoppingList(params.id);
		return { list, plans: list.entries.some((e) => e.planId) ? await plans : null };
	} catch (err) {
		if (err instanceof ApiError && err.status === 404) error(404, m.not_found());
		throw err;
	}
};

/** The ids of the reader's plans; null when they cannot be read, so the link just stays away. */
async function myPlans(): Promise<string[] | null> {
	try {
		return (await listPlans()).plans.map((p) => p.id);
	} catch {
		return null;
	}
}
