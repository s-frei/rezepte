import type { StepPhoto } from '#lib/api/recipes.js';
import type { FormPhoto, FormStep } from './form';
import { firstWordMatch } from './references';
import { overlapsAny } from './step-times';

/**
 * One photo the step editor may link to, as `ImagesSection` shows it: stored,
 * queued for a new recipe, or still uploading. `key` is the tile's key, which
 * a link holds and which survives the upload; `id` is the server's image id,
 * null until the photo is stored.
 */
export type PhotoTile = { key: string; id: string | null; thumb: string; detail: string };

/** A photo queued for a recipe not created yet, with the key its tile and its links go by. */
export type QueuedPhoto = { key: string; file: File };

/** The step's photo links whose phrase the text still holds. */
export function presentPhotos(photos: FormPhoto[], text: string): FormPhoto[] {
	return photos.filter((photo) => firstWordMatch(text, photo.phrase) !== null);
}

/**
 * Links `photo`, replacing the link on the same phrase, and keeps the links in
 * the order their phrases stand in `text`, which is how the list reads them.
 */
export function addPhoto(photos: FormPhoto[], photo: FormPhoto, text: string): FormPhoto[] {
	const at = (item: FormPhoto) => firstWordMatch(text, item.phrase)?.from ?? Infinity;
	return [...photos.filter((existing) => existing.phrase !== photo.phrase), photo].sort(
		(a, b) => at(a) - at(b)
	);
}

export function removePhoto(photos: FormPhoto[], phrase: string): FormPhoto[] {
	return photos.filter((photo) => photo.phrase !== phrase);
}

/**
 * What keeps `phrase` from carrying a photo: a stored time it overlaps (a time
 * chip would sit inside the photo's trigger), or another photo link. Its own
 * link does not count, so "Change" picks again, and a linked ingredient word
 * inside it is fine - the quantity still shows in the phrase.
 */
export function photoBlocker(step: FormStep, phrase: string): 'time' | 'photo' | null {
	if (
		overlapsAny(
			step.text,
			phrase,
			step.times.map((time) => time.phrase)
		)
	) {
		return 'time';
	}
	const others = step.photos.filter((photo) => photo.phrase !== phrase);
	return overlapsAny(
		step.text,
		phrase,
		others.map((photo) => photo.phrase)
	)
		? 'photo'
		: null;
}

/**
 * The links whose photo is among the editor's tiles. A photo removed while
 * its undo toast is up, or whose upload failed, is no tile: its links stay in
 * the form, so an undo brings them back, but nothing shows or sends them.
 */
export function visiblePhotos(photos: FormPhoto[], tiles: PhotoTile[]): FormPhoto[] {
	return photos.filter((photo) => tiles.some((tile) => tile.key === photo.imageKey));
}

/**
 * `step` with only the links that show: their phrase stands in the text and
 * their photo is among the tiles. Time proposals, "Mark as time" and "Link
 * photo" go around these.
 */
export function withShownPhotos(step: FormStep, tiles: PhotoTile[]): FormStep {
	return { ...step, photos: visiblePhotos(presentPhotos(step.photos, step.text), tiles) };
}

/**
 * What a save may send for each tile key: a stored photo's id, and for a new
 * recipe (`queued`) a queued photo's key, which its `save` swaps for the id
 * once uploaded. A photo of an existing recipe still uploading has none.
 */
export function photoIds(tiles: PhotoTile[], queued: boolean): Map<string, string> {
	return new Map(
		tiles.flatMap((tile): [string, string][] =>
			tile.id !== null ? [[tile.key, tile.id]] : queued ? [[tile.key, tile.key]] : []
		)
	);
}

/**
 * `step` with each link's `imageId` swapped through `ids` (a tile key to the
 * photo's id), a link whose key `ids` lacks left out. Without `ids` the key is
 * the id already. `photos` is left off when none is left, the way a client
 * unaware of photo links sends a step.
 */
export function resolvePhotos<S extends { photos?: StepPhoto[] }>(
	{ photos, ...step }: S,
	ids?: Map<string, string>
): Omit<S, 'photos'> & { photos?: StepPhoto[] } {
	const resolved = (photos ?? []).flatMap((photo) => {
		const imageId = ids ? ids.get(photo.imageId) : photo.imageId;
		return imageId === undefined ? [] : [{ phrase: photo.phrase, imageId }];
	});
	return resolved.length > 0 ? { ...step, photos: resolved } : step;
}
