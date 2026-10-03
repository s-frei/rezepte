import { redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';
import type { PageLoad } from './$types';

// The owner's page alone: mail is instance-wide, and the API refuses
// everyone else anyway.
export const load: PageLoad = async ({ parent }) => {
	const { user } = await parent();
	if (user?.role !== 'superadmin') redirect(307, resolve('/settings'));
};
