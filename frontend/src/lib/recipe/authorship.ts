import type { Person, Recipe } from '$lib/api/recipes';
import { m } from '$lib/paraglide/messages';

/**
 * Whether a recipe has been edited since it was written, which is what
 * decides if the colophon carries its second line.
 *
 * A new recipe seeds `updatedBy`/`updatedAt` from its author, so equality on
 * both sides means "untouched". Both halves are needed: timestamps have
 * second resolution, so an edit landing in the same second as the create
 * leaves `updatedAt` identical - and an edit by somebody else has to be
 * credited whatever the clock says.
 */
export function hasBeenEdited(
	recipe: Pick<Recipe, 'createdBy' | 'createdAt' | 'updatedBy' | 'updatedAt'>
): boolean {
	return recipe.updatedAt !== recipe.createdAt || recipe.updatedBy.id !== recipe.createdBy.id;
}

/**
 * The readable form of a card's authorship, for the `title` a desktop hover
 * shows and the `aria-label` a screen reader announces - the initials
 * themselves carry no text.
 *
 * A card compares the two people rather than calling `hasBeenEdited`: it has
 * no timestamps, and an edit by the author alone adds nothing to a card that
 * already names them. By id, because a display name is deliberately not
 * unique - two members may both be "Mia".
 */
export function authorLabel(createdBy: Person, updatedBy: Person): string {
	return createdBy.id === updatedBy.id
		? m.card_author({ user: createdBy.displayName })
		: m.card_author_and_editor({ user: createdBy.displayName, editor: updatedBy.displayName });
}
