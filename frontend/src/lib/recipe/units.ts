/**
 * The unit field's suggestions. A unit is free text to the API and to
 * scaling (which multiplies the quantity only), so the list is a
 * convenience in the reader's language, not a vocabulary: what is picked is
 * stored as written, and a unit of its own ("Bund", "can") is always fine.
 */
import { m } from '$lib/paraglide/messages';

/**
 * Splits a catalog's comma-separated unit list. One message per language
 * rather than one per unit, because the lists do not line up: an American
 * kitchen measures in cups and ounces, a German one in Bund and Zehe, and a
 * key per unit would force every language to carry the same entries.
 */
export function parseUnitList(list: string): string[] {
	return list
		.split(',')
		.map((unit) => unit.trim())
		.filter((unit) => unit !== '');
}

/** The suggested units, in the language the editor is reading. */
export function unitSuggestions(): string[] {
	return parseUnitList(m.editor_unit_options());
}

/**
 * What the open list shows. Opening the field - a tap, a focus, the chevron
 * - always lists every unit, even over a value, because that is when someone
 * wants to swap one unit for another; a native datalist filters by the value
 * instead and shows nothing over "EL". Only typing narrows the list.
 */
export function unitOptions(units: string[], value: string, typed: boolean): string[] {
	const query = value.trim().toLowerCase();
	if (!typed || query === '') {
		return units;
	}
	return units.filter((unit) => unit.toLowerCase().includes(query));
}
