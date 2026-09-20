export type OverviewViewState = {
	isEmpty: boolean;
	isNoResults: boolean;
	canLoadMore: boolean;
};

/**
 * Derives the overview page's view state from its raw counts.
 *
 * Pulled out of `+page.svelte`'s `$derived`s so the boundary conditions
 * below have a unit test.
 *
 * Assumes the caller's invariant `itemCount === 0 implies total === 0`: a
 * page of results and its total always come from the same fetch, so an
 * empty page never carries a nonzero total. This function is pure and
 * cannot check that itself - given `itemCount: 0, total: 5` it reports
 * `isNoResults: true`, which is only correct because the caller never
 * produces that combination.
 */
export function overviewViewState(params: {
	loading: boolean;
	/** True when a search term or any filter narrows the list. */
	filtered: boolean;
	itemCount: number;
	total: number;
}): OverviewViewState {
	const isEmpty = !params.loading && params.total === 0 && !params.filtered;

	const isNoResults = !params.loading && !isEmpty && params.itemCount === 0;

	const canLoadMore = !isEmpty && params.itemCount < params.total;

	return { isEmpty, isNoResults, canLoadMore };
}
