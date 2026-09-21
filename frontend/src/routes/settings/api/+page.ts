import { redirect } from '@sveltejs/kit';
import { isAdminRole } from '$lib/roles';
import type { PageLoad } from './$types';

/**
 * Admin-only page. `parent()` waits for the root layout's load, which has
 * resolved the session by then; a member who types the URL lands on their
 * own settings instead. The API enforces the same rule (403), this is only
 * the friendly half.
 */
export const load: PageLoad = async ({ parent }) => {
	const { user } = await parent();
	if (!isAdminRole(user?.role)) {
		redirect(302, '/settings');
	}
	return {};
};
