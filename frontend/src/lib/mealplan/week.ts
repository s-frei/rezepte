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
