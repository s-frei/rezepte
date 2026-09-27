/** The two shapes of the phone's bottom nav. */
export type NavMode = 'expanded' | 'minimized';

/**
 * The bar's shape plus what it measures scrolling against: `anchor` is where
 * the current movement started, `lastY` the position of the previous event.
 */
export type NavScroll = { mode: NavMode; anchor: number; lastY: number };

/** Pixels from the top in which the bar always shows in full. */
const TOP = 120;
/** Pixels from the end at which the bar comes back in full. */
const END = 24;
/** Distance one movement has to cover to change the shape; a resting finger's jitter stays under it. */
const STEP = 8;

/** A page just arrived at: the whole bar, measuring from where the page is. */
export function startNavScroll(y: number): NavScroll {
	return { mode: 'expanded', anchor: y, lastY: y };
}

/**
 * The bar's next state after a scroll to `y` on a page that scrolls to
 * `maxY`. Reading down shrinks it; any intent to go somewhere else - scrolling
 * back up, reaching the top or the end - brings it back. A page that does not
 * scroll never shrinks it, since there is nothing under the bar to reveal.
 *
 * The distance is measured over a whole movement, not per scroll event: a
 * reading-speed scroll moves a few pixels per frame and still has to count.
 * A movement starts where the direction last turned, so jitter back and
 * forth never adds up.
 */
export function navScroll(state: NavScroll, y: number, maxY: number): NavScroll {
	if (maxY <= 0 || y < TOP || y >= maxY - END) return startNavScroll(y);
	const direction = Math.sign(y - state.lastY);
	const previous = Math.sign(state.lastY - state.anchor);
	const anchor =
		direction !== 0 && previous !== 0 && direction !== previous ? state.lastY : state.anchor;
	const distance = y - anchor;
	if (distance >= STEP) return { mode: 'minimized', anchor: y, lastY: y };
	if (distance <= -STEP) return { mode: 'expanded', anchor: y, lastY: y };
	return { mode: state.mode, anchor, lastY: y };
}
