/**
 * What a recipe's credit line names: the source's own name when it has one,
 * otherwise the link's host, so a bare link still says where it leads.
 * `null` when there is nothing to credit - the line is then left out, so
 * "Adapted from" never stands alone.
 */
export function sourceLabel(name: string | null, url: string | null): string | null {
	const trimmed = name?.trim();
	if (trimmed) {
		return trimmed;
	}
	if (!url) {
		return null;
	}
	try {
		return new URL(url).hostname.replace(/^www\./, '') || null;
	} catch {
		return null;
	}
}
