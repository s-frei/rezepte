import { locales } from '$lib/paraglide/runtime';
import { m } from '$lib/paraglide/messages';
import type { StepTime } from '$lib/api/recipes';
import { formatMinutes } from './format';
import { wordPattern } from './references';

export const MAX_TIME_SECONDS = 7 * 24 * 60 * 60;
/** In code points, the service's `maxLength` on a time's phrase. */
export const MAX_PHRASE_LENGTH = 60;

export const UNIT_SECONDS = { seconds: 1, minutes: 60, hours: 3600 };
export type TimeUnit = keyof typeof UNIT_SECONDS;

export type TimeUnits = Record<TimeUnit | 'range' | 'and', string[]>;

/** Every locale's words at once: a recipe has no language of its own. */
export function catalogUnits(): TimeUnits {
	const all = (read: (locale: (typeof locales)[number]) => string) => [
		...new Set(locales.flatMap((locale) => read(locale).split('|')))
	];
	return {
		seconds: all((locale) => m.time_units_seconds({}, { locale })),
		minutes: all((locale) => m.time_units_minutes({}, { locale })),
		hours: all((locale) => m.time_units_hours({}, { locale })),
		range: all((locale) => m.time_words_range({}, { locale })),
		and: all((locale) => m.time_words_and({}, { locale }))
	};
}

/** The detector's rules are listed in docs/memory/content/features/step-times.mdx. */
const NUM = String.raw`(?:\d+(?:[.,]\d+)?(?:\s?[½¼¾])?|[½¼¾])`;

const esc = (words: string[]) =>
	[...words]
		.sort((a, b) => b.length - a.length)
		.map((w) => w.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'))
		.join('|');

const FRACTIONS: Record<string, number> = { '½': 0.5, '¼': 0.25, '¾': 0.75 };

function parseNum(raw: string): number {
	let total = 0;
	let rest = raw.trim();
	const frac = rest.match(/\s?([½¼¾])$/u);
	if (frac) {
		total += FRACTIONS[frac[1]];
		rest = rest.slice(0, frac.index).trim();
	}
	if (rest) total += parseFloat(rest.replace(',', '.'));
	return total;
}

function valid(t: StepTime): boolean {
	return (
		[...t.phrase].length <= MAX_PHRASE_LENGTH &&
		t.seconds >= 1 &&
		t.seconds <= MAX_TIME_SECONDS &&
		(t.maxSeconds === undefined || (t.maxSeconds > t.seconds && t.maxSeconds <= MAX_TIME_SECONDS))
	);
}

/**
 * The phrase without a final dot that ends the sentence. "Min." keeps its dot
 * where the sentence goes on in lower case ("10 Min. backen"); before the end
 * of the text or a capital letter the dot is the sentence's, and marking it
 * would underline punctuation ("bake 10 min." matching the German "Min.").
 */
function ownDot(phrase: string, after: string): string {
	if (!phrase.endsWith('.')) return phrase;
	return /^\s*\p{Ll}/u.test(after) ? phrase : phrase.slice(0, -1);
}

export function suggestTimes(text: string, units: TimeUnits = catalogUnits()): StepTime[] {
	const factor = new Map<string, number>();
	for (const [name, k] of Object.entries(UNIT_SECONDS))
		for (const w of units[name as TimeUnit]) factor.set(w.toLowerCase(), k);
	const unit = `(${esc([...units.seconds, ...units.minutes, ...units.hours])})(?![\\p{L}\\p{N}])`;
	const hour = `(${esc(units.hours)})(?![\\p{L}\\p{N}])`;
	const min = `(${esc(units.minutes)})(?![\\p{L}\\p{N}])`;
	const lead = String.raw`(?<![\p{L}\p{N}])`;
	const mult = (u: string) => factor.get(u.toLowerCase()) ?? 0;

	const found: { start: number; len: number; time: StepTime }[] = [];
	const scan = (re: RegExp, build: (g: RegExpExecArray) => Omit<StepTime, 'phrase'>) => {
		for (const g of text.matchAll(new RegExp(re, 'giu'))) {
			found.push({
				start: g.index,
				len: g[0].length,
				time: {
					phrase: ownDot(g[0], text.slice(g.index + g[0].length)),
					...build(g as RegExpExecArray)
				}
			});
		}
	};
	scan(
		new RegExp(
			`${lead}(${NUM})\\s+${hour}\\s+(?:(?:${esc(units.and)})\\s+)?(${NUM})\\s+${min}`,
			'u'
		),
		(g) => ({
			seconds: Math.round(parseNum(g[1]) * mult(g[2]) + parseNum(g[3]) * mult(g[4]))
		})
	);
	scan(
		new RegExp(
			`${lead}(${NUM})\\s*(?:[-–—]|\\s(?:${esc(units.range)})\\s)\\s*(${NUM})\\s+${unit}`,
			'u'
		),
		(g) => ({
			seconds: Math.round(parseNum(g[1]) * mult(g[3])),
			maxSeconds: Math.round(parseNum(g[2]) * mult(g[3]))
		})
	);
	scan(new RegExp(`${lead}(${NUM})\\s+${unit}`, 'u'), (g) => ({
		seconds: Math.round(parseNum(g[1]) * mult(g[2]))
	}));

	found.sort((a, b) => a.start - b.start || b.len - a.len);
	const out: StepTime[] = [];
	let end = 0;
	for (const f of found) {
		if (f.start < end) continue;
		end = f.start + f.len; // an invalid match still claims its span, so "25–20 min" does not yield "20 min"
		// A stored phrase anchors to its first whole-word occurrence, so a repeat
		// ("20 Minuten … weitere 20 Minuten") or one inside a compound is no suggestion.
		if (valid(f.time) && wordPattern(f.time.phrase).exec(text)?.index === f.start) out.push(f.time);
	}
	return out;
}

/** Reads the number or range out of an author's selection; the unit is the author's choice. */
export function manualTime(phrase: string, unit: TimeUnit): StepTime | null {
	const k = UNIT_SECONDS[unit];
	const g = phrase.match(new RegExp(`(${NUM})(?:\\s*[-–—]\\s*(${NUM}))?`, 'u'));
	if (!g) return null;
	const time: StepTime = { phrase, seconds: Math.round(parseNum(g[1]) * k) };
	if (g[2]) time.maxSeconds = Math.round(parseNum(g[2]) * k);
	return valid(time) ? time : null;
}

/** "45 sec", "5 min 30 sec", "1 hr 30 min", in the active language. */
function one(s: number): string {
	const min = Math.floor(s / 60);
	if (s % 60 === 0) return formatMinutes(min);
	const sec = m.duration_seconds({ count: s % 60 });
	return min === 0 ? sec : `${formatMinutes(min)} ${sec}`;
}

/** A range shares a lone unit ("20–25 min"), otherwise each end has its own ("50 sec–1 min 10 sec"). */
export function formatDuration({ seconds, maxSeconds }: StepTime): string {
	if (maxSeconds === undefined) return one(seconds);
	const [low, high] = [one(seconds), one(maxSeconds)];
	const a = low.match(/^(\d+)(\s\D+)$/u);
	const b = high.match(/^\d+(\s\D+)$/u);
	return a && b && a[2] === b[1] ? `${a[1]}–${high}` : `${low}–${high}`;
}
