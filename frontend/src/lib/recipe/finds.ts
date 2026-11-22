import Carrot from '@lucide/svelte/icons/carrot';
import Tag from '@lucide/svelte/icons/tag';
import TextAlignStart from '@lucide/svelte/icons/text-align-start';
import type { PantryItem, SearchFind } from '#lib/api/recipes.js';
import { m } from '#lib/paraglide/messages.js';
import { covers } from './covers.js';

/** Each field's icon, the same on a card and in the command palette. */
export const findIcons = { tags: Tag, ingredients: Carrot, description: TextAlignStart };

/** The field's name, plural when it names more than one find. */
export function findLabel(find: SearchFind): string {
	const several = find.items.length + (find.field === 'description' ? 0 : find.more) > 1;
	switch (find.field) {
		case 'tags':
			return several ? m.search_find_tags() : m.search_find_tag();
		case 'ingredients':
			return several ? m.search_find_ingredients() : m.search_find_ingredient();
		default:
			return m.search_find_description();
	}
}

/** The find as plain text, for the tooltip of a line cut short. */
export function findText(find: SearchFind): string {
	return find.items.map(itemText).join(', ');
}

/** What a recipe still needs once the ticks have covered what they serve. */
export function missingAfterTicks(missing: PantryItem[], ticked: PantryItem[]): PantryItem[] {
	return missing.filter((need) => !ticked.some((have) => covers(have.key, need.key)));
}

/** The first `fits` names and how many that leaves out. */
export function clampNames(names: string[], fits: number): { shown: string[]; more: number } {
	const shown = names.slice(0, fits);
	return { shown, more: names.length - shown.length };
}

/**
 * The text finds without an ingredient find the card's pantry lines already
 * name, so an ingredient is named once per card; the lines mark its hit.
 * A find with more matches than it lists keeps its line, since what it
 * leaves out may be no pantry name (water never counts as missing).
 */
export function dedupeTextFinds(finds: SearchFind[], pantryNames: string[]): SearchFind[] {
	return finds.filter(
		(find) =>
			find.field !== 'ingredients' ||
			find.more > 0 ||
			!find.items.every((item) => pantryNames.includes(itemText(item)))
	);
}

/** An item of a find as plain text. */
export function itemText(item: SearchFind['items'][number]): string {
	return item.parts.map((part) => part.text).join('');
}
