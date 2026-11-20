import Carrot from '@lucide/svelte/icons/carrot';
import Tag from '@lucide/svelte/icons/tag';
import TextAlignStart from '@lucide/svelte/icons/text-align-start';
import type { SearchFind } from '#lib/api/recipes.js';
import { m } from '#lib/paraglide/messages.js';

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
	return find.items.map((item) => item.parts.map((part) => part.text).join('')).join(', ');
}
