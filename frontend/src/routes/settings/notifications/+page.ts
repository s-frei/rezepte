import { getNotificationPrefs } from '#lib/api/notifications.js';
import { getSettings } from '#lib/api/settings.js';
import type { PageLoad } from './$types';

// Waits for the root layout first: without a session it redirects to the
// login with this page as `next`, where an API call of its own would have
// got there first, from the page the browser is still on.
// Both calls fail soft: the card says the switches could not be read, and
// without the settings it leaves out the "mail is not set up" line and the
// owner's hour of the daily new recipes.
export const load: PageLoad = async ({ parent }) => {
	await parent();
	const [prefs, settings] = await Promise.all([
		getNotificationPrefs().catch(() => null),
		getSettings().catch(() => null)
	]);
	return {
		prefs,
		mailEnabled: settings?.mailEnabled ?? true,
		digestHour: settings?.recipeDigestHour ?? null
	};
};
