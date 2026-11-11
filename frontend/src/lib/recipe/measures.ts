/**
 * What a unit measures, and how to move an amount between units of one kind.
 * A unit stays free text everywhere (see `units.ts`); this only knows the
 * units that convert exactly, so anything else ("Bund", "EL", "can") is
 * unknown here and is left as written.
 *
 * The shopping list adds amounts up with it today; converting a recipe's
 * units is meant to build on the same table.
 */

export type Dimension = 'mass' | 'volume' | 'count';

/** Factor to the base unit of its kind: grams, milliliters, pieces. */
// ponytail: metric only; add oz, lb, cup and fl oz (with their factors) when recipes convert units.
const MEASURES: Record<string, { dimension: Dimension; factor: number }> = {
	mg: { dimension: 'mass', factor: 0.001 },
	g: { dimension: 'mass', factor: 1 },
	kg: { dimension: 'mass', factor: 1000 },
	ml: { dimension: 'volume', factor: 1 },
	cl: { dimension: 'volume', factor: 10 },
	dl: { dimension: 'volume', factor: 100 },
	l: { dimension: 'volume', factor: 1000 },
	// A bare number counts pieces ("2 Eier").
	'': { dimension: 'count', factor: 1 },
	stück: { dimension: 'count', factor: 1 },
	stücke: { dimension: 'count', factor: 1 },
	stk: { dimension: 'count', factor: 1 },
	pc: { dimension: 'count', factor: 1 },
	pcs: { dimension: 'count', factor: 1 },
	piece: { dimension: 'count', factor: 1 },
	pieces: { dimension: 'count', factor: 1 }
};

const measureOf = (unit: string | null) => MEASURES[(unit ?? '').trim().toLowerCase()];

/** The kind of a unit, or undefined for one that does not convert. */
export function dimensionOf(unit: string | null): Dimension | undefined {
	return measureOf(unit)?.dimension;
}

/** The quantity in its kind's base unit (g, ml, pieces); unknown units pass unchanged. */
export function toBase(quantity: number, unit: string | null): number {
	return quantity * (measureOf(unit)?.factor ?? 1);
}

/**
 * A base quantity in the unit that reads best: kg and l from 1000 on, g and ml
 * below. Pieces keep no unit of their own; the caller names them.
 */
export function fromBase(
	quantity: number,
	dimension: Dimension
): { quantity: number; unit: string } {
	if (dimension === 'mass')
		return quantity >= 1000 ? { quantity: quantity / 1000, unit: 'kg' } : { quantity, unit: 'g' };
	if (dimension === 'volume')
		return quantity >= 1000 ? { quantity: quantity / 1000, unit: 'l' } : { quantity, unit: 'ml' };
	return { quantity, unit: '' };
}
