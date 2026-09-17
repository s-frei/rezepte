import { normaliseTag } from './form';

/** Overview list state that round-trips through the page's URL query string. */
export type ListQuery = {
	q: string;
	tags: string[];
	page: number;
};

/**
 * Brings list state into the one shape both functions below agree on:
 *
 * - `q` is trimmed.
 * - every tag is normalised the way the editor normalises a typed tag
 *   (trimmed and lower-cased, see `normaliseTag`), and blank ones are dropped.
 * - `page` falls back to `1` for anything that isn't a positive integer.
 *
 * Doing it here rather than in `parseListQuery` alone is what makes
 * `buildListQuery(state)` and `buildListQuery(parseListQuery(url))` equal
 * whenever the two describe the same list. The overview relies on that: it
 * compares the query string it last wrote with the one the current URL
 * parses back to, and a stray space or a capital letter would otherwise read
 * as "the URL changed under me".
 */
function normalise(state: ListQuery): ListQuery {
	return {
		q: state.q.trim(),
		tags: state.tags.map((tag) => normaliseTag(tag)).filter((tag) => tag.length > 0),
		page: Number.isInteger(state.page) && state.page >= 1 ? state.page : 1
	};
}

/**
 * Parses the overview page's URL query string into list state. Missing
 * params render as their default (`''`, `[]`, `1`); `tags` comes from a
 * single comma-separated `tags` param. Everything is normalised, so
 * `?tags=Dessert,+SÜSS+` and `?tags=dessert,süss` parse to the same state.
 */
export function parseListQuery(url: URL): ListQuery {
	return normalise({
		q: url.searchParams.get('q') ?? '',
		tags: (url.searchParams.get('tags') ?? '').split(','),
		// Number(null) and Number('') are 0, Number('abc') is NaN - all of
		// which `normalise` turns into page 1.
		page: Number(url.searchParams.get('page'))
	});
}

/**
 * Serialises list state into a URL query string (no leading `?`), omitting
 * fields at their default value and keeping a stable `q`, `tags`, `page`
 * order so the resulting URL is predictable and diff-friendly.
 */
export function buildListQuery(state: ListQuery): string {
	const { q, tags, page } = normalise(state);
	const params = new URLSearchParams();
	if (q) {
		params.set('q', q);
	}
	if (tags.length > 0) {
		params.set('tags', tags.join(','));
	}
	if (page !== 1) {
		params.set('page', String(page));
	}
	return params.toString();
}
