import { normaliseTag } from './form';

/**
 * What the editor's tag field offers for the text currently typed:
 * `matches` are existing tags to pick, `create` is the new tag the text
 * would produce - or `null` when there is nothing to create, because the
 * text is blank, already a known tag, or already on the recipe.
 * `options` are the rows in the order the list shows them: the matches,
 * then creating, the last station. `preferred` is the row Enter (and a
 * comma) takes: the first match, creating only when nothing matches, and
 * none (-1) while the text is blank.
 */
export type TagSuggestions = {
	matches: string[];
	create: string | null;
	options: { name: string; isNew: boolean }[];
	preferred: number;
};

/**
 * Splits the known tags into what the typed text matches and what it would
 * create. Pure on purpose: the component only renders the result, so the
 * rules that decide "existing or new" are testable without a DOM.
 *
 * A blank query lists every unselected tag, which is what lets the field
 * open its full list on focus. Otherwise the tags that contain the text are
 * ranked as the overview's search ranks them (`rankTags`), except that a tag
 * named exactly by it leads.
 */
export function tagSuggestions(params: {
	query: string;
	known: string[];
	selected: string[];
}): TagSuggestions {
	const typed = normaliseTag(params.query);
	// Normalized here rather than trusted from the caller, the way
	// `normalize` in ./query.ts does for the same reason: a raw tag name
	// slipping through would not fail loudly, it would match
	// case-sensitively and silently drop suggestions.
	const known = params.known.map((name) => normaliseTag(name));
	const selected = new Set(params.selected.map((name) => normaliseTag(name)));
	const available = known.filter((name) => !selected.has(name));
	const ranked = rankTags(
		available.map((name) => ({ name })),
		typed
	).map((tag) => tag.name);
	const matches = ranked.includes(typed)
		? [typed, ...ranked.filter((name) => name !== typed)]
		: ranked;

	// Nothing to create when the text is blank, when it already names a known
	// tag (whether or not that one is still available), or when the recipe
	// already carries it - all three would be a duplicate, not a new tag.
	const exists = known.includes(typed) || selected.has(typed);
	const create = typed === '' || exists ? null : typed;
	const options = [
		...matches.map((name) => ({ name, isNew: false })),
		...(create ? [{ name: create, isNew: true }] : [])
	];
	return { matches, create, options, preferred: typed !== '' && options.length > 0 ? 0 : -1 };
}

/**
 * The tag a key commits, or `null`. A tag goes on only by an explicit
 * choice: Enter or a comma while a row is highlighted takes that row - an
 * existing tag, or the creation row. Tab and leaving the field commit
 * nothing, so typed text never turns into a tag by walking away from it.
 */
export function tagToCommit(
	trigger: 'enter' | 'comma' | 'tab' | 'blur',
	options: TagSuggestions['options'],
	highlighted: number
): string | null {
	if (trigger !== 'enter' && trigger !== 'comma') {
		return null;
	}
	return options[highlighted]?.name ?? null;
}

/**
 * What the overview's search field offers for the text typed so far: every
 * tag that contains it, the ones that start with it first. Within each half
 * the given order is kept, which for `listTags()` is most used first, so a
 * blank query lists every tag in that order.
 */
export function rankTags<T extends { name: string }>(tags: T[], query: string): T[] {
	const needle = query.trim().toLocaleLowerCase();
	if (needle === '') {
		return tags;
	}
	const starts: T[] = [];
	const contains: T[] = [];
	for (const tag of tags) {
		const name = tag.name.toLocaleLowerCase();
		if (name.startsWith(needle)) {
			starts.push(tag);
		} else if (name.includes(needle)) {
			contains.push(tag);
		}
	}
	return [...starts, ...contains];
}
