import { describe, expect, it } from 'vitest';
import {
	byEnding,
	clampStep,
	cookPath,
	formatClock,
	fraction,
	parseTimers,
	remainingMs,
	settle,
	timerId,
	type Timer
} from './timers';

function timer(over: Partial<Timer> = {}): Timer {
	return {
		id: 'r1:0:10 min',
		recipeId: 'r1',
		slug: 'soup',
		recipeTitle: 'Soup',
		step: 0,
		phrase: '10 min',
		seconds: 600,
		state: 'running',
		endsAt: 1_000_000,
		remainingMs: null,
		...over
	};
}

describe('timerId', () => {
	it('joins recipe, step and phrase', () => {
		expect(timerId('r1', 4, '90 Minuten')).toBe('r1:4:90 Minuten');
	});
});

describe('remainingMs', () => {
	it('counts down to endsAt', () => {
		expect(remainingMs(timer(), 940_000)).toBe(60_000);
	});
	it('is 0 for a running timer past endsAt', () => {
		expect(remainingMs(timer(), 2_000_000)).toBe(0);
	});
	it('is frozen while paused', () => {
		expect(
			remainingMs(timer({ state: 'paused', endsAt: null, remainingMs: 42_000 }), 9_999_999)
		).toBe(42_000);
	});
	it('is 0 while ringing', () => {
		expect(remainingMs(timer({ state: 'ringing' }), 0)).toBe(0);
	});
});

describe('fraction', () => {
	it('is the elapsed share', () => {
		expect(fraction(timer({ seconds: 100, endsAt: 1_000_000 }), 950_000)).toBe(0.5);
	});
	it('stays within 0..1', () => {
		expect(fraction(timer(), 5_000_000)).toBe(1);
		expect(fraction(timer({ seconds: 100 }), 0)).toBe(0);
	});
	it('is 1 while ringing and measures a paused timer by what is left', () => {
		expect(fraction(timer({ state: 'ringing' }), 0)).toBe(1);
		expect(
			fraction(timer({ seconds: 100, state: 'paused', endsAt: null, remainingMs: 25_000 }), 0)
		).toBe(0.75);
	});
});

describe('settle', () => {
	it('turns an elapsed running timer into ringing', () => {
		const [t] = settle([timer()], 1_000_000);
		expect(t.state).toBe('ringing');
	});
	it('leaves running-in-time and paused timers alone', () => {
		const running = timer();
		const paused = timer({ id: 'p', state: 'paused', endsAt: null, remainingMs: 1 });
		expect(settle([running, paused], 999_999)).toEqual([running, paused]);
		expect(settle([paused], 9_999_999)).toEqual([paused]);
	});
});

describe('byEnding', () => {
	it('puts the least time left first, paused or not', () => {
		const ringing = timer({ id: 'a', state: 'ringing' });
		const long = timer({ id: 'b', endsAt: 3_792_000 });
		const short = timer({ id: 'c', endsAt: 860_000 });
		const paused = timer({ id: 'd', state: 'paused', endsAt: null, remainingMs: 5_000 });
		const ids = byEnding([long, paused, ringing, short], 0).map((t) => t.id);
		expect(ids).toEqual(['a', 'd', 'c', 'b']);
	});
});

describe('cookPath', () => {
	it('points at the 1-based step', () => {
		expect(cookPath({ slug: 'soup', step: 2 })).toBe('recipes/soup/cook?step=3');
	});
});

describe('parseTimers', () => {
	it('returns [] for null, nonsense and non-arrays', () => {
		expect(parseTimers(null)).toEqual([]);
		expect(parseTimers('nonsense')).toEqual([]);
		expect(parseTimers('{"x":1}')).toEqual([]);
	});
	it('drops malformed entries and keeps valid ones', () => {
		const good = timer();
		const raw = JSON.stringify([good, { id: 'x' }, null, { ...good, state: 'bogus' }]);
		expect(parseTimers(raw)).toEqual([good]);
	});
});

describe('formatClock', () => {
	it('formats hours, minutes and seconds', () => {
		expect(formatClock(3_792_000)).toBe('1:03:12');
		expect(formatClock(860_000)).toBe('14:20');
	});
	it('rounds up to whole seconds', () => {
		expect(formatClock(4_001)).toBe('0:05');
	});
	it('shows zero', () => {
		expect(formatClock(0)).toBe('0:00');
		expect(formatClock(-5)).toBe('0:00');
	});
});

describe('clampStep', () => {
	it('clamps into the step range', () => {
		expect(clampStep(7, 6)).toBe(5);
		expect(clampStep(-1, 6)).toBe(0);
		expect(clampStep(3, 6)).toBe(3);
		expect(clampStep(3, 0)).toBe(0);
	});
});
