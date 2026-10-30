import { browser } from '$app/env';
import { ApiError } from '#lib/api/client.js';
import {
	deleteAlarm,
	deleteSubscription,
	getPushKey,
	postAlarm,
	postPushTest,
	putSubscription
} from '#lib/api/push.js';
import { onSignIn, session } from '#lib/auth.svelte.js';
import { m } from '#lib/paraglide/messages.js';
import { cookPath, type Timer } from '#lib/recipe/timers.js';
import { timers } from '#lib/recipe/timers.svelte.js';

/**
 * Web Push for cook timers and notifications: a running timer has an alarm on
 * the server that pushes to this device when it ends, so the cook hears of it
 * with the page hidden or the phone locked, and a subscribed device gets the
 * member's notifications. Everything here is best-effort - a failure leaves
 * the in-page timer exactly as it was.
 */
export function pushSupported(): boolean {
	return (
		browser &&
		window.isSecureContext &&
		'serviceWorker' in navigator &&
		'PushManager' in window &&
		'Notification' in window
	);
}

const state = $state({
	permission: pushSupported() ? Notification.permission : 'denied',
	/** A subscription was registered with the server this page view. */
	subscribed: false
});

/** What this device does with pushes, for the settings' device box. */
export type DeviceState = 'enabled' | 'off' | 'blocked' | 'unsupported';

export function deviceState(): DeviceState {
	if (!pushSupported()) {
		return 'unsupported';
	}
	if (state.permission === 'denied') {
		return 'blocked';
	}
	return state.permission === 'granted' && state.subscribed ? 'enabled' : 'off';
}

/**
 * Never asked on this device, so a tap can still allow it. Not `'off'`:
 * that also covers a granted device between load and its resubscribe.
 */
export function pushUndecided(): boolean {
	return pushSupported() && state.permission === 'default';
}

/** True when a timer can only tell the cook while the page is open. */
export function pushUnavailable(): boolean {
	return state.permission === 'denied';
}

let registration: Promise<ServiceWorkerRegistration> | null = null;
let endpoint: Promise<string> | null = null;

function ready(): Promise<ServiceWorkerRegistration> {
	registration ??= navigator.serviceWorker
		.register('/service-worker.js', { type: 'module' })
		.then(() => navigator.serviceWorker.ready);
	return registration;
}

function decodeKey(key: string): Uint8Array<ArrayBuffer> {
	const raw = atob(key.replace(/-/g, '+').replace(/_/g, '/'));
	return Uint8Array.from(raw, (c) => c.charCodeAt(0));
}

function sameKey(a: ArrayBuffer | null | undefined, b: Uint8Array): boolean {
	if (!a || a.byteLength !== b.length) {
		return false;
	}
	const bytes = new Uint8Array(a);
	return b.every((v, i) => v === bytes[i]);
}

async function subscribe(): Promise<string> {
	const reg = await ready();
	const key = decodeKey(await getPushKey());
	let sub = await reg.pushManager.getSubscription();
	// The browser keeps a subscription across an instance set up again on
	// this origin, whose new VAPID key the push service would refuse it for.
	if (sub && !sameKey(sub.options.applicationServerKey, key)) {
		await sub.unsubscribe();
		sub = null;
	}
	sub ??= await reg.pushManager.subscribe({ userVisibleOnly: true, applicationServerKey: key });
	const { keys } = sub.toJSON();
	await putSubscription({
		endpoint: sub.endpoint,
		p256dh: keys?.p256dh ?? '',
		auth: keys?.auth ?? ''
	});
	state.subscribed = true;
	return sub.endpoint;
}

/**
 * The device's endpoint, once the cook allowed notifications. Must be called
 * from the tap: the permission prompt is only allowed inside a user gesture.
 */
function ensureEndpoint(): Promise<string> | null {
	if (state.permission === 'denied') {
		return null;
	}
	if (endpoint && Notification.permission === 'granted') {
		return endpoint;
	}
	// Reset or revoked in the browser since: ask again on this tap.
	endpoint = null;
	const asked =
		Notification.permission === 'default'
			? Notification.requestPermission()
			: Promise.resolve(Notification.permission);
	return (endpoint = asked.then((p) => {
		state.permission = p;
		if (p !== 'granted') {
			throw new Error('notifications not allowed');
		}
		return subscribe();
	})).catch((e: unknown) => {
		// Next tap tries again (a 422 for an unknown push host stays a 422).
		endpoint = null;
		state.subscribed = false;
		throw e;
	});
}

