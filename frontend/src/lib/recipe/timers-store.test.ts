import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { TIMERS_STORAGE_KEY, timerId, type Timer, type TimerStart } from './timers';

vi.mock('$app/environment', () => ({ browser: true }));

// The module is `timers.svelte.ts`; the test file cannot carry the `.svelte`
// infix, because vitest reserves `*.svelte.test.ts` for a different project.
const { Timers } = await import('./timers.svelte');

const soup: TimerStart = {
	recipeId: 'r1',
	slug: 'soup',
	recipeTitle: 'Soup',
	step: 2,
	phrase: '10 min',
	seconds: 600
};

function memoryStorage(initial: Record<string, string> = {}) {
	const data = new Map(Object.entries(initial));
	return {
		getItem: (k: string) => data.get(k) ?? null,
		setItem: (k: string, v: string) => void data.set(k, v),
		removeItem: (k: string) => void data.delete(k)
	};
}

let storage: ReturnType<typeof memoryStorage>;
let onStorage: (e: { key: string | null; newValue: string | null }) => void;

function install(s: unknown) {
	vi.stubGlobal('window', {
		addEventListener: (type: string, fn: typeof onStorage) => {
			if (type === 'storage') {
				onStorage = fn;
			}
		},
		get localStorage() {
			if (s === 'blocked') {
				throw new Error('blocked');
			}
			return s;
		}
	});
	vi.stubGlobal('document', { addEventListener: vi.fn(), visibilityState: 'visible' });
}

beforeEach(() => {
	vi.useFakeTimers();
	vi.setSystemTime(1_000_000);
	storage = memoryStorage();
	install(storage);
});

afterEach(() => {
	vi.useRealTimers();
	vi.unstubAllGlobals();
});

