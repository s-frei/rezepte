/**
 * Which section the editor's section nav marks as current.
 *
 * `inBand` holds every section currently inside the band just under the top
 * bar - the whole set, not only the entries of the latest observer callback,
 * which carry just the sections whose visibility changed. The topmost of
 * them wins, and the document order is the on-screen order. Once the page
 * is scrolled to its end the last section wins instead: it is short, so the
 * page runs out before it can ever reach the band. With nothing in the band
 * (the gap between two sections) the nav stays where it is.
 */
export function currentSection(
	order: readonly string[],
	inBand: ReadonlySet<string>,
	atBottom: boolean,
	current: string
): string {
	if (atBottom && order.length > 0) {
		return order[order.length - 1];
	}
	return order.find((id) => inBand.has(id)) ?? current;
}

/**
 * Whether the window is scrolled to the end of the page. The slack absorbs
 * the fractional scroll offsets zoomed or high-density screens report; a
 * page too short to scroll never counts, as nothing was scrolled to.
 */
export function isAtBottom(view: { innerHeight: number; scrollY: number; scrollHeight: number }) {
	return view.scrollY > 0 && view.innerHeight + view.scrollY >= view.scrollHeight - 2;
}

/**
 * The sections in the band after an observer callback. A callback reports
 * only the sections whose visibility changed, so the band is carried from
 * one callback to the next and each entry adds or drops its section.
 * Returns a fresh set rather than mutating the one it was given.
 */
export function nextBand(
	band: ReadonlySet<string>,
	entries: readonly { id: string; isIntersecting: boolean }[]
): ReadonlySet<string> {
	const next = new Set(band);
	for (const entry of entries) {
		if (entry.isIntersecting) {
			next.add(entry.id);
		} else {
			next.delete(entry.id);
		}
	}
	return next;
}

/**
 * Whether a section picked from the nav still holds the highlight. Once the
 * scroll it started has settled, the window's offset at that moment is
 * remembered, and the pick stands until the window has moved away from it.
 * The observer can still report after the scroll has ended, and a short
 * recipe scrolled to one of its lower sections sits at the end of the page,
 * where the last section would otherwise take the highlight straight back.
 * Only the reader scrolling again hands the nav back to the scroll position.
 */
export function stillPinned(pinnedAt: number | null, scrollY: number): boolean {
	return pinnedAt !== null && Math.abs(scrollY - pinnedAt) < 4;
}
