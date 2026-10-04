/** A path item holds operations under method keys, plus keys that are not operations. */
export type SpecDocument = { paths?: Record<string, Record<string, unknown>> };

// The OpenAPI 3.1 method set. Anything else under a path item - `summary`,
// `description`, `servers`, shared `parameters` - is not an operation.
const METHODS = new Set(['get', 'put', 'post', 'delete', 'options', 'head', 'patch', 'trace']);

/**
 * Counts the operations of an OpenAPI document, the figure the API page shows.
 * Pure and separate from the fetch so the 75 KB document is thrown away at the
 * call site instead of living in component state.
 */
export function countOperations(doc: SpecDocument): number {
	let operations = 0;
	for (const item of Object.values(doc.paths ?? {})) {
		operations += Object.keys(item).filter((key) => METHODS.has(key)).length;
	}
	return operations;
}
