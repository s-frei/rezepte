import { untrack } from 'svelte';

/**
 * A row of the last answer while the next is out: out of reach at once,
 * dimmed only after 300ms - past the 150ms debounce and a phone's round
 * trip - so a normal answer lands before any dim starts and the dim reads
 * as loading. With `transition-opacity` on the row; a row that fades in
 * takes `animation-fill-mode: backwards`, or the fade's fill holds its
 * opacity and the dim snaps.
 */
export const STALE_ROW = 'pointer-events-none opacity-60 delay-300 duration-200';

/** How long a list must stay shorter before its box follows: longer than a keystroke's gap. */
const SETTLE_MS = 180;
/** How long held rows that stopped matching keep their room while an answer is out: past a typo taken back after its answer. */
const DROP_MS = 400;

/**
 * An attachment for the box around a live-results list (its only child),
 * which keeps the box still while someone types:
 * - It grows with the list right away (its 200ms transition), unless an
 *   answer lands for a `text` shorter than when the box last moved:
 *   deleting, an answer for a shorter prefix would bloom it just before the
 *   next key empties it, so that waits as a shrink does. Rows growing back
 *   while an answer is out (the typed row, held rows matching again) show
 *   at once: the row Enter takes is never clipped.
 * - It shrinks once the list has kept its smaller height for `SETTLE_MS`
 *   with no search `pending` (a keystroke whose answer is not in), so a
 *   search answering 6, 1, 6 names does not jump.
 * - While an answer is pending only held rows that stopped matching leave:
 *   the box follows them after `DROP_MS` whatever is pending, instead of
 *   keeping a blank band until a slow answer lands; a typo taken back
 *   within that time moves nothing.
 * `pending` and `text` are read when needed, never tracked. The caller's
 * classes animate `height` (`transition-[height]`, off under reduced
 * motion, which keeps the delays) and clip the overflow on y only, so rows
 * bleeding sideways (`-mx-2`) keep their highlight.
 */
export const settleHeight = (pending: () => boolean, text: () => string) => (box: HTMLElement) => {
	const list = box.firstElementChild as HTMLElement;
	let shown = 0;
	/** The text's length when the box last moved. */
	let at = 0;
	let timer: ReturnType<typeof setTimeout> | undefined;
	const show = (px: number) => {
		shown = px;
		at = text().length;
		box.style.height = `${px}px`;
	};
	// Untracked: a read here would re-run the attachment on every keystroke.
	untrack(() => show(list.offsetHeight));
	const settle = () => {
		if (pending()) timer = setTimeout(settle, SETTLE_MS);
		else show(list.offsetHeight);
	};
	const observer = new ResizeObserver(() => {
		clearTimeout(timer);
		const height = list.offsetHeight;
		if (height === shown) return;
		if (height > shown && (text().length >= at || pending())) show(height);
		else if (height < shown && pending())
			timer = setTimeout(() => show(list.offsetHeight), DROP_MS);
		else timer = setTimeout(settle, SETTLE_MS);
	});
	observer.observe(list);
	return () => {
		observer.disconnect();
		clearTimeout(timer);
	};
};
