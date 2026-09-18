import { m } from '$lib/paraglide/messages';

/** Unicode glyphs for the fractional quantities that come up in recipes. */
const FRACTION_GLYPHS: Record<string, string> = {
	'0.5': '½',
	'0.25': '¼',
	'0.75': '¾',
	'0.33': '⅓',
	'0.67': '⅔'
};

/**
 * Formats an ingredient quantity for display.
 *
 * - `null` renders as an empty string.
 * - Whole numbers render plain (`2`).
 * - Common fractions (½ ¼ ¾ ⅓ ⅔) render as their Unicode glyph, with a
 *   leading integer part when there is one (`1 ½`) - this takes priority
 *   over the decimal branch below, so e.g. `1.25` is `1 ¼`, not `1,25`.
 * - Anything else renders with up to two decimals and a German comma
 *   (`1,1`, `1,45`).
 * - The sign is preserved for negative quantities (`-1 ½`, `-1,1`).
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
	return sign + String(rounded).replace('.', ',');
}

/**
 * Formats a duration in minutes for display.
 *
 * - `null` renders as an empty string.
 * - Under an hour renders as minutes (`30 Min`).
 * - An hour or more renders as hours and, when there's a remainder,
 *   minutes (`1 Std 30 Min`, `2 Std`).
 *
 * The unit copy comes from `messages/de.json` (`duration_*`), not hardcoded
 * here.
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
 * The scaling factor between two servings counts, for the "×1,5" hint:
 * up to two decimals, German comma, no trailing zeros (`2`, `0,5`, `2,33`).
 */
export function formatFactor(from: number, to: number): string {
	const ratio = Math.round((to / from) * 100) / 100;
	return String(ratio).replace('.', ',');
}

/**
 * "Portion" for exactly one, "Portionen" otherwise. Two plain keys instead
 * of an inlang plural variant: `de.json` uses none anywhere else, and a
 * one-vs-many split is all German needs here.
 */
export function servingsUnit(count: number): string {
	return count === 1 ? m.servings_unit_one() : m.servings_unit_other();
}

/** `4 Portionen`, `1 Portion` - number and unit together, for captions. */
export function formatServings(count: number): string {
	return `${count} ${servingsUnit(count)}`;
}
