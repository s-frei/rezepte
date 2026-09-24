import { normaliseTag } from './form';

/**
 * What the editor's tag field offers for the text currently typed:
 * `matches` are existing tags to pick, `create` is the new tag the text
 * would produce - or `null` when there is nothing to create, because the
 * text is blank, already a known tag, or already on the recipe.
 */
export type TagSuggestions = {
	matches: string[];
	create: string | null;
};

/**
 * Splits the known tags into what the typed text matches and what it would
 * create. Pure on purpose: the component only renders the result, so the
 * rules that decide "existing or new" are testable without a DOM.
 *
 * A blank query lists every unselected tag, which is what lets the field
 * open its full list on focus.
 */
export function tagSuggestions(params: {
	query: string;
	known: string[];
	selected: string[];
}): TagSuggestions {
	const prefix = normaliseTag(params.query);
	// Normalized here rather than trusted from the caller, the way
	// `normalize` in ./query.ts does for the same reason: a raw tag name
	// slipping through would not fail loudly, it would match
	// case-sensitively and silently drop suggestions.
	const known = params.known.map((name) => normaliseTag(name));
	const selected = new Set(params.selected.map((name) => normaliseTag(name)));
	const available = known.filter((name) => !selected.has(name));
	const matches = prefix === '' ? available : available.filter((name) => name.startsWith(prefix));

	// Nothing to create when the text is blank, when it already names a known
	// tag (whether or not that one is still available), or when the recipe
	// already carries it - all three would be a duplicate, not a new tag.
	const exists = known.includes(prefix) || selected.has(prefix);
	return { matches, create: prefix === '' || exists ? null : prefix };
}
