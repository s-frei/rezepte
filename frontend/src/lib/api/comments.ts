import type { Person } from './recipes';
import { api } from './client';

/** One comment on a recipe, as the signed-in member sees it. */
export type Comment = {
	id: number;
	/** Absent once the person's account was removed: a former member. */
	author?: Person;
	body: string;
	createdAt: string;
	editedAt: string | null;
	/** Written by somebody else since the member last opened the recipe's comments. */
	new: boolean;
	canEdit: boolean;
	canDelete: boolean;
};

const recipePath = (recipeId: string) => `/recipes/${encodeURIComponent(recipeId)}/comments`;

/** A recipe's diary, oldest first. Does not mark anything seen. */
export async function listComments(recipeId: string): Promise<Comment[]> {
	const { comments } = await api<{ comments: Comment[] }>(
		`/comments?recipeId=${encodeURIComponent(recipeId)}`
	);
	return comments;
}

export function addComment(recipeId: string, body: string): Promise<Comment> {
	return api<Comment>(recipePath(recipeId), { method: 'POST', body: JSON.stringify({ body }) });
}

export function editComment(id: number, body: string): Promise<Comment> {
	return api<Comment>(`/comments/${id}`, { method: 'PATCH', body: JSON.stringify({ body }) });
}

export async function deleteComment(id: number): Promise<void> {
	await api<void>(`/comments/${id}`, { method: 'DELETE' });
}

/**
 * Called by the recipe page once the diary is on screen, with the highest
 * entry id it shows; entries written since stay new. 0 marks nothing.
 */
export async function markCommentsSeen(recipeId: string, upTo: number): Promise<void> {
	await api<void>(`${recipePath(recipeId)}/seen`, {
		method: 'PUT',
		body: JSON.stringify({ upTo })
	});
}
