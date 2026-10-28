import { afterEach, beforeEach, describe, expect, it, vi, type Mock } from 'vitest';
import type { TimerStart } from './recipe/timers';

// The module is `push.svelte.ts`; vitest reserves `*.svelte.test.ts` for a
// different project, as in timers-store.test.ts.
vi.mock('$app/environment', () => ({ browser: true, dev: false }));
vi.mock('$app/navigation', () => ({ goto: vi.fn() }));
vi.mock('$app/paths', () => ({ resolve: (path: string) => path }));

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
};

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

function install(permission: NotificationPermission, fetchFn: Env['fetch'] = okFetch()): Env {
	const env: Env = {
		permission,
		fetch: fetchFn,
		closed: [],
		requestPermission: vi.fn(async () => env.permission),
		subscribe: vi.fn<(...args: unknown[]) => Promise<unknown>>(async () => subscription)
	};
	let current: unknown = null;
	const subscription = {
		endpoint: ENDPOINT,
		toJSON: () => ({ endpoint: ENDPOINT, keys: { p256dh: 'p', auth: 'a' } }),
		unsubscribe: vi.fn(async () => true)
	};
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
		PushManager: class {},
		Notification: notification,
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

async function load() {
	vi.resetModules();
	const push = await import('./push.svelte');
	const { timers } = await import('./recipe/timers.svelte');
	return { push, timers };
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
		const { timers } = await load();
		const t = timers.start(soup);
		await settle();
		expect(calls(env, 'PUT', '/api/v1/push/subscription')).toHaveLength(1);
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
