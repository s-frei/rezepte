import { listDeviceAccounts, safeNext } from '#lib/api/auth.js';
import type { PageLoad } from './$types';

// Opens with nobody active, too (after "Sign out" with others on the
// device): the device's accounts come from its own cookie, not a session.
// `next` is where a signed-out moment left off (a deep link, a push tap
// after the session ran out); the switch lands there.
export const load: PageLoad = async ({ parent, url }) => {
	const { user } = await parent();
	return {
		user,
		accounts: await listDeviceAccounts(),
		next: safeNext(url.searchParams.get('next'))
	};
};
