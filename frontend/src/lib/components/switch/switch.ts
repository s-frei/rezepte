import { m } from '#lib/paraglide/messages.js';
import type { Locale } from '#lib/paraglide/runtime.js';

const DAY = 86_400_000;

const midnight = (d: Date) => new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime();

/**
 * When a person was last here, on their card: today, yesterday, the weekday
 * up to six days back, then last week, then whole weeks ago. Never more than
 * 30 days back in practice, since a session unused that long has expired and
 * its card says "Sign in again". Counted in calendar days, so last night at
 * 23:00 is yesterday.
 */
export function seenLabel(lastSeenAt: string, now: Date, locale: string): string {
	const seen = new Date(lastSeenAt);
	const days = Math.round((midnight(now) - midnight(seen)) / DAY);
	const options = { locale: locale as Locale };
	if (days <= 0) return m.switch_seen_today({}, options);
	if (days === 1) return m.switch_seen_yesterday({}, options);
	if (days < 7) {
		return m.switch_seen_on({ day: seen.toLocaleDateString(locale, { weekday: 'long' }) }, options);
	}
	if (days < 14) return m.switch_seen_last_week({}, options);
	return m.switch_seen_weeks({ count: Math.floor(days / 7) }, options);
}

/** A card's name a step smaller over 16 and over 28 characters (two lines at most). */
export function nameSize(name: string): 'lg' | 'md' | 'sm' {
	return name.length > 28 ? 'sm' : name.length > 16 ? 'md' : 'lg';
}

/** The splash greets by the first word of the name; a hyphenated one stays whole. */
export function firstName(displayName: string): string {
	return displayName.trim().split(/\s+/)[0];
}
