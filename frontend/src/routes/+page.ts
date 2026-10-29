import { parseListQuery } from '#lib/recipe/query.js';
import { applyRememberedSort, readSort } from '#lib/recipe/sort-preference.js';
import type { PageLoad } from './$types';

// Parsing only - no fetching here. The page fetches recipes client-side
// (via `#lib/api/recipes`) so it can show its skeleton grid while the
// request is in flight; `load` just hands it the URL's initial filter state,
// with the remembered order where the URL names none.
export const load: PageLoad = ({ url }) =>
	applyRememberedSort(parseListQuery(url), url, readSort());
