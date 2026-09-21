import type { Recipe } from '$lib/api/recipes';
import { m } from '$lib/paraglide/messages';
import type { UserColor } from '$lib/user/color';

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
	return recipe.updatedAt !== recipe.createdAt || recipe.updatedBy !== recipe.createdBy;
}

/**
 * The readable form of a card's authorship, for the `title` a desktop hover
 * shows and the `aria-label` a screen reader announces - the initials
 * themselves carry no text.
 *
 * A card compares (name, colour) pairs rather than calling `hasBeenEdited`:
 * it has no timestamps, and an edit by the author alone adds nothing to a
 * card that already names them. The colour is part of the comparison
 * because a display name is deliberately not unique - two members may both
 * be "Mia" - so the name alone cannot tell them apart; their colours do.
 */
export function authorLabel(
	createdByName: string,
	createdByColor: UserColor,
	updatedByName: string,
	updatedByColor: UserColor
): string {
	return createdByName === updatedByName && createdByColor === updatedByColor
		? m.card_author({ user: createdByName })
		: m.card_author_and_editor({ user: createdByName, editor: updatedByName });
}
