import { formatQuantity } from './format';

/**
 * Servings scaling (spec §5: "Scaling multiplies `quantity` only"). Units
 * are free text and never converted; the recipe's stored `servings` is the
 * baseline `from`, the user's choice is `to`.
 */

/** How close a fraction has to be to ¼ ⅓ ½ ⅔ ¾ (or a whole number) to snap onto it. */
const SNAP_TOLERANCE = 0.04;

/** Snap targets below one: 0, ¼, ⅓, ½, ⅔, ¾, 1. */
const SNAP_TARGETS_BELOW_ONE = [0, 0.25, 1 / 3, 0.5, 2 / 3, 0.75, 1];

/** Between 1 and 10 only thirds need special care - quarters come out of rounding. */
const THIRDS = [1 / 3, 2 / 3];

/**
 * Multiplies a quantity from `from` servings to `to` servings. The identity
 * case returns the stored value itself (no `x * n / n` floating-point
 * drift), as does a non-positive base or target, which cannot occur with a
 * valid recipe (1-99) but must not produce `Infinity` or `NaN`.
 */
export function scaleQuantity(quantity: number, from: number, to: number): number {
	if (from === to || from <= 0 || to <= 0) {
		return quantity;
	}
	return (quantity * to) / from;
}

function snap(fraction: number, targets: readonly number[]): number | null {
	for (const target of targets) {
		// The epsilon covers binary representation error at the exact edge of
		// the window: `0.29 - 0.25` is 0.04000000000000004, not 0.04.
		if (Math.abs(fraction - target) <= SNAP_TOLERANCE + 1e-9) {
			return target;
		}
	}
	return null;
}

/**
 * Rounds a scaled quantity the way a cook would read it:
 *
 * - below 1: snaps to 0, ¼, ⅓, ½, ⅔, ¾ or 1 when within 0.04, otherwise one
 *   decimal (`0.4`);
 * - 1 to 10: a fractional part within 0.04 of ⅓ or ⅔ keeps the third,
 *   everything else rounds to the nearest quarter (`2.6` → `2.5`);
 * - from 10: whole numbers.
 *
 * Thirds are returned as `n + 1/3` / `n + 2/3`; `formatQuantity` rounds the
 * fraction to two decimals before its glyph lookup, so they render as ⅓ / ⅔.
 * The sign is preserved, although the editor never stores negatives.
 */
export function roundScaled(quantity: number): number {
	const sign = quantity < 0 ? -1 : 1;
	const abs = Math.abs(quantity);
	let rounded: number;
	if (abs < 1) {
		rounded = snap(abs, SNAP_TARGETS_BELOW_ONE) ?? Math.round(abs * 10) / 10;
	} else if (abs < 10) {
		const intPart = Math.floor(abs);
		const third = snap(abs - intPart, THIRDS);
		rounded = third === null ? Math.round(abs * 4) / 4 : intPart + third;
	} else {
		rounded = Math.round(abs);
	}
	return sign * rounded;
}

/** `roundScaled` rendered through the shared quantity formatter (½ ¼ ¾ ⅓ ⅔, German comma). */
export function formatScaled(quantity: number): string {
	return formatQuantity(roundScaled(quantity));
}

/**
 * What an ingredient row shows for `to` servings on a recipe written for
 * `from`. At the baseline the stored value is rendered exactly as before
 * (`formatQuantity`), so switching back to the recipe's servings can never
 * change a number the author typed.
 */
export function formatQuantityFor(quantity: number | null, from: number, to: number): string {
	if (quantity === null) {
		return '';
	}
	if (from === to) {
		return formatQuantity(quantity);
	}
	return formatScaled(scaleQuantity(quantity, from, to));
}
