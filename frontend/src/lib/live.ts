const COALESCE_MS = 150;

/**
 * Follows the server-sent events stream at `url` of one shared thing - a
 * shopping list, a meal plan - and calls `onchange` whenever it changed on
 * the server, so the caller refetches it. Fires on every `changed` event,
 * the one the server sends on every (re)connection included: a change
 * between the page's own fetch and the subscription, or during a reconnect,
 * shows in its revision. A burst within 150 ms is one call.
 *
 * A `changed` event names the thing's revision; the call passes the highest of
 * its burst, so a page that shows it already (its own change, its own fetch
 * just before connecting) skips the refetch. Anything else in the burst (an
 * error) passes none.
 *
 * When the server ends the stream for good (the reader lost access: the
 * reconnect gets a 404), the source errors with `readyState === CLOSED`. No
 * reconnection follows, so `onchange` is called once then: the refetch hits the same
 * 404 and the page's failure handling shows it gone. While the browser is
 * still reconnecting (`CONNECTING`) an error is not a change.
 *
 * Returns a function that stops following.
 */
export function follow(
	url: string,
	onchange: (revision?: number) => void,
	Source = EventSource
): () => void {
	const source = new Source(url, { withCredentials: true });
	let timer: ReturnType<typeof setTimeout> | undefined;
	// The burst's highest revision; null once anything without one joined it.
	let revision: number | null | undefined;
	const notify = (rev?: number) => {
		revision = rev === undefined || revision === null ? null : Math.max(revision ?? rev, rev);
		clearTimeout(timer);
		timer = setTimeout(() => {
			const r = revision ?? undefined;
			revision = undefined;
			onchange(r);
		}, COALESCE_MS);
	};
	source.addEventListener('changed', (event) => notify(revisionOf(event)));
	source.addEventListener('error', () => {
		if (source.readyState === Source.CLOSED) notify();
	});
	return () => {
		clearTimeout(timer);
		source.close();
	};
}

function revisionOf(event: Event): number | undefined {
	try {
		const rev: unknown = JSON.parse((event as MessageEvent<string>).data).revision;
		return typeof rev === 'number' ? rev : undefined;
	} catch {
		return undefined;
	}
}
