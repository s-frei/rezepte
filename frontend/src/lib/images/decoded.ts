/**
 * Resolves once the picture at `url` is loaded and decoded, or failed to, so
 * a swap can wait for it: an `<img>` shown before that paints nothing for a
 * moment, and a crossfade into it fades through an empty frame. The browser
 * keeps the decoded picture, so the `<img>` that follows paints at once.
 */
export function decoded(url: string): Promise<void> {
	const img = new Image();
	img.src = url;
	return img.decode().catch(() => {});
}
