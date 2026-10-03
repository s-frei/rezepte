import { api, ApiError } from './client';
import type { Person, RecipeInput } from './recipes';

/** An ingredient line the parser was not sure about: group and row in `recipe`. */
export type DraftReview = { group: number; ingredient: number; line: string };
export type DraftDuplicate = { id: string; slug: string; title: string; createdBy: Person };
export type RecipeDraft = {
	recipe: RecipeInput;
	review: DraftReview[];
	suggestedTags: string[];
	photo: { href: string } | null;
	duplicate: DraftDuplicate | null;
	truncated: boolean;
};
const FAILURES = ['no-recipe', 'unreachable', 'no-recipe-in-text'] as const;
export type DraftFailure = (typeof FAILURES)[number];

export function createDraft(
	source: { url: string } | { text: string },
	signal?: AbortSignal
): Promise<RecipeDraft> {
	return api<RecipeDraft>('/recipe-drafts', {
		method: 'POST',
		body: JSON.stringify(source),
		signal
	});
}

/** The import's own reason for a 422, or null for any other error. */
export function draftFailure(error: unknown): DraftFailure | null {
	if (!(error instanceof ApiError) || error.status !== 422) return null;
	const message = error.errors[0]?.message;
	return FAILURES.find((f) => f === message) ?? null;
}

/** Redeems a draft's photo link; the result goes into the editor's queue. */
export async function fetchDraftPhoto(href: string, signal?: AbortSignal): Promise<File> {
	const response = await fetch(href, { credentials: 'same-origin', signal });
	if (!response.ok) throw new Error(`photo ${response.status}`);
	const blob = await response.blob();
	// The service only hands out JPEG, PNG and WebP.
	if (!blob.type.startsWith('image/')) throw new Error(`photo type ${blob.type}`);
	return new File([blob], `import.${blob.type.split('/')[1]}`, { type: blob.type });
}
