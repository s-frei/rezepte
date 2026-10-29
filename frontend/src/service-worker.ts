/// <reference types="@sveltejs/kit" />
/// <reference no-default-lib="true"/>
/// <reference lib="esnext" />
/// <reference lib="webworker" />

// Push only: shows a cook timer's end or a notification and opens its page
// on a click. No fetch handler and no cache - the app works online, as before.
import { notificationOptions, parsePayload } from '$lib/push-worker';

const sw = self as unknown as ServiceWorkerGlobalScope;

// A new version takes over at once; there is no cache to keep consistent.
sw.addEventListener('install', () => void sw.skipWaiting());
// Controlled pages are the ones `WindowClient.navigate` may move.
sw.addEventListener('activate', (event) => event.waitUntil(sw.clients.claim()));

sw.addEventListener('push', (event) => {
	let data: unknown = null;
	try {
		data = event.data?.json();
	} catch {
		// not JSON: not ours
	}
	const p = parsePayload(data);
	if (!p) {
		return;
	}
	event.waitUntil(
		(async () => {
			const windows = await sw.clients.matchAll({ type: 'window', includeUncontrolled: true });
			const shown = await sw.registration.getNotifications({ tag: p.tag });
			const shownCount = shown.reduce((n, s) => n + (Number(s.data?.count) || 1), 0);
			const o = notificationOptions(p, {
				anyClientVisible: windows.some((c) => c.visibilityState === 'visible'),
				shownCount
			});
			// The extra fields are ignored by the browser. A timer in a visible
			// window comes with `closeAtOnce`: Chromium still wants one shown per push.
			await sw.registration.showNotification(o.title, o);
			if (o.closeAtOnce) {
				for (const n of await sw.registration.getNotifications({ tag: p.tag })) {
					n.close();
				}
			}
		})()
	);
});

sw.addEventListener('notificationclick', (event) => {
	event.notification.close();
	const url: unknown = event.notification.data?.url;
	if (typeof url !== 'string') {
		return;
	}
	event.waitUntil(
		(async () => {
			const [client] = await sw.clients.matchAll({ type: 'window', includeUncontrolled: true });
			if (client) {
				try {
					await client.focus();
					await client.navigate(url);
					return;
				} catch {
					// not controlled yet: a new window below
				}
			}
			await sw.clients.openWindow(url);
		})()
	);
});
