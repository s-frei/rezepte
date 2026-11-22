import { getDayOfWeek, parseDate, type CalendarDate } from '@internationalized/date';
import { m } from '#lib/paraglide/messages.js';
import { getLocale, type Locale } from '#lib/paraglide/runtime.js';

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

/** `day` moved by `by` days. */
export const shiftDay = (day: string, by: number): string =>
	parseDate(day).add({ days: by }).toString();

const EPOCH = parseDate('2000-01-03');
/** `day` as a count of days (a ruler's coordinate); `dayOf` turns it back. */
export const dayNumber = (day: string): number => parseDate(day).compare(EPOCH);
export const dayOf = (n: number): string => EPOCH.add({ days: Math.floor(n) }).toString();

/** The days from `from` to `to`, both in. */
export function daysFrom(from: string, to: string): string[] {
	const days: string[] = [];
	for (let day = parseDate(from); day.toString() <= to; day = day.add({ days: 1 }))
		days.push(day.toString());
	return days;
}

/** `monday` moved by `by` weeks. */
export const shiftWeek = (monday: string, by: number): string =>
	parseDate(monday).add({ weeks: by }).toString();

/**
 * "16.–22. November" / "November 16–22", with `short` months "16.–22. Nov."
 * and with `numeric` ones "16.–22.11." / "30.11.–6.12." for a narrow phone. Never a year: a
 * week across New Year reads "28. Dez. – 3. Jan.", the position line above
 * it says when. A week within one month names it once (`formatRange`, whose
 * spacing around the dash differs between engines, so it is set here: none);
 * across two, each end is formatted alone and joined by a dash, spaced
 * unless numeric.
 */
export const weekRange = (
	monday: string,
	locale: Locale,
	month: 'long' | 'short' | 'numeric' = 'long'
): string => dayRange(monday, parseDate(monday).add({ days: 6 }).toString(), locale, month);

/** `weekRange` for any two days, the second not before the first. */
export function dayRange(
	from: string,
	to: string,
	locale: Locale,
	month: 'long' | 'short' | 'numeric' = 'long'
): string {
	const start = parseDate(from);
	const end = parseDate(to);
	const format = new Intl.DateTimeFormat(locale, { day: 'numeric', month, timeZone: 'UTC' });
	const [a, b] = [start.toDate('UTC'), end.toDate('UTC')];
	// Numeric ends are short enough to go unspaced, and Chromium pads a
	// numeric range's days ("07.–13.12."), which no one writes.
	const dash = month === 'numeric' ? '–' : ' – ';
	const range =
		start.month === end.month
			? format.formatRange(a, b).replace(/\s*–\s*/, '–')
			: `${format.format(a)}${dash}${format.format(b)}`;
	return month === 'numeric' ? range.replace(/(^|\D)0(\d)/g, '$1$2') : range;
}

/** "Mo 23. Nov." / "Mon Nov 23". */
export function dayLabel(day: string, locale: Locale): string {
	const names = dayNames(day, locale);
	return `${names.weekday} ${names.date}`;
}

/**
 * What a span of days is called in a name: "23.–29. Nov.", one day "Mo 23. Nov.".
 * Its spaces are no-break ones but the one after a dash, so a narrow title
 * wraps behind the dash, never before it and never inside a date.
 */
export function spanLabel(from: string, to: string, locale: Locale): string {
	const label = from === to ? dayLabel(from, locale) : dayRange(from, to, locale, 'short');
	return label.replace(/\s/g, '\u00A0').replace('–\u00A0', '– ');
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

/** "16.11." / "11/16": a stamp's date beside its weekday. */
export function stampDate(day: string): string {
	return new Intl.DateTimeFormat(getLocale(), {
		day: 'numeric',
		month: 'numeric',
		timeZone: 'UTC'
	}).format(parseDate(day).toDate('UTC'));
}

/**
 * Where the week starting `monday` lies from the one starting `current`:
 * "This week", "Next week", "Last week", "In 3 weeks", "2 weeks ago".
 */
export function weekFromNow(monday: string, current: string, locale: Locale = getLocale()): string {
	const by = parseDate(monday).compare(parseDate(current)) / 7;
	const said = new Intl.RelativeTimeFormat(locale, { numeric: 'auto' }).format(by, 'week');
	return said[0].toLocaleUpperCase(locale) + said.slice(1);
}

/**
 * The first of `count` days in view with today in their middle (left of
 * it when the middle falls between two): 3 days back from 7, 1 from 4.
 */
export const aroundToday = (today: string, count: number): string =>
	shiftDay(today, -Math.floor((count - 1) / 2));

/**
 * Where `count` days shown from `start` lie, never to be misread for past
 * or future: "Around today" with today in their middle (`aroundToday`); a
 * whole Monday-to-Sunday week as that week (`weekFromNow`: "This week",
 * "Next week"); else from their first day, "From today", "From tomorrow",
 * "From yesterday", or by its date: "From Thu Nov 19".
 */
export function windowFromNow(
	start: string,
	today: string,
	count: number,
	locale: Locale = getLocale()
): string {
	if (start === aroundToday(today, count)) return m.week_around_today({}, { locale });
	const date = parseDate(start);
	if (count === 7 && getDayOfWeek(date, 'en', 'mon') === 0)
		return weekFromNow(start, mondayOf(today), locale);
	const by = date.compare(parseDate(today));
	const day =
		Math.abs(by) <= 1
			? new Intl.RelativeTimeFormat(locale, { numeric: 'auto' }).format(by, 'day')
			: dayLabel(start, locale);
	return m.week_from_day({ day }, { locale });
}
