import { cubicOut } from 'svelte/easing';
import { prefersReducedMotion } from 'svelte/motion';
import { slide } from 'svelte/transition';

/**
 * The overview notes' `out:` transition: while `folding()` holds (the note's
 * × was pressed) it folds away and fades and the overview closes up under it;
 * otherwise, or with reduced motion, it goes at once. Notes never arrive with
 * motion - they are there on load or not at all.
 */
export function collapse(node: Element, folding: () => boolean) {
	const duration = folding() && !prefersReducedMotion.current ? 240 : 0;
	const fold = slide(node, { duration, easing: cubicOut });
	return { ...fold, css: (t: number, u: number) => `${fold.css?.(t, u)}; opacity: ${t}` };
}
