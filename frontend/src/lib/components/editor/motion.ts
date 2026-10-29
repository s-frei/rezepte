// How the editor's lists open and close around a row the author added or
// took out. Only the author's own action moves anything: a row that arrives
// with a loaded recipe or an import, or one svelte-dnd-action inserts as the
// shadow of a drag, is simply there.
import { prefersReducedMotion } from 'svelte/motion';
import { cubicOut } from 'svelte/easing';
import type { TransitionConfig } from 'svelte/transition';

/**
 * Ids of the rows, steps, groups and photo tiles the author just added, for
 * the one update that renders them: only those move in. The add functions
 * put an id in and take it out again once the update has rendered (a photo
 * tile's intro takes it itself), so a later remount - a drag into another
 * group, svelte-dnd-action's shadow - is simply there. While one is in, the
 * lists do not flip: Svelte measures the flip with the new row at full
 * height, so the rows below an inserted one slid a row too far, over the row
 * above, while the reveal was already pushing them down. Ids are unique
 * across the editor (`newId`), so one set serves every list. A module-level
 * `Set` rather than a `SvelteSet`: nothing renders from it, the transitions
 * read it when they start.
 */
export const fresh = new Set<string>();

/**
 * The gap a `space-y-*` list gives the element before `node` because `node`
 * is the last one: that sibling's bottom margin only exists while something
 * follows it, so it arrives and leaves with `node` and has to open and close
 * with it, or the list jumps by the gap at one end of the motion.
 */
function trailingGap(node: Element): number {
	const previous = node.previousElementSibling;
	if (node.nextElementSibling || !previous) return 0;
	return parseFloat(getComputedStyle(previous).marginBlockEnd) || 0;
}

/**
 * Opens a list item from nothing to its height, fading in, so the items
 * below make room instead of jumping. `fresh` says the author added it
 * (the callers keep a set of the ids their add functions just made, for the
 * update that renders them); anything else enters at once.
 */
export function reveal(node: HTMLElement, { fresh = false } = {}): TransitionConfig {
	if (!fresh || prefersReducedMotion.current) return { duration: 0 };
	const end = parseFloat(getComputedStyle(node).marginBlockEnd) || 0;
	const gap = trailingGap(node);
	const { style } = node;
	// The height is read every frame (`scrollHeight`, what the content needs)
	// rather than measured once up front: a step's editor mounts, and its link
	// line appears with the focus, after the reveal has started, and a height
	// taken at the start cut both off and then jumped by them at the end.
	// `overflow: clip`, not `hidden`: a hidden box is a scroll container, and
	// focusing the field inside it scrolled the step's top row out of sight.
	return {
		duration: 180,
		easing: cubicOut,
		css: (t) => `opacity: ${t};`,
		tick: (t) => {
			if (t === 1) {
				for (const p of ['overflow', 'height', 'margin-block-end', 'margin-block-start']) {
					style.removeProperty(p);
				}
				return;
			}
			style.overflow = 'clip';
			style.height = `${t * node.scrollHeight}px`;
			style.marginBlockEnd = `${t * end}px`;
			style.marginBlockStart = `${(t - 1) * gap}px`;
		}
	};
}

/**
 * Folds an item shut before the caller takes it out of its list. Not an
 * `out:` transition: under `animate:flip` Svelte pins a leaving item
 * `position: absolute` where it stood, so the rows below slid up through the
 * fading one. Closed first and removed after, the rows below follow its
 * height and the flip that comes after has nothing left to move. `fill:
 * forwards` holds it shut until Svelte takes it away.
 */
export function collapse(node: Element | null | undefined): Promise<void> {
	if (!(node instanceof HTMLElement) || prefersReducedMotion.current) return Promise.resolve();
	const style = getComputedStyle(node);
	node.style.overflow = 'clip';
	return node
		.animate(
			[
				{
					height: `${node.offsetHeight}px`,
					marginBlockEnd: style.marginBlockEnd,
					marginBlockStart: style.marginBlockStart,
					opacity: 1
				},
				{
					height: '0px',
					marginBlockEnd: '0px',
					marginBlockStart: `${-trailingGap(node)}px`,
					opacity: 0
				}
			],
			{ duration: 140, easing: 'cubic-bezier(0.4, 0, 0.2, 1)', fill: 'forwards' }
		)
		.finished.then(
			() => {},
			// Cancelled (the element went first): removing it is still right.
			() => {}
		);
}
