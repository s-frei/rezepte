import { ApiError } from '$lib/api/client';
import type { EditPolicy, Recipe } from '$lib/api/recipes';
import { m } from '$lib/paraglide/messages';

/**
 * The colophon line for a locked recipe, or null when it is open. The
 * author reads "you"; everybody else reads the author's name.
 */
export function lockNotice(
	recipe: Pick<Recipe, 'locked' | 'createdBy'>,
	meId: string | undefined
): string | null {
	if (!recipe.locked) {
		return null;
	}
	return recipe.createdBy.id === meId
		? m.detail_locked_self()
		: m.detail_locked_other({ author: recipe.createdBy.displayName });
}

/**
 * The line under the editor's policy control. `lockedByDefault` is the
 * household setting, null while unknown. Like `lockNotice`, the author reads
 * "you" and everybody else (an admin editing someone else's recipe) reads
 * the author's name.
 */
export function policyHint(
	policy: EditPolicy,
	lockedByDefault: boolean | null,
	author: { isAuthor: boolean; name: string }
): string {
	const self = author.isAuthor;
	const named = { author: author.name };
	if (policy === 'open') {
		return self ? m.editor_policy_hint_open() : m.editor_policy_hint_open_other(named);
	}
	if (policy === 'locked') {
		return self ? m.editor_policy_hint_locked() : m.editor_policy_hint_locked_other(named);
	}
	if (lockedByDefault === null) {
		return m.editor_policy_hint_default_unknown();
	}
	if (!lockedByDefault) {
		return m.editor_policy_hint_default_open();
	}
	return self
		? m.editor_policy_hint_default_locked()
		: m.editor_policy_hint_default_locked_other(named);
}

/**
 * The toast for a failed write from the editor. A 403 means the right to
 * edit went away while the editor was open - the author locked the recipe,
 * or the owner locked the household - so it says that instead of the
 * operation's generic failure.
 */
export function refusalOr(error: unknown, fallback: string): string {
	return error instanceof ApiError && error.status === 403 ? m.editor_forbidden() : fallback;
}
