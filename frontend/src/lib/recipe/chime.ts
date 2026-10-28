/** A ringing timer gives up after this long; the dock still shows it. */
export const CHIME_LIMIT_MS = 120_000;
const REPEAT_MS = 2000;

let ctx: AudioContext | null = null;
let repeat: ReturnType<typeof setInterval> | null = null;
let stopAt: ReturnType<typeof setTimeout> | null = null;

function context(): AudioContext | null {
	if (!ctx && typeof AudioContext !== 'undefined') {
		try {
			ctx = new AudioContext();
		} catch {
			return null;
		}
	}
	return ctx;
}

/** Creates or resumes the shared context; browsers only allow that inside a gesture. */
export function unlockAudio(): void {
	const c = context();
	if (c && c.state === 'suspended') {
		void c.resume().catch(() => {});
	}
}

function tone(c: AudioContext, freq: number, at: number): void {
	const osc = c.createOscillator();
	const gain = c.createGain();
	osc.type = 'sine';
	osc.frequency.value = freq;
	gain.gain.setValueAtTime(0.0001, at);
	gain.gain.linearRampToValueAtTime(0.25, at + 0.02);
	gain.gain.exponentialRampToValueAtTime(0.0001, at + 0.4);
	osc.connect(gain);
	gain.connect(c.destination);
	osc.start(at);
	osc.stop(at + 0.45);
}

function ring(): void {
	const c = context();
	if (c) {
		try {
			tone(c, 880, c.currentTime);
			tone(c, 660, c.currentTime + 0.25);
		} catch {
			// A context the browser refuses to play stays silent; the dock still rings.
		}
	}
	navigator.vibrate?.([200, 100, 200]);
}

/** Idempotent: two short tones every 2 s until stopChime or CHIME_LIMIT_MS. */
export function startChime(): void {
	if (repeat !== null) {
		return;
	}
	ring();
	repeat = setInterval(ring, REPEAT_MS);
	stopAt = setTimeout(stopChime, CHIME_LIMIT_MS);
}

export function stopChime(): void {
	if (repeat !== null) {
		clearInterval(repeat);
		repeat = null;
	}
	if (stopAt !== null) {
		clearTimeout(stopAt);
		stopAt = null;
	}
}
