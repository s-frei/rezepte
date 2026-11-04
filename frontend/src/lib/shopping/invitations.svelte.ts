import { listShoppingLists } from '#lib/api/shopping.js';
import { m } from '#lib/paraglide/messages.js';

/** How many shopping-list invitations wait for an answer; drives the nav dot. */
export const invitations = $state({
	count: 0,
	/** A failed refresh leaves the count as it was. */
	async refresh() {
		try {
			invitations.count = (await listShoppingLists()).invitations.length;
		} catch {
			/* silent */
		}
	},
	/** "1 invitation", "3 invitations": the nav's screen-reader text. */
	get label() {
		return invitations.count === 1
			? m.nav_lists_invitations_one()
			: m.nav_lists_invitations({ count: invitations.count });
	}
});
