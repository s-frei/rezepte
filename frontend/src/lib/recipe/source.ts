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
		return readableHost(url).replace(/^www\./, '') || null;
	} catch {
		return null;
	}
}

/**
 * The link's host as a person reads it. `URL` hands a domain with umlauts
 * back as punycode (`xn--mller-kva.de`) and browsers offer no way back, so
 * the host is taken from the address as it was typed - but only when `URL`
 * agrees it is the same host, which keeps a port, credentials or anything
 * odd in the typed text out of the credit line.
 */
function readableHost(url: string): string {
	const { hostname } = new URL(url);
	const typed = /^[a-z][a-z0-9+.-]*:\/\/(?:[^@/?#]*@)?([^/?#:]+)/i.exec(url.trim())?.[1];
	try {
		if (typed && new URL(`http://${typed}`).hostname === hostname) {
			return typed.toLowerCase();
		}
	} catch {
		// Not a host on its own (an IPv6 literal cut at its first colon):
		// the parsed hostname is the readable one then.
	}
	return hostname;
}
