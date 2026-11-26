import { getSettings } from '#lib/api/settings.js';
import { requirePage } from '#lib/settings/pages.js';
import type { PageLoad } from './$types';

// The first page the settings area hides by role: export and import are
// for admins only, and the API refuses everyone else anyway. The settings
// say whether the Export card offers "As PDF"; without them it does not.
export const load: PageLoad = async ({ parent }) => {
	const settings = getSettings().catch(() => null);
	await requirePage('transfer', parent);
	return { settings: await settings };
};
