import type { Crop } from './crop';

/**
 * URL of a person's picture, or null without one. Served by a plain file
 * route outside /api/v1 (session cookie required) with a one-year immutable
 * cache: every upload gets a new id, so a changed picture is a changed URL.
 */
export function avatarUrl(person: { id: string; avatarId?: string | null }): string | null {
	if (!person.avatarId) return null;
	return `/avatars/${encodeURIComponent(person.id)}/${encodeURIComponent(person.avatarId)}.jpg`;
}

/** The `?crop=` parameter. Four decimals are a tenth of a pixel on a
 * 2400 px image, well inside the half pixel the service tolerates. */
export function cropQuery(crop: Crop): string {
	return `?crop=${[crop.x, crop.y, crop.size].map((n) => n.toFixed(4)).join(',')}`;
}

/** The multipart body both upload operations take. */
export function avatarBody(file: File): FormData {
	const body = new FormData();
	body.append('file', file, file.name);
	return body;
}
