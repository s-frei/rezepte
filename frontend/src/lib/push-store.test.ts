import { afterEach, beforeEach, describe, expect, it, vi, type Mock } from 'vitest';
import type { User } from './api/auth';
import type { TimerStart } from './recipe/timers';

// The module is `push.svelte.ts`; vitest reserves `*.svelte.test.ts` for a
// different project, as in timers-store.test.ts.
vi.mock('$app/env', () => ({ browser: true }));
vi.mock('$app/navigation', () => ({ goto: vi.fn() }));
// kit's resolve: a route id starts with '/', a plain path does not.
vi.mock('$app/paths', () => ({
	resolve: (path: string) => (path.startsWith('/') ? path : `/${path}`)
}));

const soup: TimerStart = {
	recipeId: 'r1',
	slug: 'soup',
	recipeTitle: 'Soup',
	step: 2,
	phrase: '10 min',
	seconds: 600
};

const ENDPOINT = 'https://fcm.googleapis.com/fcm/send/x';

type Env = {
	permission: NotificationPermission;
	fetch: Mock<(url: string, init?: RequestInit) => Promise<Response>>;
	subscribe: Mock<(...args: unknown[]) => Promise<unknown>>;
	requestPermission: Mock<() => Promise<NotificationPermission>>;
	closed: string[];
	/** Fires `visibilitychange` with the page visible again. */
	show: () => void;
	/** The subscription the browser already had, from `existingKey`. */
	old?: { unsubscribe: Mock<() => Promise<boolean>> } | null;
};

const SERVER_KEY = [4, 0, 0];

function okFetch() {
	return vi.fn(async (url: string, init?: RequestInit) => {
		if (url === '/api/v1/push/key') {
			return Response.json({ key: 'BAAA' });
		}
		if (url === '/api/v1/push/alarms' && init?.method === 'POST') {
			return Response.json({ id: 'alarm-1' });
		}
		return new Response(null, { status: 204 });
	});
}

function install(
	permission: NotificationPermission,
	fetchFn: Env['fetch'] = okFetch(),
	{
		supported = true,
		pathname = '/recipes',
		existingKey
	}: { supported?: boolean; pathname?: string; existingKey?: number[] | null } = {}
): Env {
	const doc = Object.assign(new EventTarget(), { visibilityState: 'visible', cookie: '' });
	vi.stubGlobal('document', doc);
	const env: Env = {
		permission,
		fetch: fetchFn,
		closed: [],
		show: () => doc.dispatchEvent(new Event('visibilitychange')),
		requestPermission: vi.fn(async () => env.permission),
		subscribe: vi.fn<(...args: unknown[]) => Promise<unknown>>(async () => subscription)
	};
	const made = (key: number[] | null) => ({
		endpoint: ENDPOINT,
		options: { applicationServerKey: key && new Uint8Array(key).buffer },
		toJSON: () => ({ endpoint: ENDPOINT, keys: { p256dh: 'p', auth: 'a' } }),
		unsubscribe: vi.fn(async () => {
			current = null;
			return true;
		})
	});
	// The server's key 'BAAA' decodes to these bytes.
	const subscription = made(SERVER_KEY);
	const old = existingKey === undefined ? null : made(existingKey);
	env.old = old;
	let current: unknown = old;
	const registration = {
		pushManager: {
			getSubscription: vi.fn(async () => current),
			subscribe: vi.fn(async (...args: unknown[]) => {
				current = await env.subscribe(...args);
				return current;
			})
		},
		getNotifications: vi.fn(async ({ tag }: { tag: string }) => [
			{ close: () => env.closed.push(tag) }
		])
	};
	const notification = {
		get permission() {
			return env.permission;
		},
		requestPermission: env.requestPermission
	};
	vi.stubGlobal('Notification', notification);
	vi.stubGlobal('window', {
		isSecureContext: true,
		...(supported && { PushManager: class {} }),
		Notification: notification,
		location: { pathname },
		localStorage: { getItem: () => null, setItem: () => {}, removeItem: () => {} }
	});
	vi.stubGlobal('navigator', {
		serviceWorker: {
			register: vi.fn(async () => registration),
			ready: Promise.resolve(registration),
			getRegistration: vi.fn(async () => registration)
		}
	});
	vi.stubGlobal('fetch', env.fetch);
	return env;
}

const alice = { id: 'alice' } as User;
const bob = { id: 'bob' } as User;

/** Imports the store as the layout does, then signs `user` in as its load does. */
async function load({ user = alice as User | null } = {}) {
	vi.resetModules();
	const push = await import('./push.svelte');
	const { timers } = await import('./recipe/timers.svelte');
	const { session } = await import('./auth.svelte');
	session.user = user;
	return { push, timers, session };
}

