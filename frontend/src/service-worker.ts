/// <reference types="@sveltejs/kit" />
/// <reference no-default-lib="true"/>
/// <reference lib="esnext" />
/// <reference lib="webworker" />

// Push only: shows a cook timer's end and opens its step on a click. No
// fetch handler and no cache - the app works online, as before.
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
			const windows = await sw.clients.matchAll({ type: 'window' });
			const visible = windows.some((c) => c.visibilityState === 'visible');
			const { closeAtOnce, ...options } = notificationOptions(p, visible);
			await sw.registration.showNotification(p.title, options);
			if (closeAtOnce) {
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
			const [client] = await sw.clients.matchAll({ type: 'window' });
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
