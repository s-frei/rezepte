import type { ReadonlyURL } from '$app/state';
import { normalizeTag } from './form';
import { ingredientKey } from './key';
import { snapMaxMinutes } from './time-filter';

/**
 * The orders the overview grid can be sorted by. `''` is the default order,
 * which the URL leaves out: the newest created first, or the best matches
 * first while a search text or ingredient chips narrow the list (see
 * `serverSort`). It has a value of its own because it is neither: with a
 * search, "newest first" is a choice of its own, and so is "recently
 * changed" without one.
 */
export type Sort = '' | 'updated' | 'created' | 'title' | 'tasty';

/** The default order, see `Sort`. */
export const DEFAULT_SORT = '';

const SORTS: readonly Sort[] = [DEFAULT_SORT, 'updated', 'created', 'title', 'tasty'];

/**
 * Narrows a raw string to `Sort` iff it is one of the known values -
 * exported so the sort `Select`'s `onchange` (which only ever hands back a
 * plain string, one of `sortOptions`' own values) can narrow it the same
 * way `normalize` does, without duplicating the check.
 */
export function isSort(value: string): value is Sort {
	return SORTS.some((sort) => sort === value);
}

/**
 * The order the API is asked for. The default is newest created first; with
 * a search text or chips the API's own default, `updated`, which ranks the
 * best matches first then (most recently changed only breaks ties).
 */
export function serverSort(sort: Sort, searching: boolean): Exclude<Sort, ''> {
	if (sort !== DEFAULT_SORT) {
		return sort;
	}
	return searching ? 'updated' : 'created';
}

/**
 * The option the sort control shows: an explicit order that comes out the
 * same as the default reads as the default, so "newest first" without a
 * search, or "recently changed" with one (the API ranks it as best matches),
 * is never offered twice.
 */
export function shownSort(sort: Sort, searching: boolean): Sort {
	return sort === serverSort(DEFAULT_SORT, searching) ? DEFAULT_SORT : sort;
}

/** Overview list state that round-trips through the page's URL query string. */
export type ListQuery = {
	q: string;
	tags: string[];
	/** Ingredient chips: recipes are ranked by how many of them they use. */
	ingredients: string[];
	/** Pantry staples: they count as in stock but are not chips. */
	have: string[];
	/** Maximum total time in minutes, 0 = off. */
	maxMinutes: number;
	/** Restrict to the caller's own favorites, off by default. */
	favorites: boolean;
	/** Restrict to recipes written by this username, `''` = off. */
	author: string;
	/** Result order, `DEFAULT_SORT` unless one was chosen. */
	sort: Sort;
	page: number;
};

/** The most chips the server ranks by, its `maxChips`. */
export const MAX_CHIPS = 20;

/**
 * Trimmed names without blanks or repeats by ingredient key ("Zwiebel" and
 * "Zwiebeln" are one, as on the server), none whose key is in `skip`.
 */
function names(list: string[], skip: Set<string> = new Set()): string[] {
	const seen = new Set(skip);
	return list
		.map((name) => name.trim())
		.filter((name) => {
			const key = ingredientKey(name);
			if (key === '' || seen.has(key)) {
				return false;
			}
			seen.add(key);
			return true;
		});
}

/**
 * Brings list state into the one shape both functions below agree on:
 *
 * - `q` is trimmed.
 * - every tag is normalized the way the editor normalizes a typed tag
 *   (trimmed and lower-cased, see `normalizeTag`), and blank ones are dropped.
 * - `ingredients` and `have` keep their names as typed (trimmed), dropping
 *   blanks and repeats by ingredient key; a name in both stays a chip only,
 *   and the chips stop at `MAX_CHIPS`, as the server's do.
 * - `author` is trimmed but keeps its case: the service stores usernames as
 *   they were typed (it only trims them too), so lower-casing here the way
 *   tags are would stop "Mara" from matching anything.
 * - `sort` falls back to `DEFAULT_SORT` for anything outside the
 *   known values - as the server falls back to its own, so a value that
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
function normalize(state: Omit<ListQuery, 'sort'> & { sort: string }): ListQuery {
	const ingredients = names(state.ingredients).slice(0, MAX_CHIPS);
	return {
		q: state.q.trim(),
		tags: state.tags.map((tag) => normalizeTag(tag)).filter((tag) => tag.length > 0),
		ingredients,
		have: names(state.have, new Set(ingredients.map(ingredientKey))),
		maxMinutes: snapMaxMinutes(state.maxMinutes),
		favorites: Boolean(state.favorites),
		author: state.author.trim(),
		sort: isSort(state.sort) ? state.sort : DEFAULT_SORT,
		page: Number.isInteger(state.page) && state.page >= 1 ? state.page : 1
	};
}

/**
 * Parses the overview page's URL query string into list state. Missing
 * params render as their default (`''`, `[]`, `1`); `tags` comes from a
 * single comma-separated `tags` param, `ingredients` and `have` from one
 * param per name, since an ingredient's name may hold a comma ("Milch 1,5 %"). Everything is normalized, so
 * `?tags=Dessert,+SÜSS+` and `?tags=dessert,süss` parse to the same state.
 */
export function parseListQuery(url: ReadonlyURL): ListQuery {
	return normalize({
		q: url.searchParams.get('q') ?? '',
		tags: (url.searchParams.get('tags') ?? '').split(','),
		// Number(null) and Number('') are 0, Number('abc') is NaN - all of
		// which `normalize` turns into 0 (off).
		ingredients: url.searchParams.getAll('ingredients'),
		have: url.searchParams.getAll('have'),
		maxMinutes: Number(url.searchParams.get('maxMinutes')),
		favorites: url.searchParams.get('favorites') === 'true',
		author: url.searchParams.get('author') ?? '',
		sort: url.searchParams.get('sort') ?? '',
		// Number(null) and Number('') are 0, Number('abc') is NaN - all of
		// which `normalize` turns into page 1.
		page: Number(url.searchParams.get('page'))
	});
}

/**
 * Serializes list state into a URL query string (no leading `?`), omitting
 * fields at their default value and keeping a stable `q`, `tags`,
 * `ingredients`, `have`, `maxMinutes`, `favorites`, `author`, `sort`, `page` order so the
 * resulting URL is predictable and diff-friendly.
 */
export function buildListQuery(state: ListQuery): string {
	const { q, tags, ingredients, have, maxMinutes, favorites, author, sort, page } =
		normalize(state);
	const params = new URLSearchParams();
	if (q) {
		params.set('q', q);
	}
	if (tags.length > 0) {
		params.set('tags', tags.join(','));
	}
	for (const name of ingredients) {
		params.append('ingredients', name);
	}
	for (const name of have) {
		params.append('have', name);
	}
	if (maxMinutes > 0) {
		params.set('maxMinutes', String(maxMinutes));
	}
	if (favorites) {
		params.set('favorites', 'true');
	}
	if (author) {
		params.set('author', author);
	}
	if (sort !== DEFAULT_SORT) {
		params.set('sort', sort);
	}
	if (page !== 1) {
		params.set('page', String(page));
	}
	return params.toString();
}
