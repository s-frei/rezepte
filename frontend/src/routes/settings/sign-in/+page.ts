import { requirePage } from '#lib/settings/pages.js';
import type { PageLoad } from './$types';

// The owner's page alone: the provider is instance-wide, and the API refuses
// everyone else anyway.
export const load: PageLoad = ({ parent }) => requirePage('sign-in', parent);
