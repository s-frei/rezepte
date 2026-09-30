import { domToBlob } from 'modern-screenshot';
import { tick } from 'svelte';

/**
 * The largest canvas the recipe card is drawn on. iOS Safari refuses a
 * canvas above 16,777,216 pixels and hands back an empty image instead of
 * an error; this leaves headroom below that.
 */
export const MAX_CANVAS_PIXELS = 16_000_000;

/** The longest side Chrome and Firefox draw a canvas at; past it they draw nothing. */
export const MAX_CANVAS_SIDE = 32_767;

/**
 * How many device pixels per CSS pixel the card is drawn at: 3 for a sharp
 * picture on a phone, lower for a tall recipe so the canvas stays under
 * `MAX_CANVAS_PIXELS` and its longest side under `MAX_CANVAS_SIDE`, never
 * below 1. Only sharpness changes - the card is
 * always laid out 420 CSS px wide, so its type keeps the same proportion to
 * the picture however far a messenger scales it down.
 */
export function pickScale(width: number, height: number): number {
	const fit = Math.min(
		Math.sqrt(MAX_CANVAS_PIXELS / (width * height)),
		MAX_CANVAS_SIDE / Math.max(width, height)
	);
	return Math.max(1, Math.min(3, Math.floor(fit * 100) / 100));
}

/**
 * Draws `node` - the mounted `ShareCard` - into a PNG. Waits for the web
 * fonts and every picture in it first: a font or photo still loading is
 * drawn as a fallback face or a blank box. A picture that fails lets the
 * card swap in its placeholder, hence the `tick()` before drawing.
 */
export async function renderShareImage(node: HTMLElement): Promise<Blob> {
	await document.fonts.ready;
	await Promise.all(
		[...node.querySelectorAll('img')].map((img) => img.decode().catch(() => undefined))
	);
	await tick();
	const { width, height } = node.getBoundingClientRect();
	return domToBlob(node, { scale: pickScale(width, height), type: 'image/png' });
}