describe('Timers', () => {
	it('follows another tab through the storage event, without writing back', () => {
		const timers = new Timers();
		timers.start(soup);
		const other: Timer = {
			...soup,
			phrase: '5 min',
			id: timerId('r1', 2, '5 min'),
			state: 'paused',
			endsAt: null,
			remainingMs: 300_000
		};
		const json = JSON.stringify([other]);
		onStorage({ key: TIMERS_STORAGE_KEY, newValue: json });
		expect(timers.list).toEqual([other]);
		expect(vi.getTimerCount()).toBe(0);
		expect(storage.getItem(TIMERS_STORAGE_KEY)).not.toBe(json);
		timers.start(soup);
		onStorage({ key: null, newValue: null });
		expect(timers.list).toEqual([]);
		onStorage({ key: 'other', newValue: json });
		expect(timers.list).toEqual([]);
	});

	it('start is idempotent per time', () => {
		const timers = new Timers();
		const fn = vi.fn();
		timers.listen(fn);
		const a = timers.start(soup);
		const b = timers.start(soup);
		expect(b).toEqual(a);
		expect(timers.list).toHaveLength(1);
		expect(fn).toHaveBeenCalledTimes(1);
		expect(fn).toHaveBeenCalledWith('start', a);
		timers.start({ ...soup, phrase: '5 min', seconds: 300 });
		expect(timers.list).toHaveLength(2);
	});

	it('start returns a paused or ringing timer unchanged', () => {
		const timers = new Timers();
		const t = timers.start(soup);
		timers.toggle(t.id);
		expect(timers.start(soup).state).toBe('paused');
		timers.toggle(t.id);
		vi.advanceTimersByTime(601_000);
		expect(timers.start(soup).state).toBe('ringing');
		expect(timers.list).toHaveLength(1);
	});

	it('toggle pauses with the remaining time frozen and resumes with a new endsAt', () => {
		const timers = new Timers();
		const fn = vi.fn();
		timers.listen(fn);
		const { id } = timers.start(soup);
		vi.advanceTimersByTime(100_000);
		timers.toggle(id);
		expect(timers.byId(id)).toMatchObject({ state: 'paused', endsAt: null, remainingMs: 500_000 });
		vi.advanceTimersByTime(60_000);
		expect(timers.byId(id)?.remainingMs).toBe(500_000);
		timers.toggle(id);
		expect(timers.byId(id)).toMatchObject({
			state: 'running',
			endsAt: 1_000_000 + 160_000 + 500_000,
			remainingMs: null
		});
		expect(fn.mock.calls.map((c) => c[0])).toEqual(['start', 'pause', 'resume']);
	});

	it('toggle ignores ringing and unknown timers', () => {
		const timers = new Timers();
		const { id } = timers.start(soup);
		vi.advanceTimersByTime(601_000);
		timers.toggle(id);
		timers.toggle('nope');
		expect(timers.byId(id)?.state).toBe('ringing');
	});

	it('rings when time is up', () => {
		const timers = new Timers();
		timers.start(soup);
		vi.advanceTimersByTime(599_000);
		expect(timers.ringing).toHaveLength(0);
		vi.advanceTimersByTime(2_000);
		expect(timers.list[0].state).toBe('ringing');
		expect(timers.ringing).toHaveLength(1);
	});

	it('stop removes the timer and fires stop', () => {
		const timers = new Timers();
		const fn = vi.fn();
		const { id } = timers.start(soup);
		timers.listen(fn);
		timers.stop(id);
		timers.stop(id);
		expect(timers.list).toEqual([]);
		expect(fn).toHaveBeenCalledTimes(1);
	});

	it('listen returns an unsubscribe', () => {
		const timers = new Timers();
		const fn = vi.fn();
		timers.listen(fn)();
		timers.start(soup);
		expect(fn).not.toHaveBeenCalled();
	});

	it('setAlarm stores the alarm id', () => {
		const timers = new Timers();
		const { id } = timers.start(soup);
		timers.setAlarm(id, 'a1');
		expect(timers.byId(id)?.alarmId).toBe('a1');
		timers.setAlarm(id, undefined);
		expect(timers.byId(id)?.alarmId).toBeUndefined();
	});

	it('persists and restores', () => {
		const first = new Timers();
		first.start(soup);
		first.start({ ...soup, phrase: '5 min', seconds: 300 });
		const second = new Timers();
		expect(second.list).toEqual(first.list);
		expect(second.byId(timerId('r1', 2, '10 min'))).toBeDefined();
	});

	it('settles elapsed timers on load', () => {
		const stored: Timer = {
			...soup,
			id: timerId(soup.recipeId, soup.step, soup.phrase),
			state: 'running',
			endsAt: 900_000,
			remainingMs: null
		};
		storage.setItem(TIMERS_STORAGE_KEY, JSON.stringify([stored]));
		const timers = new Timers();
		expect(timers.list[0].state).toBe('ringing');
		expect(timers.ringing).toHaveLength(1);
	});

	it('works with blocked storage', () => {
		install('blocked');
		const timers = new Timers();
		const { id } = timers.start(soup);
		expect(timers.list).toHaveLength(1);
		timers.stop(id);
		expect(timers.list).toEqual([]);
	});

	it('clear empties storage and fires stop per timer', () => {
		const timers = new Timers();
		timers.start(soup);
		timers.start({ ...soup, phrase: '5 min', seconds: 300 });
		const fn = vi.fn();
		timers.listen(fn);
		timers.clear();
		expect(timers.list).toEqual([]);
		expect(fn.mock.calls.map((c) => c[0])).toEqual(['stop', 'stop']);
		expect(new Timers().list).toEqual([]);
	});

	it('ticks only while a timer runs', () => {
		const timers = new Timers();
		expect(vi.getTimerCount()).toBe(0);
		const { id } = timers.start(soup);
		expect(vi.getTimerCount()).toBe(1);
		timers.toggle(id);
		expect(vi.getTimerCount()).toBe(0);
		timers.toggle(id);
		vi.advanceTimersByTime(601_000);
		expect(vi.getTimerCount()).toBe(0);
		timers.stop(id);
		expect(vi.getTimerCount()).toBe(0);
	});

	it('re-settles when the tab becomes visible', () => {
		const timers = new Timers();
		timers.start(soup);
		const handler = vi
			.mocked(document.addEventListener)
			.mock.calls.find((c) => c[0] === 'visibilitychange')?.[1] as () => void;
		vi.setSystemTime(2_000_000);
		handler();
		expect(timers.list[0].state).toBe('ringing');
	});
});
