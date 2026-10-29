export type TimerState = 'running' | 'paused' | 'ringing';

export type Timer = {
	id: string;
	recipeId: string;
	slug: string;
	recipeTitle: string;
	/** 0-based step index. */
	step: number;
	phrase: string;
	seconds: number;
	state: TimerState;
	/** Epoch ms while running or ringing. */
	endsAt: number | null;
	/** Time left while paused. */
	remainingMs: number | null;
	/** The server-side push alarm, when one was scheduled. */
	alarmId?: string;
};

export type TimerStart = Pick<
	Timer,
	'recipeId' | 'slug' | 'recipeTitle' | 'step' | 'phrase' | 'seconds'
>;

export const TIMERS_STORAGE_KEY = 'rezepte-timers';

/** One timer per time in a step: the same phrase twice in a step is one timer. */
export function timerId(recipeId: string, step: number, phrase: string): string {
	return `${recipeId}:${step}:${phrase}`;
}

export function remainingMs(t: Timer, now: number): number {
	if (t.state === 'paused') {
		return Math.max(0, t.remainingMs ?? 0);
	}
	if (t.state === 'ringing') {
		return 0;
	}
	return Math.max(0, (t.endsAt ?? now) - now);
}

/** The elapsed share of the timer, 0..1, for the progress ring. */
export function fraction(t: Timer, now: number): number {
	const total = t.seconds * 1000;
	if (total <= 0) {
		return 1;
	}
	return Math.min(1, Math.max(0, 1 - remainingMs(t, now) / total));
}

/** Running timers whose time is up start ringing; everything else is untouched. */
export function settle(list: Timer[], now: number): Timer[] {
	return list.map((t) =>
		t.state === 'running' && (t.endsAt ?? 0) <= now ? { ...t, state: 'ringing' as const } : t
	);
}

/** Least time left first, paused or not; a ringing timer has none left. */
export function byEnding(list: Timer[], now: number): Timer[] {
	return [...list].sort((a, b) => remainingMs(a, now) - remainingMs(b, now));
}

/** Cook mode at the timer's step (`?step` is 1-based). */
export function cookPath(
	t: Pick<Timer, 'slug' | 'step'>
): `/recipes/${string}/cook?step=${number}` {
	return `/recipes/${t.slug}/cook?step=${t.step + 1}`;
}

function isTimer(v: unknown): v is Timer {
	if (typeof v !== 'object' || v === null) {
		return false;
	}
	const t = v as Record<string, unknown>;
	const num = (x: unknown) => typeof x === 'number' && Number.isFinite(x);
	return (
		typeof t.id === 'string' &&
		typeof t.recipeId === 'string' &&
		typeof t.slug === 'string' &&
		typeof t.recipeTitle === 'string' &&
		num(t.step) &&
		typeof t.phrase === 'string' &&
		num(t.seconds) &&
		(t.state === 'running' || t.state === 'paused' || t.state === 'ringing') &&
		(t.endsAt === null || num(t.endsAt)) &&
		(t.remainingMs === null || num(t.remainingMs)) &&
		(t.alarmId === undefined || typeof t.alarmId === 'string')
	);
}

export function parseTimers(raw: string | null): Timer[] {
	if (raw === null) {
		return [];
	}
	try {
		const parsed: unknown = JSON.parse(raw);
		return Array.isArray(parsed) ? parsed.filter(isTimer) : [];
	} catch {
		return [];
	}
}

/** "1:03:12", "14:20", "0:05": rounded up to whole seconds. */
export function formatClock(ms: number): string {
	const total = Math.max(0, Math.ceil(ms / 1000));
	const h = Math.floor(total / 3600);
	const m = Math.floor((total % 3600) / 60);
	const s = total % 60;
	const ss = String(s).padStart(2, '0');
	return h > 0 ? `${h}:${String(m).padStart(2, '0')}:${ss}` : `${m}:${ss}`;
}

export function clampStep(step: number, count: number): number {
	return Math.max(0, Math.min(step, count - 1));
}
