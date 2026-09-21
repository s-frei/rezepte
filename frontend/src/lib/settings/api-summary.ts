/** The figures the API card shows, read off the document the instance serves. */
export type ApiSummary = { operations: number; areas: number };

/** A path item holds operations under method keys, plus keys that are not operations. */
export type SpecDocument = { paths?: Record<string, Record<string, unknown>> };

// The OpenAPI 3.1 method set. Anything else under a path item - `summary`,
// `description`, `servers`, shared `parameters` - is not an operation.
const METHODS = new Set(['get', 'put', 'post', 'delete', 'options', 'head', 'patch', 'trace']);

/** The tags of one operation, tolerating a document that does not look like one. */
function tagsOf(operation: unknown): string[] {
	if (typeof operation !== 'object' || operation === null) {
		return [];
	}
	const tags = (operation as { tags?: unknown }).tags;
	return Array.isArray(tags) ? tags.filter((tag): tag is string => typeof tag === 'string') : [];
}

/**
 * Reduces an OpenAPI document to the two numbers the card shows. Pure and
 * separate from the fetch so the 75 KB document is thrown away at the call
 * site instead of living in component state.
 */
export function summariseSpec(doc: SpecDocument): ApiSummary {
	const areas = new Set<string>();
	let operations = 0;
	for (const item of Object.values(doc.paths ?? {})) {
		for (const [key, operation] of Object.entries(item)) {
			if (!METHODS.has(key)) {
				continue;
			}
			operations++;
			for (const tag of tagsOf(operation)) {
				areas.add(tag);
			}
		}
	}
	return { operations, areas: areas.size };
}
