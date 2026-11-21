import { isSignedOut } from '#lib/api/client.js';
import { listShoppingLists } from '#lib/api/shopping.js';
import type { PageLoad } from './$types';

/**
 * The lists overview, read before the page shows, as the plans side's is: a
 * switch from the plans keeps them on screen until the lists are there. A
 * failure is the page's to say (`failed`), with Retry: a load also runs to
 * preload a link, so it never toasts.
 */
export const load: PageLoad = async () => {
	try {
		return { ...(await listShoppingLists()), failed: false, signedOut: false };
	} catch (error) {
		return { items: [], invitations: [], failed: true, signedOut: isSignedOut(error) };
	}
};
