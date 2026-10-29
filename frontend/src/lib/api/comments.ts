import type { Image, ImageVariant, Person } from './recipes';
import { api } from './client';

/** Most photos one comment can carry. */
export const MAX_COMMENT_IMAGES = 4;

/** A photo on a comment; never a recipe image unless `inRecipe`. */
export type CommentImage = { id: string; width: number; height: number; inRecipe: boolean };

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
	images: CommentImage[];
};

const recipePath = (recipeId: string) => `/recipes/${encodeURIComponent(recipeId)}/comments`;

/** A recipe's diary, oldest first. Does not mark anything seen. */
export async function listComments(recipeId: string): Promise<Comment[]> {
	const { comments } = await api<{ comments: Comment[] }>(
		`/comments?recipeId=${encodeURIComponent(recipeId)}`
	);
	return comments;
}

/** `imageIds` is the full set of staged photos to attach; empty or left out attaches none. */
export function addComment(recipeId: string, body: string, imageIds?: string[]): Promise<Comment> {
	return api<Comment>(recipePath(recipeId), {
		method: 'POST',
		body: JSON.stringify({ body, imageIds })
	});
}

/** Omitting `imageIds` keeps the photos; passing it replaces the whole set. */
export function editComment(id: number, body: string, imageIds?: string[]): Promise<Comment> {
	return api<Comment>(`/comments/${id}`, {
		method: 'PATCH',
		body: JSON.stringify({ body, imageIds })
	});
}

/** Stages one photo for the member's next comment. */
export function uploadCommentImage(recipeId: string, file: File): Promise<CommentImage> {
	const body = new FormData();
	body.append('file', file, file.name);
	return api<CommentImage>(`/recipes/${encodeURIComponent(recipeId)}/comment-images`, {
		method: 'POST',
		body
	});
}

/** Like `imageUrl`, for a comment photo; same cookie and caching rules. */
export function commentImageUrl(recipeId: string, imageId: string, variant: ImageVariant): string {
	return `/images/${encodeURIComponent(recipeId)}/comments/${encodeURIComponent(imageId)}/${variant}.jpg`;
}

/** Copies a comment photo into the recipe's photos; 409 if it is already there. */
export function addCommentImageToRecipe(recipeId: string, commentImageId: string): Promise<Image> {
	return api<Image>(`/recipes/${encodeURIComponent(recipeId)}/images/from-comment`, {
		method: 'POST',
		body: JSON.stringify({ commentImageId })
	});
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
