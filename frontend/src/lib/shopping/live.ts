import { listEventsUrl } from '#lib/api/shopping.js';

const COALESCE_MS = 150;

/**
 * Calls `onchange` whenever the list changed on the server, so the caller
 * refetches it. Fires on every `changed` event and on every `open` after the
 * first (a reconnect may have missed events); a burst within 150 ms is one call.
 *
 * When the server ends the stream for good (the reader lost access: the
 * reconnect gets a 404), the source errors with `readyState === CLOSED`. No
 * `open` follows, so `onchange` is called once then: the refetch hits the same
 * 404 and the page's failure handling shows "list gone". While the browser is
 * still reconnecting (`CONNECTING`) an error is not a change.
 *
 * Returns a function that stops following.
 */
export function followList(id: string, onchange: () => void, Source = EventSource): () => void {
	const source = new Source(listEventsUrl(id), { withCredentials: true });
	let timer: ReturnType<typeof setTimeout> | undefined;
	let opened = false;
	const notify = () => {
		clearTimeout(timer);
		timer = setTimeout(onchange, COALESCE_MS);
	};
	source.addEventListener('changed', notify);
	source.addEventListener('open', () => {
		if (opened) notify();
		opened = true;
	});
	source.addEventListener('error', () => {
		if (source.readyState === Source.CLOSED) notify();
	});
	return () => {
		clearTimeout(timer);
		source.close();
	};
}
