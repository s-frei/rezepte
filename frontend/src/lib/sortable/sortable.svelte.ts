// Reorders a list's `[data-sortable-key]` children by pointer and keyboard.
// A mouse drags after 4px; a finger lifts the item after a 250ms press (a
// move of more than 8px before that is a scroll and lets go). While held,
// the item stays in the list as the landing slot and its face follows the
// pointer at the size it was picked up at; the slot under the pointer is
// where it goes: each new slot calls `onmove`, which the caller runs inside
// `flip` so everything else slides. Alt+Arrow on a focused item moves it by
// one, or by a row up and down.
//
// The CSS contract a component using it adds:
// - on every item: `user-select: none; -webkit-touch-callout: none;
//   touch-action: pan-y` - without the touch-action the browser's pan cancels
//   the press before the long-press lifts it;
// - `[data-sortable-face]` inside the item, `position: absolute; inset: 0`
//   (the item is its containing block): it is what follows the pointer, with
//   its picked-up width and height set inline. Without a face the item itself
//   moves and no slot stays behind. Clipping (`overflow: hidden`) and rounding
//   go on the face, never on the item: the item would cut off the lifted face;
// - `[data-sortable-ignore]` on a control inside an item that has a press of
//   its own, such as a menu button or a ×: a press there never picks it up;
// - `[data-sortable-slot]` on the held item: style it as the landing slot, the
//   dashed `primary` gap;
// - `[data-resizing]` on an item while `flip` changes its size: hide what
//   would stretch with it, such as a ×.
// `[data-lifted]` marks the held item from the lift to the drop.
import { tick } from 'svelte';
import type { Action } from 'svelte/action';
import { prefersReducedMotion } from 'svelte/motion';
import { EASE, ring } from './flip.js';
import { keyTarget } from './order.js';

export type SortableOptions = {
	/** The items' keys in their current order, as their `data-sortable-key`. */
	keys: string[];
	/** An item that cannot be picked up or moved by key. */
	disabled?: (key: string) => boolean;
	/** Items per row, for Alt+ArrowUp and Alt+ArrowDown. */
	columns: () => number;
	/** Move the item at `from` to `to`; the caller updates its list inside `flip`. */
	onmove: (from: number, to: number) => void;
	/** A drag or key press has put `key` in a new place: save the order. */
	ondrop?: (key: string) => void;
	/** What the live region announces after a move; `position` counts from 1. */
	label: (key: string, position: number) => string;
};

const LIFT = 1.06;

type Held = {
	el: HTMLElement;
	/** What follows the pointer: the item's `[data-sortable-face]`, or the item. */
	face: HTMLElement;
	key: string;
	from: number;
	pointer: number;
	/** Where the press started, and where the pointer is. */
	x0: number;
	y0: number;
	x: number;
	y: number;
	/** The grab point inside the item, and its size when picked up. */
	gx: number;
	gy: number;
	w: number;
	h: number;
	lifted: boolean;
	timer?: ReturnType<typeof setTimeout>;
	liftAnim?: Animation;
	/** The item it last swapped with: skipped until the pointer leaves it, or the two would trade back. */
	over: string | null;
};

