import { toast } from 'svelte-sonner';
import { ApiError } from '$lib/api/client';
import { ACCEPTED_IMAGE_TYPES, MAX_IMAGE_BYTES } from '$lib/api/recipes';
import { m } from '$lib/paraglide/messages';
import { refusalOr } from '$lib/recipe/access';

/**
 * Keeps the picked files that may be uploaded and toasts once per reason for
 * the rest. The server would answer 413/415 anyway; refusing here saves
 * pushing megabytes over the wire first. At most `room` files are kept; the
 * rest get `limitMessage`.
 */
export function acceptFiles(files: File[], room: number, limitMessage: string): File[] {
	const typed = files.filter((f) => ACCEPTED_IMAGE_TYPES.includes(f.type));
	const accepted = typed.filter((f) => f.size <= MAX_IMAGE_BYTES);
	if (typed.length < files.length) toast.error(m.images_rejected_type());
	if (accepted.length < typed.length) toast.error(m.images_rejected_size());
	if (accepted.length > room) toast.error(limitMessage);
	return accepted.slice(0, Math.max(0, room));
}

/**
 * What went wrong with an upload, in the user's terms. 413 and 415 are the
 * rejections the pre-checks can still miss - a file whose reported type is
 * one thing and whose bytes are another, or a body that grew past the cap
 * on the way - so they name the actual limit instead of failing vaguely.
 */
export function uploadErrorMessage(error: unknown): string {
	if (error instanceof ApiError) {
		if (error.status === 413) {
			return m.images_rejected_size();
		}
		if (error.status === 415) {
			return m.images_rejected_type();
		}
	}
	return refusalOr(error, m.images_upload_error());
}
