import { normalizeTag } from './form';

/**
 * Turns an imported keyword into a tag. A keyword already set (in any
 * spelling) only leaves the suggestions; at the limit nothing moves.
 */
export function acceptSuggestion(tags: string[], suggestions: string[], tag: string, max: number) {
	const rest = suggestions.filter((s) => s !== tag);
	if (tags.some((t) => normalizeTag(t) === normalizeTag(tag))) return { tags, suggestions: rest };
	if (tags.length >= max) return { tags, suggestions };
	return { tags: [...tags, tag], suggestions: rest };
}
