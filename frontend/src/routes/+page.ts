import { parseListQuery } from '$lib/recipe/query';
import type { PageLoad } from './$types';

// Parsing only - no fetching here. The page fetches recipes client-side
// (via `$lib/api/recipes`) so it can show its skeleton grid while the
// request is in flight; `load` just hands it the URL's initial filter state.
export const load: PageLoad = ({ url }) => parseListQuery(url);
