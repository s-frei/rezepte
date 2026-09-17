import { describe, expect, it } from 'vitest';
import { overviewViewState } from './overview-state';

function state(overrides: Partial<Parameters<typeof overviewViewState>[0]> = {}) {
	return overviewViewState({
		loading: false,
		q: '',
		tagCount: 0,
		itemCount: 0,
		visibleItemCount: 0,
		total: 0,
		...overrides
	});
}

describe('overviewViewState', () => {
	it('is empty when there are no recipes at all and no filters are active', () => {
		expect(state()).toEqual({ isEmpty: true, isNoResults: false, canLoadMore: false });
	});

	it('is no-results (not empty) when a search matches nothing and there are no more pages', () => {
		expect(state({ q: 'pasta', itemCount: 0, visibleItemCount: 0, total: 0 })).toEqual({
			isEmpty: false,
			isNoResults: true,
			canLoadMore: false
		});
	});

	it('renders the grid with more-to-load when items are visible and more pages exist', () => {
		expect(state({ itemCount: 24, visibleItemCount: 24, total: 50 })).toEqual({
			isEmpty: false,
			isNoResults: false,
			canLoadMore: true
		});
	});

	it('renders the grid without a load-more button on the last page', () => {
		expect(state({ itemCount: 50, visibleItemCount: 50, total: 50 })).toEqual({
			isEmpty: false,
			isNoResults: false,
			canLoadMore: false
		});
	});

	it('regression: offers "Mehr laden" instead of no-results when the client-side multi-tag filter hides everything loaded so far but more pages remain', () => {
		// e.g. two tags selected; the server only filtered by the first, and
		// none of the first page's items also carry the second tag - but
		// there's more to fetch (24 loaded out of 50 total).
		expect(state({ tagCount: 2, itemCount: 24, visibleItemCount: 0, total: 50 })).toEqual({
			isEmpty: false,
			isNoResults: false,
			canLoadMore: true
		});
	});

	it('is no-results once the multi-tag filter hides everything and every page has been loaded', () => {
		expect(state({ tagCount: 2, itemCount: 50, visibleItemCount: 0, total: 50 })).toEqual({
			isEmpty: false,
			isNoResults: true,
			canLoadMore: false
		});
	});

	it('is neither empty nor no-results while loading, regardless of counts', () => {
		expect(state({ loading: true })).toEqual({
			isEmpty: false,
			isNoResults: false,
			canLoadMore: false
		});
		expect(
			state({ loading: true, q: 'pasta', itemCount: 0, visibleItemCount: 0, total: 0 })
		).toEqual({
			isEmpty: false,
			isNoResults: false,
			canLoadMore: false
		});
	});
});
