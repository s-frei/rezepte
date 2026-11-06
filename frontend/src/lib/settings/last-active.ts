import { m } from '#lib/paraglide/messages.js';

const DAY = 86_400_000;

export type ActivityState = 'recent' | 'quiet' | 'dormant' | 'none';

/**
 * What the people list and the person page say about an account's last
 * activity, whether it counts as dormant (30 days or more), and its `state`
 * for the list's dot: recent below 7 days, quiet up to 29, dormant, or none
 * when nothing is recorded. Counted in
 * calendar days; the service records the moment at most hourly. A
 * date ahead of `now` (another clock) reads as today.
 */
export function lastActive(
	at: string | null | undefined,
	now = new Date()
): { text: string; dormant: boolean; state: ActivityState } {
	const date = at ? new Date(at) : null;
	if (!date || Number.isNaN(date.getTime())) {
		return { text: m.person_active_none(), dormant: false, state: 'none' };
	}
	const midnight = (d: Date) => new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime();
	// Rounded: a day across a clock change is 23 or 25 hours long.
	const days = Math.max(0, Math.round((midnight(now) - midnight(date)) / DAY));
	const dormant = days >= 30;
	const state: ActivityState = dormant ? 'dormant' : days < 7 ? 'recent' : 'quiet';
	if (days === 0) return { text: m.person_active_today(), dormant, state };
	if (days === 1) return { text: m.person_active_yesterday(), dormant, state };
	if (days < 7) return { text: m.person_active_days({ count: days }), dormant, state };
	if (days < 60) {
		const weeks = Math.floor(days / 7);
		return {
			text: weeks === 1 ? m.person_active_weeks_one() : m.person_active_weeks({ count: weeks }),
			dormant,
			state
		};
	}
	return { text: m.person_active_months({ count: Math.floor(days / 30) }), dormant, state };
}
