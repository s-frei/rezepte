// The service worker's pure parts, tested without a worker.

/** What the Go service pushes for a timer alarm; `tag` is the alarm's id. */
export type PushPayload = { title: string; body: string; url: string; tag: string };

export function parsePayload(data: unknown): PushPayload | null {
	if (typeof data !== 'object' || data === null) {
		return null;
	}
	const { title, body, url, tag } = data as Record<string, unknown>;
	if (
		typeof title !== 'string' ||
		typeof body !== 'string' ||
		typeof url !== 'string' ||
		typeof tag !== 'string'
	) {
		return null;
	}
	// A click opens this; it stays inside the app.
	if (!url.startsWith('/') || url.startsWith('//')) {
		return null;
	}
	return { title, body, url, tag };
}

/**
 * A visible window already rings and shows the banner, so the notification
 * there is silent and closed at once: Chromium wants one shown per push.
 */
export function notificationOptions(
	p: PushPayload,
	anyClientVisible: boolean
): NotificationOptions & { closeAtOnce: boolean } {
	return {
		body: p.body,
		tag: p.tag,
		data: { url: p.url },
		icon: '/icon-192.png',
		silent: anyClientVisible,
		closeAtOnce: anyClientVisible
	};
}
