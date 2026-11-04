import { getLocale } from '#lib/paraglide/runtime.js';

const STEPS: [Intl.RelativeTimeFormatUnit, number][] = [
	['year', 31536000],
	['month', 2592000],
	['day', 86400],
	['hour', 3600],
	['minute', 60]
];

/**
 * "5 min. ago" / "vor 5 Min." in the app's language; under a minute is "now".
 * No-break spaces keep it on one line.
 */
export function relativeTime(iso: string, now = Date.now()): string {
	const seconds = Math.max(0, Math.round((now - new Date(iso).getTime()) / 1000));
	const [unit, size] = STEPS.find(([, s]) => seconds >= s) ?? ['second', 1];
	const value = unit === 'second' ? 0 : Math.floor(seconds / size);
	return new Intl.RelativeTimeFormat(getLocale(), { numeric: 'auto', style: 'short' })
		.format(-value, unit)
		.replaceAll(' ', '\u00a0');
}
