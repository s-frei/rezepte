import { requirePage } from '#lib/settings/pages.js';
import type { PageLoad } from './$types';

// The first page the settings area hides by role: export and import are
// for admins only, and the API refuses everyone else anyway.
export const load: PageLoad = ({ parent }) => requirePage('transfer', parent);
