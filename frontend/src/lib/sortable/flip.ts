// FLIP for the sortable's items: every item that changes place or size
// slides there from where it was, so a move is visible even among photos
// that look alike.
import { tick } from 'svelte';
import { prefersReducedMotion } from 'svelte/motion';

export const EASE = 'cubic-bezier(0.2, 0, 0, 1)';

/** The slide each item is running, so a new move starts from where it is now. */
const running = new WeakMap<HTMLElement, Animation>();

/**
 * Runs `change` (the caller updates its list) and slides every
 * `[data-sortable-key]` child of `container` from its old box into its new
 * one, translating and scaling. A held item (`data-lifted`) follows the
 * pointer instead. An item whose size changes carries `data-resizing` while
 * it slides, so the caller can hide what would stretch with it (a ×).
 * `settle` slides over the others and gets the settle ring once it lands.
 * Under reduced motion the moved items crossfade and nothing rings.
 */
export async function flip(
	container: HTMLElement,
	change: () => void,
	{ duration = 220, settle = null }: { duration?: number; settle?: HTMLElement | null } = {}
): Promise<void> {
	const items = () => [...container.querySelectorAll<HTMLElement>('[data-sortable-key]')];
	const first = new Map(items().map((el) => [el.dataset.sortableKey, el.getBoundingClientRect()]));
	change();
	await tick();
	const reduce = prefersReducedMotion.current;
	for (const el of items()) {
		const a = first.get(el.dataset.sortableKey);
		if (!a || el.hasAttribute('data-lifted')) continue;
		running.get(el)?.cancel();
		const b = el.getBoundingClientRect();
		const dx = a.left - b.left;
		const dy = a.top - b.top;
		const sx = a.width / b.width;
		const sy = a.height / b.height;
		if (
			Math.abs(dx) < 0.5 &&
			Math.abs(dy) < 0.5 &&
			Math.abs(sx - 1) < 0.01 &&
			Math.abs(sy - 1) < 0.01
		)
			continue;
		let anim: Animation;
		if (reduce) {
			anim = el.animate([{ opacity: 0.2 }, { opacity: 1 }], { duration: 160 });
		} else {
			if (Math.abs(sx - 1) > 0.02 || Math.abs(sy - 1) > 0.02) el.setAttribute('data-resizing', '');
			if (el === settle) el.style.zIndex = '1';
			anim = el.animate(
				[
					{ transformOrigin: '0 0', transform: `translate(${dx}px, ${dy}px) scale(${sx}, ${sy})` },
					{ transformOrigin: '0 0', transform: 'none' }
				],
				{ duration, easing: EASE }
			);
		}
		running.set(el, anim);
		anim.finished
			.catch(() => {})
			.then(() => {
				if (running.get(el) !== anim) return;
				running.delete(el);
				el.removeAttribute('data-resizing');
				if (el === settle) el.style.zIndex = '';
			});
	}
	if (settle) ring(settle, duration);
}

/**
 * The settle ring: a `primary` ring around `el` that holds, then fades over
 * 700ms, after `delay` (the slide into place). None under reduced motion.
 */
export function ring(el: HTMLElement, delay = 0): void {
	if (prefersReducedMotion.current) return;
	const color = getComputedStyle(el).getPropertyValue('--color-primary').trim();
	el.animate(
		[
			{ boxShadow: `0 0 0 3px ${color}` },
			{ boxShadow: `0 0 0 3px ${color}`, offset: 0.4 },
			{ boxShadow: '0 0 0 3px transparent' }
		],
		{ duration: 700, delay, easing: 'ease-out' }
	);
}
