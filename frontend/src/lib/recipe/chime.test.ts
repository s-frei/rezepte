import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { CHIME_LIMIT_MS } from './chime';

// Fresh module per test: the shared AudioContext is cached at module level.
let startChime: typeof import('./chime').startChime;
let stopChime: typeof import('./chime').stopChime;
let unlockAudio: typeof import('./chime').unlockAudio;

let oscillators = 0;
let resumed = 0;
let initialState = 'running';

class FakeAudioContext {
	state = initialState;
	currentTime = 0;
	destination = {};
	resume() {
		resumed++;
		this.state = 'running';
		return Promise.resolve();
	}
	createOscillator() {
		oscillators++;
		return {
			type: '',
			frequency: { value: 0 },
			connect: vi.fn(),
			start: vi.fn(),
			stop: vi.fn()
		};
	}
	createGain() {
		return {
			gain: {
				setValueAtTime: vi.fn(),
				linearRampToValueAtTime: vi.fn(),
				exponentialRampToValueAtTime: vi.fn()
			},
			connect: vi.fn()
		};
	}
}

beforeEach(async () => {
	initialState = 'running';
	vi.useFakeTimers();
	oscillators = 0;
	resumed = 0;
	vi.stubGlobal('AudioContext', FakeAudioContext);
	vi.stubGlobal('navigator', { vibrate: vi.fn() });
	vi.resetModules();
	({ startChime, stopChime, unlockAudio } = await import('./chime'));
});

afterEach(() => {
	stopChime();
	vi.useRealTimers();
	vi.unstubAllGlobals();
});

describe('chime', () => {
	it('startChime twice schedules one repeat', () => {
		startChime();
		startChime();
		expect(vi.getTimerCount()).toBeGreaterThan(0);
		const first = oscillators;
		vi.advanceTimersByTime(2000);
		// One repeat adds one pair of tones, not two.
		expect(oscillators - first).toBe(first);
	});

	it('plays until the limit, then stops itself', () => {
		startChime();
		vi.advanceTimersByTime(CHIME_LIMIT_MS + 5000);
		const played = oscillators;
		vi.advanceTimersByTime(10_000);
		expect(oscillators).toBe(played);
		expect(vi.getTimerCount()).toBe(0);
	});

	it('stopChime stops it', () => {
		startChime();
		stopChime();
		const played = oscillators;
		vi.advanceTimersByTime(10_000);
		expect(oscillators).toBe(played);
		expect(vi.getTimerCount()).toBe(0);
	});

	it('vibrates with each chime', () => {
		startChime();
		vi.advanceTimersByTime(2000);
		expect(navigator.vibrate).toHaveBeenCalledWith([200, 100, 200]);
		expect(vi.mocked(navigator.vibrate).mock.calls.length).toBe(2);
	});

	it('queues no tones while the context is suspended, then plays after unlock', () => {
		initialState = 'suspended';
		startChime();
		vi.advanceTimersByTime(10_000);
		expect(oscillators).toBe(0);
		expect(navigator.vibrate).toHaveBeenCalledTimes(6);
		unlockAudio();
		vi.advanceTimersByTime(2000);
		expect(oscillators).toBe(2);
	});

	it('unlockAudio resumes the shared context', () => {
		initialState = 'suspended';
		unlockAudio();
		unlockAudio();
		expect(resumed).toBe(1);
	});

	it('does nothing without an AudioContext', async () => {
		vi.stubGlobal('AudioContext', undefined);
		vi.resetModules();
		const fresh = await import('./chime');
		expect(() => {
			fresh.unlockAudio();
			fresh.startChime();
			vi.advanceTimersByTime(5000);
			fresh.stopChime();
		}).not.toThrow();
	});
});
