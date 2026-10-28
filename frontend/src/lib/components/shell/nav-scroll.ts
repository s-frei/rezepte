/**
 * Pixels from the top in which the phone's bottom nav floats; past them it
 * docks to the bottom edge. Near the top a page is still being looked over,
 * so the bar hovers like a control; once reading starts it settles out of
 * the way of the text, and it never shrinks.
 */
export const DOCK_AT = 120;

/** Whether the bar is docked at scroll position `y`. */
export function docked(y: number): boolean {
	return y > DOCK_AT;
}
