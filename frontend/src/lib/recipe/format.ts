import { getLocale } from '$lib/paraglide/runtime';
import { m } from '$lib/paraglide/messages';

/** Unicode glyphs for the fractional quantities that come up in recipes. */
const FRACTION_GLYPHS: Record<string, string> = {
	'0.5': '½',
	'0.25': '¼',
	'0.75': '¾',
	'0.33': '⅓',
	'0.67': '⅔'
};

// Intl formatters are expensive to build and the locale only changes on a
// reload (Paraglide's setLocale reloads the page), so one instance per locale
// is enough. Keyed on the locale rather than built once, so the pair survives
// a locale change without anyone having to find this line - the same reason
// the user list's collator is cached this way.
let numberFormat: Intl.NumberFormat | undefined;
let numberFormatLocale: string | undefined;

function decimals(): Intl.NumberFormat {
	const locale = getLocale();
	if (!numberFormat || numberFormatLocale !== locale) {
		numberFormat = new Intl.NumberFormat(locale, { maximumFractionDigits: 2 });
		numberFormatLocale = locale;
	}
	return numberFormat;
}

/**
 * Formats an ingredient quantity for display.
 *
 * - `null` renders as an empty string.
 * - Whole numbers render plain (`2`).
 * - Common fractions (½ ¼ ¾ ⅓ ⅔) render as their Unicode glyph, with a
 *   leading integer part when there is one (`1 ½`) - this takes priority
 *   over the decimal branch below, so e.g. `1.25` is `1 ¼`, not `1.25`.
 * - Anything else renders with up to two decimals and the locale's own
 *   decimal separator (`1.1`, `1.45` in English; `1,1`, `1,45` in German).
 * - The sign is preserved for negative quantities (`-1 ½`, `-1.1`).
 */
export function formatQuantity(quantity: number | null): string {
	if (quantity === null) {
		return '';
	}
	if (Number.isInteger(quantity)) {
		return String(quantity);
	}

	const sign = quantity < 0 ? '-' : '';
	const abs = Math.abs(quantity);
	const intPart = Math.floor(abs);
	// Round to 2 decimals first so binary floating-point noise (e.g. 1.33 - 1
	// = 0.32999999999999996) doesn't miss the fraction glyph lookup below.
	const fraction = Math.round((abs - intPart) * 100) / 100;
	const glyph = FRACTION_GLYPHS[String(fraction)];
	if (glyph) {
		return sign + (intPart > 0 ? `${intPart} ${glyph}` : glyph);
	}

	const rounded = Math.round(abs * 100) / 100;
	return sign + decimals().format(rounded);
}

/**
 * Formats a duration in minutes for display.
 *
 * - `null` renders as an empty string.
 * - Under an hour renders as minutes (`30 min`).
 * - An hour or more renders as hours and, when there's a remainder,
 *   minutes (`1 hr 30 min`, `2 hr`).
 *
 * The unit copy comes from the message catalogues (`duration_*`), not
 * hardcoded here, so the examples read as the active language does.
 */
export function formatMinutes(minutes: number | null): string {
	if (minutes === null) {
		return '';
	}
	if (minutes < 60) {
		return m.duration_minutes({ count: minutes });
	}

	const hours = Math.floor(minutes / 60);
	const rest = minutes % 60;
	return rest === 0
		? m.duration_hours({ count: hours })
		: m.duration_hours_minutes({ hours, minutes: rest });
}

/**
 * The scaling factor between two servings counts, for the "×1.5" hint:
 * up to two decimals, the locale's own decimal separator, no trailing zeros
 * (`2`, `0.5`, `2.33` in English; `0,5`, `2,33` in German).
 */
export function formatFactor(from: number, to: number): string {
	const ratio = Math.round((to / from) * 100) / 100;
	return decimals().format(ratio);
}

/**
 * "serving" for exactly one, "servings" otherwise. Two plain keys instead of
 * an inlang plural variant: no catalogue uses one anywhere else, and a
 * one-vs-many split is all either language needs here.
 */
export function servingsUnit(count: number): string {
	return count === 1 ? m.servings_unit_one() : m.servings_unit_other();
}

/** `4 servings`, `1 serving` - number and unit together, for captions. */
export function formatServings(count: number): string {
	return `${count} ${servingsUnit(count)}`;
}

let dateFormat: Intl.DateTimeFormat | undefined;
let dateFormatLocale: string | undefined;

function longDate(): Intl.DateTimeFormat {
	const locale = getLocale();
	if (!dateFormat || dateFormatLocale !== locale) {
		dateFormat = new Intl.DateTimeFormat(locale, {
			day: 'numeric',
			month: 'long',
			year: 'numeric'
		});
		dateFormatLocale = locale;
	}
	return dateFormat;
}

/**
 * An API timestamp as a long date in the active locale (`March 3, 2026` in
 * English, `3. März 2026` in German). Anything the browser cannot parse
 * renders as an empty string rather than the "Invalid Date" the formatter
 * would otherwise produce - the colophon this feeds reads as a sentence, and
 * a broken date there should go quiet.
 */
export function formatDate(iso: string): string {
	const date = new Date(iso);
	return Number.isNaN(date.getTime()) ? '' : longDate().format(date);
}
