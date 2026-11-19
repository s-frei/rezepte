import type { Comment } from '#lib/api/comments.js';
import { m } from '#lib/paraglide/messages.js';
import { getLocale } from '#lib/paraglide/runtime.js';
import { formatDay } from './format';

export type DiaryDay<T = Comment> = {
	/** The local calendar day, YYYY-MM-DD; stable for keyed each blocks. */
	key: string;
	/** Today, yesterday, or the date in the active locale, with its weekday unless asked not to. */
	label: string;
	entries: T[];
};

/** The local calendar day as YYYY-MM-DD (Swedish writes dates ISO-style). */
const dayKey = (d: Date) => d.toLocaleDateString('sv');

/**
 * Groups entries by the reader's local calendar day, keeping their order
 * (the diary's oldest first, the mail outbox's newest first). `weekday`
 * false drops the weekday from older days, for a label that has to fit a
 * phone on one line.
 */
export function groupByDay<T extends { createdAt: string }>(
	comments: T[],
	now: Date,
	{ weekday = true } = {}
): DiaryDay<T>[] {
	const today = dayKey(now);
	const y = new Date(now);
	y.setDate(y.getDate() - 1);
	const yesterday = dayKey(y);
	const days: DiaryDay<T>[] = [];
	for (const c of comments) {
		const d = new Date(c.createdAt);
		const key = dayKey(d);
		let day = days.at(-1);
		if (day?.key !== key) {
			const label =
				key === today
					? m.diary_today()
					: key === yesterday
						? m.diary_yesterday()
						: formatDay(d, { weekday, year: d.getFullYear() !== now.getFullYear() });
			day = { key, label, entries: [] };
			days.push(day);
		}
		day.entries.push(c);
	}
	return days;
}

/** The entry's time of day, HH:MM in the active locale. */
export function entryTime(iso: string): string {
	return new Intl.DateTimeFormat(getLocale(), { hour: '2-digit', minute: '2-digit' }).format(
		new Date(iso)
	);
}

/** The service's limit on an entry, in runes. */
export const DIARY_MAX = 2000;

/**
 * Code points over the limit once trimmed (zero or less when it fits).
 * `[...s]` counts code points, the same way the service counts runes.
 */
export function overLimit(body: string): number {
	return [...body.trim()].length - DIARY_MAX;
}

/** Whether body can be written: not blank and within the limit. */
export const sendable = (body: string) => body.trim() !== '' && overLimit(body) <= 0;
