import { uploadCommentImage, type CommentImage } from '#lib/api/comments.js';

/**
 * One photo in a comment's composer or editor. A picked file carries `file`
 * and an object URL in `preview` until it is staged on Send; a photo the
 * comment already has carries only `staged` and its thumbnail URL.
 * `progress` is its upload's fraction, 0 to 1; 1 once it is up.
 */
export type PendingPhoto = {
	key: string;
	file?: File;
	preview: string;
	width: number;
	height: number;
	progress: number;
	staged?: CommentImage;
	failed?: boolean;
};

/**
 * Uploads every photo not yet staged, one after another, and returns the ids
 * of all of them in order. A photo that went up keeps `staged`, so after a
 * failure (which marks the photo `failed` and rethrows) a retry uploads only
 * the rest. `onprogress` counts the uploads, not the photos; each photo's
 * own `progress` follows its upload.
 */
export async function stagePhotos(
	recipeId: string,
	photos: PendingPhoto[],
	onprogress: (n: number, of: number) => void
): Promise<string[]> {
	const todo = photos.filter((photo) => !photo.staged);
	for (const [i, photo] of todo.entries()) {
		onprogress(i + 1, todo.length);
		photo.failed = false;
		photo.progress = 0;
		try {
			photo.staged = await uploadCommentImage(
				recipeId,
				photo.file as File,
				(f) => (photo.progress = f)
			);
			photo.progress = 1;
		} catch (error) {
			photo.failed = true;
			throw error;
		}
	}
	return photos.map((photo) => (photo.staged as CommentImage).id);
}

/** Frees the object URLs of picked files; existing photos have none. */
export function revokePreviews(photos: PendingPhoto[]) {
	for (const photo of photos) if (photo.file) URL.revokeObjectURL(photo.preview);
}

/**
 * After a 422 from saving a comment, one of the staged ids is gone (evicted
 * by a newer upload, or swept): every picked file goes up again on the next
 * Send. Photos the comment already had carry no file and stay.
 */
export function unstage(photos: PendingPhoto[]) {
	for (const photo of photos) {
		if (!photo.file) continue;
		photo.staged = undefined;
		photo.failed = false;
		photo.progress = 0;
	}
}
