import { m } from '#lib/paraglide/messages.js';
import { getLocale } from '#lib/paraglide/runtime.js';
import type { Timer } from '#lib/recipe/timers.js';

/** "fertig 19:42" - the clock time it ends - or "pausiert". */
export function timerStatus(t: Timer, now: number): string {
	if (t.state === 'paused') {
		return m.timer_paused();
	}
	const time = new Intl.DateTimeFormat(getLocale(), { hour: '2-digit', minute: '2-digit' }).format(
		t.endsAt ?? now
	);
	return m.timer_ends_at({ time });
}
