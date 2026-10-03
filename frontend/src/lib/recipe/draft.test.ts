import { expect, it } from 'vitest';
import type { RecipeDraft } from '$lib/api/drafts';
import { putDraft, takeDraft } from './draft';

const imported = () => ({ draft: {} as RecipeDraft, photo: null, photoFailed: true });

it('hands a draft over exactly once', () => {
	expect(takeDraft()).toBeNull();
	const draft = imported();
	putDraft(draft);
	expect(takeDraft()).toBe(draft);
	expect(takeDraft()).toBeNull();
});