/** Lets every pending promise of the push path settle. */
const settle = () => new Promise((r) => setTimeout(r, 0));

function calls(env: Env, method: string, url: string) {
	return env.fetch.mock.calls.filter(
		([u, init]) => u === url && ((init as RequestInit | undefined)?.method ?? 'GET') === method
	);
}

beforeEach(() => {
	vi.setSystemTime(1_000_000);
});

afterEach(() => {
	vi.useRealTimers();
	vi.unstubAllGlobals();
});

describe('push', () => {
	it('schedules one alarm on start, with the pinned copy', async () => {
		const env = install('granted');
		const { timers } = await load();
		const t = timers.start(soup);
		await settle();
		const posts = calls(env, 'POST', '/api/v1/push/alarms');
		expect(posts).toHaveLength(1);
		expect(JSON.parse(posts[0][1]!.body as string)).toEqual({
			endpoint: ENDPOINT,
			fireAt: new Date(1_000_000 + 600_000).toISOString(),
			title: 'Soup · done',
			body: 'Step 3 · 10 min',
			url: '/recipes/soup/cook?step=3'
		});
		expect(calls(env, 'PUT', '/api/v1/push/subscription')).toHaveLength(1);
		expect(timers.byId(t.id)?.alarmId).toBe('alarm-1');
	});

	it('cancels the alarm on pause and closes its notification', async () => {
		const env = install('granted');
		const { timers } = await load();
		const t = timers.start(soup);
		await settle();
		timers.toggle(t.id);
		await settle();
		expect(calls(env, 'DELETE', '/api/v1/push/alarms/alarm-1')).toHaveLength(1);
		expect(env.closed).toEqual(['alarm-1']);
		expect(timers.byId(t.id)?.alarmId).toBeUndefined();
	});

	it('asks for permission only when undecided, inside the tap', async () => {
		const env = install('default');
		env.requestPermission.mockImplementation(async () => (env.permission = 'granted'));
		const { timers } = await load();
		timers.start(soup);
		// Synchronously, before any await: the browser wants the gesture.
		expect(env.requestPermission).toHaveBeenCalledTimes(1);
		await settle();
		expect(calls(env, 'POST', '/api/v1/push/alarms')).toHaveLength(1);
	});

	it('does not subscribe when permission is denied', async () => {
		const env = install('denied');
		const { push, timers } = await load();
		expect(() => timers.start(soup)).not.toThrow();
		await settle();
		expect(env.requestPermission).not.toHaveBeenCalled();
		expect(env.subscribe).not.toHaveBeenCalled();
		expect(env.fetch).not.toHaveBeenCalled();
		expect(push.pushUnavailable()).toBe(true);
	});

	it('keeps the timer running when the network is down', async () => {
		const env = install(
			'granted',
			vi.fn(async (): Promise<Response> => {
				throw new TypeError('offline');
			})
		);
		const { timers } = await load();
		const t = timers.start(soup);
		await settle();
		expect(env.fetch).toHaveBeenCalled();
		expect(timers.byId(t.id)?.state).toBe('running');
		timers.toggle(t.id);
		await settle();
		expect(timers.byId(t.id)?.state).toBe('paused');
	});

	it('swallows a refused endpoint', async () => {
		const env = install(
			'granted',
			vi.fn(async (url: string) =>
				url === '/api/v1/push/key'
					? Response.json({ key: 'BAAA' })
					: Response.json({ title: 'Unprocessable Entity' }, { status: 422 })
			)
		);
		const { push, timers } = await load();
		await settle();
		// The subscription on load was refused; the tap tries once more.
		expect(push.deviceState()).toBe('off');
		const t = timers.start(soup);
		await settle();
		expect(calls(env, 'PUT', '/api/v1/push/subscription')).toHaveLength(2);
		expect(calls(env, 'POST', '/api/v1/push/alarms')).toHaveLength(0);
		expect(timers.byId(t.id)?.state).toBe('running');
	});

	it('drops an alarm that arrives after a pause', async () => {
		const env = install('granted');
		const { timers } = await load();
		const t = timers.start(soup);
		timers.toggle(t.id);
		await settle();
		expect(calls(env, 'DELETE', '/api/v1/push/alarms/alarm-1')).toHaveLength(1);
		expect(timers.byId(t.id)?.alarmId).toBeUndefined();
	});

	it('disablePush cancels alarms and the subscription', async () => {
		const env = install('granted');
		const { push, timers } = await load();
		const t = timers.start(soup);
		await settle();
		await push.disablePush();
		expect(calls(env, 'DELETE', '/api/v1/push/alarms/alarm-1')).toHaveLength(1);
		const del = calls(env, 'DELETE', '/api/v1/push/subscription');
		expect(del).toHaveLength(1);
		expect(JSON.parse(del[0][1]!.body as string)).toEqual({ endpoint: ENDPOINT });
		expect(timers.byId(t.id)?.alarmId).toBeUndefined();
		// The sign-out's clear() that follows has nothing left to cancel.
		timers.clear();
		await settle();
		expect(calls(env, 'DELETE', '/api/v1/push/alarms/alarm-1')).toHaveLength(1);
	});
});

