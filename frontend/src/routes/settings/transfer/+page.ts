import { redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';
import { isAdminRole } from '$lib/roles';
import type { PageLoad } from './$types';

// The first page the settings area hides by role: export and import are
// for admins only, and the API refuses everyone else anyway.
export const load: PageLoad = async ({ parent }) => {
	const { user } = await parent();
	if (!isAdminRole(user?.role)) redirect(307, resolve('/settings'));
};
