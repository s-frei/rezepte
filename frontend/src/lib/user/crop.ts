/** A square picked in the crop dialog, as fractions of the upright image:
 * `x` and `y` place its top-left corner against width and height, `size`
 * is its side against the shorter side - what `?crop=` sends. */
export type Crop = { x: number; y: number; size: number };

/**
 * Where the image sits in the crop frame: `zoom` 1 makes its shorter side
 * fill the frame, and `x`, `y` are its top-left corner in frame pixels -
 * zero or negative, since the image always covers the frame.
 */
export type View = { zoom: number; x: number; y: number };

export const MIN_ZOOM = 1;
export const MAX_ZOOM = 4;

/** Frame pixels per image pixel. */
export function scaleOf(w: number, h: number, frame: number, zoom: number): number {
	return (zoom * frame) / Math.min(w, h);
}

/** The image as it first appears: zoom 1, centered. */
export function centered(w: number, h: number, frame: number): View {
	const s = scaleOf(w, h, frame, 1);
	return { zoom: 1, x: (frame - w * s) / 2, y: (frame - h * s) / 2 };
}

/** `view` with the zoom bounded and the image pushed back over the frame. */
export function clampView(w: number, h: number, frame: number, view: View): View {
	const zoom = Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, view.zoom));
	const s = scaleOf(w, h, frame, zoom);
	const clamp = (v: number, size: number) => Math.min(0, Math.max(frame - size * s, v));
	return { zoom, x: clamp(view.x, w), y: clamp(view.y, h) };
}

/** Zooms to `zoom` keeping the image point under frame point (px, py) in place. */
export function zoomAt(
	w: number,
	h: number,
	frame: number,
	view: View,
	zoom: number,
	px: number,
	py: number
): View {
	const before = scaleOf(w, h, frame, view.zoom);
	const bounded = Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, zoom));
	const after = scaleOf(w, h, frame, bounded);
	const ix = (px - view.x) / before;
	const iy = (py - view.y) / before;
	return clampView(w, h, frame, { zoom: bounded, x: px - ix * after, y: py - iy * after });
}

/** The square the frame shows, as the fractions `?crop=` sends. */
export function toCrop(w: number, h: number, frame: number, view: View): Crop {
	const s = scaleOf(w, h, frame, view.zoom);
	// `0 - v` rather than `-v`, so an unmoved image gives 0 and not -0.
	return { x: (0 - view.x) / s / w, y: (0 - view.y) / s / h, size: 1 / view.zoom };
}
