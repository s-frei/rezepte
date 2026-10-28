import { browser } from '$app/environment';
import {
	remainingMs,
	parseTimers,
	settle,
	timerId,
	TIMERS_STORAGE_KEY,
	type Timer,
	type TimerStart
} from './timers';

export type TimerEvent = 'start' | 'pause' | 'resume' | 'stop';

function readStored(): Timer[] {
	if (!browser) {
		return [];
	}
	try {
		return parseTimers(window.localStorage.getItem(TIMERS_STORAGE_KEY));
	} catch {
		// `window.localStorage` itself throws when site data is blocked.
		return [];
	}
}

function writeStored(list: Timer[]): void {
	if (!browser) {
		return;
	}
	try {
		if (list.length === 0) {
			window.localStorage.removeItem(TIMERS_STORAGE_KEY);
		} else {
			window.localStorage.setItem(TIMERS_STORAGE_KEY, JSON.stringify(list));
		}
	} catch {
		// Storage blocked or full: the timers still run for this page view.
	}
}

/**
 * The cook timers of this browser. They outlive steps, pages and reloads:
 * the list is written to `localStorage` on every change, and a timer whose
 * `endsAt` passed while the page was closed comes back ringing.
 */
export class Timers {
	list = $state<Timer[]>([]);
	now = $state(Date.now());

	private interval: ReturnType<typeof setInterval> | null = null;
	private listeners: ((event: TimerEvent, t: Timer) => void)[] = [];

	constructor() {
		this.list = settle(readStored(), Date.now());
		this.sync();
		if (browser && typeof document !== 'undefined') {
			document.addEventListener('visibilitychange', () => {
				if (document.visibilityState === 'visible') {
					this.tick();
				}
			});
		}
	}

	get ringing(): Timer[] {
		return this.list.filter((t) => t.state === 'ringing');
	}

	byId(id: string): Timer | undefined {
		return this.list.find((t) => t.id === id);
	}

	start(input: TimerStart): Timer {
		const id = timerId(input.recipeId, input.step, input.phrase);
		const existing = this.byId(id);
		if (existing) {
			return existing;
		}
		const now = Date.now();
		const t: Timer = {
			...input,
			id,
			state: 'running',
			endsAt: now + input.seconds * 1000,
			remainingMs: null
		};
		this.now = now;
		this.commit([...this.list, t]);
		this.emit('start', t);
		return t;
	}

	toggle(id: string): void {
		const t = this.byId(id);
		if (!t || t.state === 'ringing') {
			return;
		}
		const now = Date.now();
		this.now = now;
		const next: Timer =
			t.state === 'running'
				? { ...t, state: 'paused', endsAt: null, remainingMs: remainingMs(t, now) }
				: {
						...t,
						state: 'running',
						endsAt: now + (t.remainingMs ?? 0),
						remainingMs: null
					};
		this.commit(this.list.map((x) => (x.id === id ? next : x)));
		this.emit(next.state === 'paused' ? 'pause' : 'resume', next);
	}

	stop(id: string): void {
		const t = this.byId(id);
		if (!t) {
			return;
		}
		this.commit(this.list.filter((x) => x.id !== id));
		this.emit('stop', t);
	}

	/** Removes every timer (sign-out). */
	clear(): void {
		const gone = this.list;
		this.commit([]);
		for (const t of gone) {
			this.emit('stop', t);
		}
	}

	listen(fn: (event: TimerEvent, t: Timer) => void): () => void {
		this.listeners.push(fn);
		return () => {
			this.listeners = this.listeners.filter((l) => l !== fn);
		};
	}

	setAlarm(id: string, alarmId: string | undefined): void {
		this.commit(this.list.map((t) => (t.id === id ? { ...t, alarmId } : t)));
	}

	private emit(event: TimerEvent, t: Timer): void {
		for (const fn of this.listeners) {
			fn(event, t);
		}
	}

	private commit(list: Timer[]): void {
		this.list = list;
		writeStored(list);
		this.sync();
	}

	private tick(): void {
		const now = Date.now();
		this.now = now;
		const settled = settle(this.list, now);
		if (settled.some((t, i) => t !== this.list[i])) {
			this.commit(settled);
		}
	}

	/** One interval, alive only while a timer is running. */
	private sync(): void {
		const running = this.list.some((t) => t.state === 'running');
		if (running && this.interval === null) {
			this.interval = setInterval(() => this.tick(), 1000);
		} else if (!running && this.interval !== null) {
			clearInterval(this.interval);
			this.interval = null;
		}
	}
}

export const timers = new Timers();
