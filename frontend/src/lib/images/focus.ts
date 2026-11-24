import type { Attachment } from 'svelte/attachments';

/** A point of the upright photo, as fractions: 0 is the left or top edge, 1 the right or bottom. */
export type ImageFocus = { x: number; y: number };

/** A part of the photo, as fractions of its width and height. */
export type Rect = { x: number; y: number; w: number; h: number };

export const CENTER: ImageFocus = { x: 0.5, y: 0.5 };

/** Whether `focus` is the default, give or take a rounding step. */
export function isCentered(focus: ImageFocus): boolean {
	return Math.abs(focus.x - 0.5) < 0.005 && Math.abs(focus.y - 0.5) < 0.005;
}

/** Whether two points are the same one, apart from float noise. */
export function sameFocus(a: ImageFocus, b: ImageFocus): boolean {
	return Math.abs(a.x - b.x) < 0.0005 && Math.abs(a.y - b.y) < 0.0005;
}

/** The recipe card keeps the photo's own shape between 4:5 portrait and 16:9 landscape. */
export const CARD_MIN = 4 / 5,
	CARD_MAX = 16 / 9;

const clamp = (v: number) => Math.min(1, Math.max(0, v));

/** The ratio a recipe card gives a photo of `imageRatio`. */
export function cardRatio(imageRatio: number): number {
	return Math.min(Math.max(imageRatio, CARD_MIN), CARD_MAX);
}

/**
 * The `object-position` that puts `focus` at the center of a `boxRatio` box as
 * far as the edges allow. A photo is cut on one axis only; there the visible
 * fraction `v` leaves `1 - v` to slide over.
 */
export function focusPosition(
	focus: ImageFocus | null | undefined,
	imageRatio: number,
	boxRatio: number
): string {
	if (!focus) return '50% 50%';
	if (imageRatio > boxRatio) {
		const v = boxRatio / imageRatio;
		return `${clamp((focus.x - v / 2) / (1 - v)) * 100}% 50%`;
	}
	if (imageRatio < boxRatio) {
		const v = imageRatio / boxRatio;
		return `50% ${clamp((focus.y - v / 2) / (1 - v)) * 100}%`;
	}
	return '50% 50%';
}

/** What a `boxRatio` box shows of the photo around `focus`. */
export function cropRect(focus: ImageFocus, imageRatio: number, boxRatio: number): Rect {
	if (imageRatio > boxRatio) {
		const w = boxRatio / imageRatio;
		return { x: Math.min(1 - w, Math.max(0, focus.x - w / 2)), y: 0, w, h: 1 };
	}
	const h = imageRatio / boxRatio;
	return { x: 0, y: Math.min(1 - h, Math.max(0, focus.y - h / 2)), w: 1, h };
}

/** The part of the photo every one of `ratios` shows. */
export function alwaysVisible(focus: ImageFocus, imageRatio: number, ratios: number[]): Rect {
	const rects = ratios.map((r) => cropRect(focus, imageRatio, r));
	const x = Math.max(...rects.map((r) => r.x));
	const y = Math.max(...rects.map((r) => r.y));
	const x1 = Math.min(...rects.map((r) => r.x + r.w));
	const y1 = Math.min(...rects.map((r) => r.y + r.h));
	return { x, y, w: Math.max(0, x1 - x), h: Math.max(0, y1 - y) };
}

/**
 * The middle of what every shape shows around `focus`. Without zoom a photo
 * is cut on one axis per shape, so a point placed anywhere moves only where
 * moving changes something.
 */
export function settle(focus: ImageFocus, imageRatio: number, ratios: number[]): ImageFocus {
	const r = alwaysVisible(focus, imageRatio, ratios);
	return { x: r.x + r.w / 2, y: r.y + r.h / 2 };
}

/**
 * Keeps an `<img class="object-cover">` on its photo's focus in whatever box
 * it gets, measuring the box as it resizes. Leaves the image centered
 * without the photo's size.
 */
export function objectFocus(
	get: () => { focus?: ImageFocus | null; width?: number | null; height?: number | null }
): Attachment<HTMLImageElement> {
	return (img) => {
		const { focus, width, height } = get();
		if (!width || !height) {
			img.style.objectPosition = '';
			return;
		}
		const place = () => {
			if (!img.clientWidth || !img.clientHeight) return;
			img.style.objectPosition = focusPosition(
				focus,
				width / height,
				img.clientWidth / img.clientHeight
			);
		};
		place();
		const observer = new ResizeObserver(place);
		observer.observe(img);
		return () => observer.disconnect();
	};
}
