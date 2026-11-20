import { getDayOfWeek, parseDate, type CalendarDate } from '@internationalized/date';
import type { Locale } from '#lib/paraglide/runtime.js';

/**
 * Plan days are dates (`YYYY-MM-DD`), never instants: a week is the same
 * seven days in every time zone. Weeks start on Monday for everyone.
 */

const monday = (date: CalendarDate) => date.subtract({ days: getDayOfWeek(date, 'en', 'mon') });

/** The Monday of the week `day` falls in. */
export const mondayOf = (day: string): string => monday(parseDate(day)).toString();

/** The seven days of the week starting `monday`. */
export function weekDays(monday: string): string[] {
	const start = parseDate(monday);
	return Array.from({ length: 7 }, (_, i) => start.add({ days: i }).toString());
}

/** `monday` moved by `by` weeks. */
export const shiftWeek = (monday: string, by: number): string =>
	parseDate(monday).add({ weeks: by }).toString();

/**
 * "16.–22. November" / "November 16–22", or with `short` months
 * "16.–22. Nov." for a phone. The range comes from `Intl`, whose spacing
 * around the dash differs between engines, so it is set here: none within a
 * month, a space on each side across two.
 */
export function weekRange(
	monday: string,
	locale: Locale,
	month: 'long' | 'short' = 'long'
): string {
	const start = parseDate(monday);
	const end = start.add({ days: 6 });
	const format = new Intl.DateTimeFormat(locale, { day: 'numeric', month, timeZone: 'UTC' });
	return format
		.formatRange(start.toDate('UTC'), end.toDate('UTC'))
		.replace(/\s*–\s*/, start.month === end.month ? '–' : ' – ');
}

/** The days before `today` in the week starting `monday`; none for any other week. */
export function pastDays(monday: string, today: string): string[] {
	if (mondayOf(today) !== monday) return [];
	return weekDays(monday).filter((day) => day < today);
}

/**
 * One day's names, from `Intl`: `weekday` "Mo" / "Mon" (de's dot dropped,
 * as on a calendar), `long` "Montag", `date` "16. Nov." / "Nov 16", `number`
 * "16." / "16", and `full` "Montag, 16. November" for a screen reader.
 */
export function dayNames(day: string, locale: Locale) {
	const date = parseDate(day).toDate('UTC');
	const format = (options: Intl.DateTimeFormatOptions) =>
		new Intl.DateTimeFormat(locale, { ...options, timeZone: 'UTC' }).format(date);
	// A day alone has no ordinal dot in `Intl`; taken from beside its month,
	// it keeps the one German writes ("16." against "Nov 16").
	const parts = new Intl.DateTimeFormat(locale, {
		day: 'numeric',
		month: 'short',
		timeZone: 'UTC'
	}).formatToParts(date);
	const at = parts.findIndex((part) => part.type === 'day');
	const dot = parts[at + 1]?.value.startsWith('.') ? '.' : '';
	return {
		weekday: format({ weekday: 'short' }).replace(/\.$/, ''),
		long: format({ weekday: 'long' }),
		date: format({ day: 'numeric', month: 'short' }),
		number: parts[at].value + dot,
		full: format({ weekday: 'long', day: 'numeric', month: 'long' })
	};
}
