import { describe, expect, it } from 'vitest';
import { overviewViewState } from './overview-state';

function state(overrides: Partial<Parameters<typeof overviewViewState>[0]> = {}) {
	return overviewViewState({
		loading: false,
		filtered: false,
		itemCount: 0,
		total: 0,
		...overrides
	});
}

describe('overviewViewState', () => {
	it('is empty when there are no recipes at all and no filters are active', () => {
		expect(state()).toEqual({ isEmpty: true, isNoResults: false, canLoadMore: false });
	});

	it('is no-results (not empty) when a search matches nothing', () => {
		expect(state({ filtered: true, itemCount: 0, total: 0 })).toEqual({
			isEmpty: false,
			isNoResults: true,
			canLoadMore: false
		});
	});

	it('reports no results when a filter matches nothing', () => {
		expect(overviewViewState({ loading: false, filtered: true, itemCount: 0, total: 0 })).toEqual({
			isEmpty: false,
			isNoResults: true,
			canLoadMore: false
		});
	});

	// Pins the bug fixed alongside this generalisation: overviewViewState used
	// to take `q`/`tagCount` separately, so a filter this function did not
	// know about yet (maximum time) fell through as "no filter active" and a
	// time-only search with zero matches showed the "create your first
	// recipe" empty state instead of "no results". A single `filtered` flag
	// means the next filter is one clause where the page derives it, not
	// another parameter here.
	it('reports no results, not empty, when any filter narrows to zero matches', () => {
		expect(state({ filtered: true, total: 0, itemCount: 0 })).toEqual({
			isEmpty: false,
			isNoResults: true,
			canLoadMore: false
		});
	});

	it('renders the grid with more-to-load when items are visible and more pages exist', () => {
		expect(state({ itemCount: 24, total: 50 })).toEqual({
			isEmpty: false,
			isNoResults: false,
			canLoadMore: true
		});
	});

	it('renders the grid without a load-more button on the last page', () => {
		expect(state({ itemCount: 50, total: 50 })).toEqual({
			isEmpty: false,
			isNoResults: false,
			canLoadMore: false
		});
	});

	it('pins the caller invariant: an empty page never carries a nonzero total', () => {
		// The function is pure and cannot verify `itemCount === 0 implies
		// total === 0` itself - `fetchPage` in +page.svelte guarantees it by
		// always setting `items` and `total` from the same response. Fed a
		// combination that invariant rules out, this is what comes back:
		// misclassified as no-results (and, incoherently, loadable further)
		// rather than a loaded page of five.
		expect(state({ itemCount: 0, total: 5 })).toEqual({
			isEmpty: false,
			isNoResults: true,
			canLoadMore: true
		});
	});

	it('is neither empty nor no-results while loading, regardless of counts', () => {
		expect(state({ loading: true })).toEqual({
			isEmpty: false,
			isNoResults: false,
			canLoadMore: false
		});
		expect(state({ loading: true, filtered: true, itemCount: 0, total: 0 })).toEqual({
			isEmpty: false,
			isNoResults: false,
			canLoadMore: false
		});
	});
});
