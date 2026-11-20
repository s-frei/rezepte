import { listPlans } from '#lib/api/mealplan.js';
import { listShoppingLists } from '#lib/api/shopping.js';
import { m } from '#lib/paraglide/messages.js';

/** "1 invitation", "3 invitations": the screen-reader text of a dot. */
export const invitationsLabel = (count: number) =>
	count === 1 ? m.nav_lists_invitations_one() : m.nav_lists_invitations({ count });

/**
 * How many invitations wait for an answer, to lists and to plans: the place's
 * dot counts both, the switch's Lists and Week segments each their own.
 */
export const invitations = $state({
	lists: 0,
	plans: 0,
	get count() {
		return invitations.lists + invitations.plans;
	},
	/** A failed half leaves its count as it was. */
	async refresh() {
		const [lists, plans] = await Promise.allSettled([listShoppingLists(), listPlans()]);
		if (lists.status === 'fulfilled') invitations.lists = lists.value.invitations.length;
		if (plans.status === 'fulfilled') invitations.plans = plans.value.invitations.length;
	},
	get label() {
		return invitationsLabel(invitations.count);
	}
});
