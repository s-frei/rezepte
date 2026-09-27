/**
 * The unit field's suggestions. A unit is free text to the API and to
 * scaling (which multiplies the quantity only), so the list is a
 * convenience in the reader's language, not a vocabulary: what is picked is
 * stored as written, and a unit of its own ("Bund", "can") is always fine.
 */
import { m } from '$lib/paraglide/messages';

/** The suggested units, in the language the editor is reading. */
export function unitSuggestions(): string[] {
	return [
		m.editor_unit_gram(),
		m.editor_unit_kilogram(),
		m.editor_unit_milliliter(),
		m.editor_unit_liter(),
		m.editor_unit_tablespoon(),
		m.editor_unit_teaspoon(),
		m.editor_unit_piece(),
		m.editor_unit_pinch()
	];
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
