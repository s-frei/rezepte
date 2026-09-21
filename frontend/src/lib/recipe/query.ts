import { normaliseTag } from './form';
import { snapMaxMinutes } from './time-filter';

/** The three orders the overview grid can be sorted by. */
export type Sort = 'updated' | 'created' | 'title';

const SORTS: readonly Sort[] = ['updated', 'created', 'title'];

/**
 * Narrows a raw string to `Sort` iff it is one of the three known values -
 * exported so the sort `Select`'s `onchange` (which only ever hands back a
 * plain string, one of `sortOptions`' own values) can narrow it the same
 * way `normalise` does, without duplicating the check.
 */
export function isSort(value: string): value is Sort {
	return SORTS.some((sort) => sort === value);
}

/** Overview list state that round-trips through the page's URL query string. */
export type ListQuery = {
	q: string;
	tags: string[];
	/** Maximum total time in minutes, 0 = off. */
	maxMinutes: number;
	/** Restrict to the caller's own favourites, off by default. */
	favourites: boolean;
	/** Restrict to recipes written by this username, `''` = off. */
	author: string;
	/** Result order, `'updated'` (most recently changed first) is the default. */
	sort: Sort;
	page: number;
};

/**
 * Brings list state into the one shape both functions below agree on:
 *
 * - `q` is trimmed.
 * - every tag is normalised the way the editor normalises a typed tag
 *   (trimmed and lower-cased, see `normaliseTag`), and blank ones are dropped.
 * - `author` is trimmed but keeps its case: the service stores usernames as
 *   they were typed (it only trims them too), so lower-casing here the way
 *   tags are would stop "Mara" from matching anything.
 * - `sort` falls back to `'updated'` (the default) for anything outside the
 *   three known values - the same rule the server applies, so a value that
 *   can only arrive from a stale bookmark degrades to the default list
 *   rather than failing to parse.
 * - `maxMinutes` snaps to one of the filter's own stops (see `time-filter`),
 *   rounding up, with anything past the last one - and anything that isn't a
 *   positive number - reading as off. The control offers exactly those stops,
 *   so holding the URL to them keeps a hand-edited or stale link a state the
 *   control can show and label truthfully.
 * - `page` falls back to `1` for anything that isn't a positive integer.
 *
 * Doing it here rather than in `parseListQuery` alone is what makes
 * `buildListQuery(state)` and `buildListQuery(parseListQuery(url))` equal
 * whenever the two describe the same list. The overview relies on that: it
 * compares the query string it last wrote with the one the current URL
 * parses back to, and a stray space or a capital letter would otherwise read
 * as "the URL changed under me".
 *
 * `state.sort` is typed as a plain `string`, wider than `ListQuery`'s own
 * `Sort` - unlike every other field, a raw URL value can't already claim
 * that narrower type, and this is what lets `parseListQuery` hand it
 * `searchParams.get('sort') ?? ''` straight through without a cast.
 */
function normalise(state: Omit<ListQuery, 'sort'> & { sort: string }): ListQuery {
	return {
		q: state.q.trim(),
		tags: state.tags.map((tag) => normaliseTag(tag)).filter((tag) => tag.length > 0),
		maxMinutes: snapMaxMinutes(state.maxMinutes),
		favourites: Boolean(state.favourites),
		author: state.author.trim(),
		sort: isSort(state.sort) ? state.sort : 'updated',
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
		// which `normalise` turns into 0 (off).
		maxMinutes: Number(url.searchParams.get('maxMinutes')),
		favourites: url.searchParams.get('favourites') === 'true',
		author: url.searchParams.get('author') ?? '',
		sort: url.searchParams.get('sort') ?? '',
		// Number(null) and Number('') are 0, Number('abc') is NaN - all of
		// which `normalise` turns into page 1.
		page: Number(url.searchParams.get('page'))
	});
}

/**
 * Serialises list state into a URL query string (no leading `?`), omitting
 * fields at their default value and keeping a stable `q`, `tags`,
 * `maxMinutes`, `favourites`, `author`, `sort`, `page` order so the
 * resulting URL is predictable and diff-friendly.
 */
export function buildListQuery(state: ListQuery): string {
	const { q, tags, maxMinutes, favourites, author, sort, page } = normalise(state);
	const params = new URLSearchParams();
	if (q) {
		params.set('q', q);
	}
	if (tags.length > 0) {
		params.set('tags', tags.join(','));
	}
	if (maxMinutes > 0) {
		params.set('maxMinutes', String(maxMinutes));
	}
	if (favourites) {
		params.set('favourites', 'true');
	}
	if (author) {
		params.set('author', author);
	}
	if (sort !== 'updated') {
		params.set('sort', sort);
	}
	if (page !== 1) {
		params.set('page', String(page));
	}
	return params.toString();
}