async function arm(t: Timer, pending: Promise<string>): Promise<void> {
	if (t.endsAt === null) {
		return;
	}
	const id = await postAlarm({
		endpoint: await pending,
		fireAt: new Date(t.endsAt).toISOString(),
		title: m.timer_notify_title({ recipe: t.recipeTitle }),
		body: m.timer_name({ number: t.step + 1, phrase: t.phrase }),
		url: `/${cookPath(t)}`
	});
	const now = timers.byId(t.id);
	if (now?.state === 'running' && now.endsAt === t.endsAt) {
		timers.setAlarm(t.id, id);
	} else {
		// Paused or stopped while the request ran.
		await disarm(id);
	}
}

async function disarm(alarmId: string): Promise<void> {
	await deleteAlarm(alarmId);
	const reg = await registration;
	for (const n of (await reg?.getNotifications({ tag: alarmId })) ?? []) {
		n.close();
	}
}

/** Turns push on for this device. Must be called from the click, like a timer's start. */
export async function enableDevice(): Promise<void> {
	await (ensureEndpoint() ?? Promise.reject(new Error('notifications blocked')));
}

/** Asks the server to push a test here; a 404, a 429 or the network throws. */
export async function sendPushTest(): Promise<void> {
	if (!endpoint) {
		throw new Error('push not enabled on this device');
	}
	try {
		await postPushTest(await endpoint);
	} catch (e) {
		// The server has no such subscription (any more): the device is off.
		if (e instanceof ApiError && e.status === 404) {
			endpoint = null;
			state.subscribed = false;
		}
		throw e;
	}
}

const ignore = () => {};

if (pushSupported()) {
	ready().catch(ignore);
	// Allowed before: subscribe afresh, the PUT is idempotent and moves the
	// endpoint to whoever is signed in now. Only with a session and off a
	// public share: signed out, the push routes answer 401 and `api()` would
	// send the visitor to the login page.
	const resubscribe = () => {
		if (
			Notification.permission === 'granted' &&
			session.user &&
			!window.location.pathname.startsWith('/s/')
		) {
			endpoint = null;
			ensureEndpoint()?.catch(ignore);
		}
	};
	onSignIn(resubscribe);
	// Notifications can be revoked and allowed again in the browser or OS
	// settings at any time; a revoke may drop the browser's subscription.
	// Only then, or while not subscribed, is the PUT sent again: on every
	// return a session that ran out while hidden would answer 401 and bounce
	// the cook mid-recipe to the login page.
	document.addEventListener('visibilitychange', () => {
		if (document.visibilityState === 'visible') {
			const before = state.permission;
			state.permission = Notification.permission;
			if (state.permission !== 'granted') {
				state.subscribed = false;
			} else if (before !== 'granted' || !state.subscribed) {
				resubscribe();
			}
		}
	});
	timers.listen((event, t) => {
		if (event === 'start' || event === 'resume') {
			const pending = ensureEndpoint();
			if (pending) {
				arm(t, pending).catch(ignore);
			}
		} else if (t.alarmId) {
			const alarmId = t.alarmId;
			if (event === 'pause') {
				timers.setAlarm(t.id, undefined);
			}
			disarm(alarmId).catch(ignore);
		}
	});
}

/** Sign-out: this device stops getting pushes for whoever was signed in. */
export async function disablePush(): Promise<void> {
	if (!pushSupported()) {
		return;
	}
	const armed = timers.list.filter((t) => t.alarmId);
	for (const t of armed) {
		timers.setAlarm(t.id, undefined);
	}
	await Promise.allSettled(armed.map((t) => disarm(t.alarmId!)));
	try {
		const reg = await navigator.serviceWorker.getRegistration();
		const sub = await reg?.pushManager.getSubscription();
		if (sub) {
			await deleteSubscription(sub.endpoint).catch(ignore);
			await sub.unsubscribe();
		}
	} catch {
		// see above
	}
	endpoint = null;
	state.subscribed = false;
}