export const sortable: Action<HTMLElement, SortableOptions> = (node, initial) => {
	let opts = initial;
	let held: Held | null = null;

	const live = document.createElement('div');
	live.setAttribute('aria-live', 'polite');
	Object.assign(live.style, {
		position: 'absolute',
		width: '1px',
		height: '1px',
		overflow: 'hidden',
		clipPath: 'inset(50%)',
		whiteSpace: 'nowrap'
	});
	document.body.append(live);

	const itemAt = (target: EventTarget | null) => {
		const el =
			target instanceof Element ? target.closest<HTMLElement>('[data-sortable-key]') : null;
		return el && node.contains(el) ? el : null;
	};
	const scale = () => (prefersReducedMotion.current ? 1 : LIFT);

	/** Puts the held face under the pointer at its picked-up size, from its resting box. */
	function place(h: Held) {
		h.face.style.transform = '';
		const slot = h.face.getBoundingClientRect();
		const s = scale();
		h.face.style.transform =
			`translate(${h.x - h.gx * s - slot.left}px, ${h.y - h.gy * s - slot.top}px) ` +
			`scale(${(h.w * s) / slot.width}, ${(h.h * s) / slot.height})`;
	}

	function lift(h: Held, touch: boolean) {
		h.lifted = true;
		h.el.setAttribute('data-lifted', '');
		Object.assign(h.el.style, { zIndex: '10', pointerEvents: 'none' });
		Object.assign(h.face.style, { transformOrigin: '0 0', boxShadow: 'var(--shadow-lift)' });
		if (h.face !== h.el) {
			h.el.setAttribute('data-sortable-slot', '');
			// The face keeps its size even when its slot becomes the 2x2 cover.
			Object.assign(h.face.style, {
				width: `${h.w}px`,
				height: `${h.h}px`,
				right: 'auto',
				bottom: 'auto'
			});
		}
		place(h);
		if (!touch) return;
		navigator.vibrate?.(10);
		if (!prefersReducedMotion.current)
			h.liftAnim = h.face.animate(
				[{ transform: 'none', boxShadow: 'none' }, { transform: h.face.style.transform }],
				{
					duration: 160,
					easing: 'ease-out'
				}
			);
	}

	function release() {
		clearTimeout(held?.timer);
		held = null;
		removeEventListener('pointermove', move);
		removeEventListener('pointerup', up);
		removeEventListener('pointercancel', up);
	}

	function down(e: PointerEvent) {
		if (held || !e.isPrimary || (e.pointerType === 'mouse' && e.button !== 0)) return;
		if (e.target instanceof Element && e.target.closest('[data-sortable-ignore]')) return;
		const el = itemAt(e.target);
		const key = el?.dataset.sortableKey;
		if (!el || !key || opts.disabled?.(key)) return;
		const face = el.querySelector<HTMLElement>('[data-sortable-face]') ?? el;
		const r = face.getBoundingClientRect();
		const h: Held = {
			el,
			face,
			key,
			from: opts.keys.indexOf(key),
			pointer: e.pointerId,
			x0: e.clientX,
			y0: e.clientY,
			x: e.clientX,
			y: e.clientY,
			gx: e.clientX - r.left,
			gy: e.clientY - r.top,
			w: r.width,
			h: r.height,
			lifted: false,
			over: null
		};
		held = h;
		if (e.pointerType !== 'mouse') h.timer = setTimeout(() => lift(h, true), 250);
		addEventListener('pointermove', move);
		addEventListener('pointerup', up);
		addEventListener('pointercancel', up);
	}

	function move(e: PointerEvent) {
		const h = held;
		if (!h || e.pointerId !== h.pointer) return;
		h.x = e.clientX;
		h.y = e.clientY;
		const distance = Math.hypot(h.x - h.x0, h.y - h.y0);
		if (!h.lifted) {
			if (e.pointerType === 'mouse' && distance > 4) lift(h, false);
			else if (e.pointerType !== 'mouse' && distance > 8) release();
			if (!h.lifted) return;
		}
		h.liftAnim?.cancel();
		const over = itemAt(document.elementFromPoint(h.x, h.y))?.dataset.sortableKey ?? null;
		if (over !== h.over) {
			h.over = null;
			if (over && over !== h.key) {
				h.over = over;
				opts.onmove(opts.keys.indexOf(h.key), opts.keys.indexOf(over));
				// The slot has moved once the caller's list renders.
				void tick().then(() => held === h && place(h));
			}
		}
		place(h);
	}

	function up(e: PointerEvent) {
		const h = held;
		if (!h || e.pointerId !== h.pointer) return;
		release();
		if (!h.lifted) return;
		const { el, face } = h;
		const at = face.getBoundingClientRect();
		Object.assign(face.style, {
			transform: '',
			boxShadow: '',
			width: '',
			height: '',
			right: '',
			bottom: ''
		});
		el.style.pointerEvents = '';
		el.removeAttribute('data-lifted');
		el.removeAttribute('data-sortable-slot');
		// Glide from where it was let go into its slot, whatever the slot's size.
		const slot = face.getBoundingClientRect();
		const glide = prefersReducedMotion.current
			? face.animate([{ opacity: 0.2 }, { opacity: 1 }], { duration: 160 })
			: face.animate(
					[
						{
							transform: `translate(${at.left - slot.left}px, ${at.top - slot.top}px) scale(${at.width / slot.width}, ${at.height / slot.height})`,
							boxShadow: getComputedStyle(face).getPropertyValue('--shadow-lift')
						},
						{ transform: 'none', boxShadow: 'none' }
					],
					{ duration: 220, easing: EASE }
				);
		glide.finished
			.then(() => ring(el))
			.catch(() => {})
			.finally(() => {
				el.style.zIndex = '';
				face.style.transformOrigin = '';
			});
		// The pointerup's own click would open the item that was just put down.
		// Only that one: the next press or key starts a gesture of its own.
		if (e.type === 'pointerup') {
			const swallow = (click: Event) => {
				click.stopPropagation();
				click.preventDefault();
			};
			const disarm = () => {
				removeEventListener('click', swallow, { capture: true });
				removeEventListener('pointerdown', disarm, { capture: true });
				removeEventListener('keydown', disarm, { capture: true });
			};
			addEventListener('click', swallow, { capture: true, once: true });
			addEventListener('pointerdown', disarm, { capture: true, once: true });
			addEventListener('keydown', disarm, { capture: true, once: true });
			setTimeout(disarm, 100);
		}
		const to = opts.keys.indexOf(h.key);
		if (to !== h.from) moved(h.key, to);
	}

	function moved(key: string, to: number) {
		opts.ondrop?.(key);
		live.textContent = opts.label(key, to + 1);
	}

	function keydown(e: KeyboardEvent) {
		if (!e.altKey || held) return;
		if (e.target instanceof Element && e.target.closest('[data-sortable-ignore]')) return;
		const el = itemAt(e.target);
		const key = el?.dataset.sortableKey;
		if (!el || !key || opts.disabled?.(key)) return;
		const from = opts.keys.indexOf(key);
		const to = keyTarget(from, opts.keys.length, e.key, opts.columns());
		if (to === null) return;
		e.preventDefault();
		const focused = document.activeElement;
		opts.onmove(from, to);
		moved(key, to);
		// Moving an element in the DOM drops its focus.
		void tick().then(() => {
			if (focused instanceof HTMLElement && document.activeElement !== focused) focused.focus();
			ring(el, 220);
		});
	}

	// A lifted finger drags instead of scrolling the page.
	const touchmove = (e: TouchEvent) => {
		if (held?.lifted) e.preventDefault();
	};
	// A long press would open the image's menu, a mouse drag the image itself.
	const contextmenu = (e: Event) => {
		if (held) e.preventDefault();
	};
	const dragstart = (e: Event) => e.preventDefault();

	node.addEventListener('pointerdown', down);
	node.addEventListener('keydown', keydown);
	node.addEventListener('touchmove', touchmove, { passive: false });
	node.addEventListener('contextmenu', contextmenu);
	node.addEventListener('dragstart', dragstart);

	return {
		update(next) {
			opts = next;
		},
		destroy() {
			release();
			live.remove();
			node.removeEventListener('pointerdown', down);
			node.removeEventListener('keydown', keydown);
			node.removeEventListener('touchmove', touchmove);
			node.removeEventListener('contextmenu', contextmenu);
			node.removeEventListener('dragstart', dragstart);
		}
	};
};