describe('device state', () => {
	it('subscribes on load when permission is granted', async () => {
		const env = install('granted');
		const { push } = await load();
		await settle();
		expect(calls(env, 'PUT', '/api/v1/push/subscription')).toHaveLength(1);
		expect(push.deviceState()).toBe('enabled');
	});

	it('stays quiet when signed out, even with permission granted', async () => {
		const env = install('granted');
		const { push } = await load({ user: null });
		await settle();
		env.show();
		await settle();
		expect(env.fetch).not.toHaveBeenCalled();
		expect(push.deviceState()).toBe('off');
	});

	it('never subscribes on a public share', async () => {
		const env = install('granted', okFetch(), { pathname: '/s/abc' });
		await load();
		await settle();
		env.show();
		await settle();
		expect(env.fetch).not.toHaveBeenCalled();
	});

	it('subscribes once when the session appears', async () => {
		const env = install('granted');
		const { push, session } = await load({ user: null });
		await settle();
		session.user = alice;
		await settle();
		// The same account saved again is no new sign-in.
		session.user = { ...alice };
		await settle();
		expect(calls(env, 'PUT', '/api/v1/push/subscription')).toHaveLength(1);
		expect(push.deviceState()).toBe('enabled');
	});

	it('subscribes again after sign-out and sign-in as someone else', async () => {
		const env = install('granted');
		const { push, session } = await load();
		await settle();
		await push.disablePush();
		session.user = null;
		expect(push.deviceState()).toBe('off');
		session.user = bob;
		await settle();
		expect(calls(env, 'PUT', '/api/v1/push/subscription')).toHaveLength(2);
		expect(push.deviceState()).toBe('enabled');
	});

	it('stays off and quiet on load while undecided', async () => {
		const env = install('default');
		const { push } = await load();
		await settle();
		expect(env.fetch).not.toHaveBeenCalled();
		expect(env.requestPermission).not.toHaveBeenCalled();
		expect(push.deviceState()).toBe('off');
	});

	it('stays off when the subscription on load fails', async () => {
		install(
			'granted',
			vi.fn(async (): Promise<Response> => {
				throw new TypeError('offline');
			})
		);
		const { push } = await load();
		await settle();
		expect(push.deviceState()).toBe('off');
	});

	it('enableDevice asks, then subscribes', async () => {
		const env = install('default');
		env.requestPermission.mockImplementation(async () => (env.permission = 'granted'));
		const { push } = await load();
		const done = push.enableDevice();
		expect(env.requestPermission).toHaveBeenCalledTimes(1);
		await done;
		expect(calls(env, 'PUT', '/api/v1/push/subscription')).toHaveLength(1);
		expect(push.deviceState()).toBe('enabled');
	});

	it('enableDevice rejects when the member declines', async () => {
		const env = install('default');
		env.requestPermission.mockImplementation(async () => (env.permission = 'denied'));
		const { push } = await load();
		await expect(push.enableDevice()).rejects.toThrow();
		expect(push.deviceState()).toBe('blocked');
	});

	it('reads a permission revoked in the browser on visibilitychange', async () => {
		const env = install('granted');
		const { push } = await load();
		await settle();
		expect(push.deviceState()).toBe('enabled');
		env.permission = 'denied';
		env.show();
		expect(push.deviceState()).toBe('blocked');
	});

	it('asks again after the permission was reset to undecided', async () => {
		const env = install('granted');
		const { push } = await load();
		await settle();
		env.permission = 'default';
		env.requestPermission.mockImplementation(async () => (env.permission = 'granted'));
		const done = push.enableDevice();
		expect(env.requestPermission).toHaveBeenCalledTimes(1);
		await done;
		expect(calls(env, 'PUT', '/api/v1/push/subscription')).toHaveLength(2);
	});

	it('is off after a revoke and subscribes afresh once allowed again', async () => {
		const env = install('granted');
		const { push } = await load();
		await settle();
		env.permission = 'default';
		env.show();
		expect(push.deviceState()).toBe('off');
		env.permission = 'granted';
		env.show();
		await settle();
		expect(calls(env, 'PUT', '/api/v1/push/subscription')).toHaveLength(2);
		expect(push.deviceState()).toBe('enabled');
	});

	it('does not subscribe again each time the page comes back', async () => {
		// A session that ran out while hidden would answer 401 and bounce the
		// cook to the login page.
		const env = install('granted');
		await load();
		await settle();
		env.show();
		await settle();
		env.show();
		await settle();
		expect(calls(env, 'PUT', '/api/v1/push/subscription')).toHaveLength(1);
	});

	it('subscribes on visibility when granted but not subscribed', async () => {
		const env = install('granted', okFetch());
		let fail = true;
		const base = env.fetch.getMockImplementation()!;
		env.fetch.mockImplementation(async (url, init) => {
			if (url === '/api/v1/push/subscription' && fail) throw new TypeError('offline');
			return base(url, init);
		});
		const { push } = await load();
		await settle();
		expect(push.deviceState()).toBe('off');
		fail = false;
		env.show();
		await settle();
		expect(calls(env, 'PUT', '/api/v1/push/subscription')).toHaveLength(2);
		expect(push.deviceState()).toBe('enabled');
	});

	it('renews a subscription the browser kept from another key', async () => {
		// The instance was set up again on this origin: a new VAPID key.
		const env = install('granted', okFetch(), { existingKey: [9, 9, 9] });
		const { push } = await load();
		await settle();
		expect(env.old!.unsubscribe).toHaveBeenCalledTimes(1);
		expect(env.subscribe).toHaveBeenCalledTimes(1);
		const [opts] = env.subscribe.mock.calls[0] as [{ applicationServerKey: Uint8Array }];
		expect([...opts.applicationServerKey]).toEqual(SERVER_KEY);
		expect(calls(env, 'PUT', '/api/v1/push/subscription')).toHaveLength(1);
		expect(push.deviceState()).toBe('enabled');
	});

	it('renews a subscription without a key', async () => {
		const env = install('granted', okFetch(), { existingKey: null });
		await load();
		await settle();
		expect(env.old!.unsubscribe).toHaveBeenCalledTimes(1);
		expect(env.subscribe).toHaveBeenCalledTimes(1);
	});

	it("reuses a subscription made with the server's key", async () => {
		const env = install('granted', okFetch(), { existingKey: SERVER_KEY });
		await load();
		await settle();
		expect(env.old!.unsubscribe).not.toHaveBeenCalled();
		expect(env.subscribe).not.toHaveBeenCalled();
		expect(calls(env, 'PUT', '/api/v1/push/subscription')).toHaveLength(1);
	});

	it('sendPushTest turns the device off when the server forgot it', async () => {
		install(
			'granted',
			vi.fn(async (url: string) => {
				if (url === '/api/v1/push/key') {
					return Response.json({ key: 'BAAA' });
				}
				if (url === '/api/v1/push/test') {
					return Response.json({ title: 'Not Found' }, { status: 404 });
				}
				return new Response(null, { status: 204 });
			})
		);
		const { push } = await load();
		await settle();
		expect(push.deviceState()).toBe('enabled');
		await expect(push.sendPushTest()).rejects.toThrow();
		expect(push.deviceState()).toBe('off');
	});

	it('sendPushTest posts the endpoint', async () => {
		const env = install('granted');
		const { push } = await load();
		await settle();
		await push.sendPushTest();
		const posts = calls(env, 'POST', '/api/v1/push/test');
		expect(posts).toHaveLength(1);
		expect(JSON.parse(posts[0][1]!.body as string)).toEqual({ endpoint: ENDPOINT });
	});

	it('sendPushTest rejects on a throttled test', async () => {
		install(
			'granted',
			vi.fn(async (url: string) => {
				if (url === '/api/v1/push/key') {
					return Response.json({ key: 'BAAA' });
				}
				if (url === '/api/v1/push/test') {
					return Response.json({ title: 'Too Many Requests' }, { status: 429 });
				}
				return new Response(null, { status: 204 });
			})
		);
		const { push } = await load();
		await settle();
		await expect(push.sendPushTest()).rejects.toThrow();
	});

	it('is unsupported without a push manager', async () => {
		const env = install('default', okFetch(), { supported: false });
		const { push } = await load();
		await settle();
		expect(push.deviceState()).toBe('unsupported');
		expect(env.fetch).not.toHaveBeenCalled();
	});
});
