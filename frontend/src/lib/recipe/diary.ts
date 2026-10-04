import type { Comment } from '$lib/api/comments';
import { m } from '$lib/paraglide/messages';
import { getLocale } from '$lib/paraglide/runtime';
import { formatDay } from './format';

export type DiaryDay = {
	/** The local calendar day, YYYY-MM-DD; stable for keyed each blocks. */
	key: string;
	/** Today, yesterday, or the weekday and date in the active locale. */
	label: string;
	entries: Comment[];
};

/** The local calendar day as YYYY-MM-DD (Swedish writes dates ISO-style). */
const dayKey = (d: Date) => d.toLocaleDateString('sv');

/** Groups entries (already oldest first) by the reader's local calendar day. */
export function groupByDay(comments: Comment[], now: Date): DiaryDay[] {
	const today = dayKey(now);
	const y = new Date(now);
	y.setDate(y.getDate() - 1);
	const yesterday = dayKey(y);
	const days: DiaryDay[] = [];
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
						: formatDay(d, { year: d.getFullYear() !== now.getFullYear() });
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
