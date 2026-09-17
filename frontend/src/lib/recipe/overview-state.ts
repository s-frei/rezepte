export type OverviewViewState = {
	isEmpty: boolean;
	isNoResults: boolean;
	canLoadMore: boolean;
};

/**
 * Derives the overview page's view state from its raw counts.
 *
 * Pulled out of `+page.svelte`'s `$derived`s so the tricky boundary
 * condition below has a unit test: more pages may still exist server-side
 * even while the *visible* list (after the client-side multi-tag filter,
 * see the "Ruling: multi-tag filter" ledger note) is momentarily empty.
 */
export function overviewViewState(params: {
	loading: boolean;
	q: string;
	tagCount: number;
	itemCount: number;
	visibleItemCount: number;
	total: number;
}): OverviewViewState {
	const isEmpty = !params.loading && params.total === 0 && !params.q && params.tagCount === 0;

	// "No results" only once no more pages are left to try. While
	// `itemCount < total`, a page not yet loaded might still contain an item
	// matching every selected tag, so "Mehr laden" - not the no-results
	// panel - is the correct affordance even when nothing currently loaded
	// is visible.
	const isNoResults =
		!params.loading &&
		!isEmpty &&
		params.visibleItemCount === 0 &&
		params.itemCount >= params.total;

	const canLoadMore = !isEmpty && params.itemCount < params.total;

	return { isEmpty, isNoResults, canLoadMore };
}
