import { isAdminRole } from '$lib/roles';
import type { PageLoad } from './$types';

/**
 * Open to every member: any session may call the API and read the spec this
 * page points at. Only token management is admin-only, and the page gates
 * that section on this flag rather than repeating the role check in markup.
 * `parent()` waits for the root layout's load, which has resolved the
 * session by then.
 */
export const load: PageLoad = async ({ parent }) => {
	const { user } = await parent();
	return { isAdmin: isAdminRole(user?.role) };
};
