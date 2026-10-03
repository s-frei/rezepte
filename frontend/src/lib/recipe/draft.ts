import type { RecipeDraft } from '$lib/api/drafts';

/** An import on its way from the dialog to /recipes/new. Not reactive:
 * the editor reads it once, the way RecipeForm reads `initial`. */
export type ImportedDraft = { draft: RecipeDraft; photo: File | null; photoFailed: boolean };

let held: ImportedDraft | null = null;

export function putDraft(draft: ImportedDraft): void {
	held = draft;
}

/** Hands the draft over once; a reload of /recipes/new finds nothing. Also
 * drops a draft whose navigation was given up ("Keep editing" in the
 * editor's discard dialog), so it never turns up in a later "New recipe". */
export function takeDraft(): ImportedDraft | null {
	const draft = held;
	held = null;
	return draft;
}
