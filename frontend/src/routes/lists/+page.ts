import { isSignedOut } from '#lib/api/client.js';
import { listShoppingLists } from '#lib/api/shopping.js';
import type { PageLoad } from './$types';

/**
 * The lists overview, read before the page shows, as the plans side's is: a
 * switch from the plans keeps them on screen until the lists are there. A
 * failure is the page's to say (`failed`), with Retry: a load also runs to
 * preload a link, so it never toasts, and a 401 is the page's to redirect
 * (`signedOut`), since a preload runs while the page being left is the address.
 */
export const load: PageLoad = async () => {
	try {
		return { ...(await listShoppingLists({ redirect: false })), failed: false, signedOut: false };
	} catch (error) {
		return { items: [], invitations: [], failed: true, signedOut: isSignedOut(error) };
	}
};
