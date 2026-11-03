import { listShoppingLists } from '#lib/api/shopping.js';

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
	set(n: number) {
		invitations.count = n;
	}
});
