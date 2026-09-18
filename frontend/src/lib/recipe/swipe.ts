/** Minimum horizontal travel (px) for a pointer gesture to count as a swipe (design handoff). */
export const SWIPE_THRESHOLD = 50;

export type SwipeDirection = 'next' | 'previous' | null;

/**
 * Turns the start and end x of a pointer gesture into a paging direction:
 * dragging left (content moves left, like turning a page forward) is
 * `next`, dragging right is `previous`, shorter moves are `null`.
 */
export function swipeDirection(
	startX: number,
	endX: number,
	threshold: number = SWIPE_THRESHOLD
): SwipeDirection {
	const delta = endX - startX;
	if (delta <= -threshold) {
		return 'next';
	}
	if (delta >= threshold) {
		return 'previous';
	}
	return null;
}
