// Reorders a list's `[data-sortable-key]` children by pointer and keyboard.
// A mouse drags after 4px; a finger lifts the item after a 250ms press (a
// move of more than 8px before that is a scroll and lets go). While held,
// the item stays in the list as the landing slot and its face follows the
// pointer at the size it was picked up at, or with `faceFollowsSlot` at the
// size of its slot, easing from one to the next; the slot under the pointer is
// where it goes: each new slot calls `onmove`, and everything else slides
// there (`flip`). Held within `EDGE` of the window's top or bottom, the page
// scrolls, faster the closer it gets. Alt+Arrow on a focused item moves it one
// place: Left and Up earlier, Right and Down later.
//
// The CSS contract a component using it adds:
// - on every item: `user-select: none; -webkit-touch-callout: none;
//   -webkit-tap-highlight-color: transparent; touch-action: pan-y` - without
//   the touch-action the browser's pan cancels the press before the
//   long-press lifts it;
// - on every `<img>` in an item: `draggable="false"` and
//   `-webkit-user-drag: none`, or the browser's own long-press drags the image;
// - `[data-sortable-face]` inside the item, `position: absolute; inset: 0`
//   (the item is its containing block): it is what follows the pointer, with
//   its picked-up width and height set inline (left alone with
//   `faceFollowsSlot`). Every item needs one: the held item stays put as the
//   slot the pointer's target is measured from (`restingAt`). Clipping (`overflow: hidden`) and rounding
//   go on the face, never on the item: the item would cut off the lifted face;
// - `[data-sortable-ignore]` on a control inside an item that has a press of
//   its own, such as a menu button or a ×: a press there never picks it up;
// - `[data-sortable-slot]` on the held item: style it as the landing slot, the
//   dashed `primary` gap;
// - `[data-resizing]` on an item while `flip` changes its size, and on the
//   held item while its face eases to a new slot's size: hide what would
//   stretch with it, such as a ×.
// `[data-lifted]` marks the held item from the lift to the drop.
//
// From the press to the release the list prevents `selectstart` and
// `contextmenu`: the lifted item lets pointers through, so Chrome's own
// long-press (~650ms) lands on the list behind it, which is selectable.
import type { Action } from 'svelte/action';
import { prefersReducedMotion } from 'svelte/motion';
import { EASE, SLIDE, flip, ring } from './flip.js';
import { keyTarget } from './order.js';

export type SortableOptions = {
	/** The items' keys in their current order, as their `data-sortable-key`. */
	keys: string[];
	/** An item that cannot be picked up or moved by key. */
	disabled?: (key: string) => boolean;
	/** Move the item at `from` to `to`: the caller updates its list, at once. */
	onmove: (from: number, to: number) => void;
	/** A drag or key press has put `key` in a new place: save the order. */
	ondrop?: (key: string) => void;
	/** What the live region announces after a move; `position` counts from 1. */
	label: (key: string, position: number) => string;
	/** The held face takes the size of the slot it is over, so it shows how it lands. */
	faceFollowsSlot?: boolean;
};

const LIFT = 1.06;
/** How close to the window's top or bottom a held item scrolls the page, in px. */
const EDGE = 64;

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
	/** The grab point inside the face, as a share of its size. */
	fx: number;
	fy: number;
	/** The face's size before the lift scale: as picked up, or easing to `slotW` x `slotH`. */
	w: number;
	h: number;
	slotW: number;
	slotH: number;
	resizing?: number;
	scrolling?: number;
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

	/**
	 * The item whose resting place is under the pointer. Not the box the
	 * browser hit-tests: a neighbor sliding past (`flip`) would count, and the
	 * two would trade back and forth. Offsets ignore transforms; the held slot,
	 * which never slides, turns them into screen coordinates.
	 */
	function restingAt(h: Held) {
		// Whatever covers the list, such as a sticky bar, is not the list.
		if (!node.contains(document.elementFromPoint(h.x, h.y))) return null;
		const slot = h.el.getBoundingClientRect();
		for (const el of node.querySelectorAll<HTMLElement>('[data-sortable-key]')) {
			const left = slot.left + el.offsetLeft - h.el.offsetLeft;
			const top = slot.top + el.offsetTop - h.el.offsetTop;
			if (h.x >= left && h.x < left + el.offsetWidth && h.y >= top && h.y < top + el.offsetHeight)
				return el.dataset.sortableKey ?? null;
		}
		return null;
	}

	/** Puts the held face under the pointer at its size `w` x `h`, from its resting box. */
	function place(h: Held) {
		h.face.style.transform = '';
		const slot = h.face.getBoundingClientRect();
		const s = scale();
		h.face.style.transform =
			`translate(${h.x - h.fx * h.w * s - slot.left}px, ${h.y - h.fy * h.h * s - slot.top}px) ` +
			`scale(${(h.w * s) / slot.width}, ${(h.h * s) / slot.height})`;
	}

	/** With `faceFollowsSlot`: eases the face from its size now to its new slot's, around the pointer. */
	function follow(h: Held) {
		const slot = h.el.getBoundingClientRect();
		if (Math.abs(slot.width - h.slotW) < 0.5 && Math.abs(slot.height - h.slotH) < 0.5) return;
		[h.slotW, h.slotH] = [slot.width, slot.height];
		cancelAnimationFrame(h.resizing ?? 0);
		if (prefersReducedMotion.current) {
			[h.w, h.h] = [h.slotW, h.slotH];
			place(h);
			return;
		}
		h.el.setAttribute('data-resizing', '');
		const [w0, h0, t0] = [h.w, h.h, performance.now()];
		const step = (now: number) => {
			if (held !== h) return;
			const t = Math.min(1, (now - t0) / SLIDE);
			const eased = 1 - (1 - t) ** 3;
			h.w = w0 + (h.slotW - w0) * eased;
			h.h = h0 + (h.slotH - h0) * eased;
			place(h);
			if (t < 1) h.resizing = requestAnimationFrame(step);
			else h.el.removeAttribute('data-resizing');
		};
		h.resizing = requestAnimationFrame(step);
	}

	function lift(h: Held, touch: boolean) {
		h.lifted = true;
		getSelection()?.removeAllRanges();
		h.el.setAttribute('data-lifted', '');
		Object.assign(h.el.style, { zIndex: '10', pointerEvents: 'none' });
		Object.assign(h.face.style, { transformOrigin: '0 0', boxShadow: 'var(--shadow-lift)' });
		if (h.face !== h.el) h.el.setAttribute('data-sortable-slot', '');
		if (h.face !== h.el && !opts.faceFollowsSlot) {
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
		cancelAnimationFrame(held?.resizing ?? 0);
		cancelAnimationFrame(held?.scrolling ?? 0);
		held?.el.removeAttribute('data-resizing');
		held = null;
		removeEventListener('keydown', keysWhileHeld, true);
		removeEventListener('pointermove', move);
		removeEventListener('scroll', scroll);
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
		// An item still sliding or gliding into place is measured where it rests;
		// the glide's cleanup runs now, not in the middle of this drag.
		for (const a of el.getAnimations({ subtree: true })) a.cancel();
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
			fx: (e.clientX - r.left) / r.width,
			fy: (e.clientY - r.top) / r.height,
			w: r.width,
			h: r.height,
			slotW: r.width,
			slotH: r.height,
			lifted: false,
			over: null
		};
		held = h;
		if (e.pointerType !== 'mouse') h.timer = setTimeout(() => lift(h, true), 250);
		addEventListener('keydown', keysWhileHeld, true);
		addEventListener('pointermove', move);
		addEventListener('pointerup', up);
		addEventListener('pointercancel', up);
		addEventListener('scroll', scroll, { passive: true });
	}

	// The page scrolling under a held item moves its slot and what is under
	// the pointer, not the pointer.
	function scroll() {
		if (!held?.lifted) return;
		track(held);
		place(held);
	}

	// Scrolls the page while the pointer stays near the window's top or bottom.
	function autoscroll(h: Held) {
		cancelAnimationFrame(h.scrolling ?? 0);
		const step = () => {
			if (held !== h) return;
			const v = h.y < EDGE ? h.y - EDGE : h.y > innerHeight - EDGE ? h.y - innerHeight + EDGE : 0;
			if (!v) return;
			scrollBy(0, Math.round(v / 5));
			h.scrolling = requestAnimationFrame(step);
		};
		h.scrolling = requestAnimationFrame(step);
	}

	// Alt+Arrow mid-drag would reach the browser (Alt+Left is Back) with focus anywhere.
	function keysWhileHeld(e: KeyboardEvent) {
		if (e.altKey && e.key.startsWith('Arrow')) e.preventDefault();
	}

	/** Moves the held item to the slot under the pointer, once per slot. */
	function track(h: Held) {
		const over = restingAt(h);
		if (over === h.over) return;
		h.over = null;
		if (!over || over === h.key) return;
		h.over = over;
		const [from, to] = [opts.keys.indexOf(h.key), opts.keys.indexOf(over)];
		// The face follows its slot in the render that moves it.
		void flip(node, () => opts.onmove(from, to), {
			after: () => {
				if (held !== h) return;
				// First, before anything measures: the face at its old slot's place could hang past the edge.
				place(h);
				if (opts.faceFollowsSlot && h.face !== h.el) follow(h);
			}
		});
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
		track(h);
		place(h);
		autoscroll(h);
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
					{ duration: SLIDE, easing: EASE }
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
		if (!el || !key) return;
		const from = opts.keys.indexOf(key);
		const to = keyTarget(from, opts.keys.length, e.key);
		if (to === null) return;
		// Even when nothing moves: Alt+Left is the browser's Back.
		e.preventDefault();
		if (to === from || opts.disabled?.(key)) return;
		const focused = document.activeElement;
		void flip(node, () => opts.onmove(from, to)).then(() => {
			// Moving an element in the DOM drops its focus.
			if (focused instanceof HTMLElement && document.activeElement !== focused) focused.focus();
			// Where the caller put it, which may be short of `to`, or nowhere.
			const at = opts.keys.indexOf(key);
			if (at === from) return;
			moved(key, at);
			ring(el, SLIDE);
		});
	}

	// A lifted finger drags instead of scrolling the page.
	const touchmove = (e: TouchEvent) => {
		if (held?.lifted) e.preventDefault();
	};
	// A long press would open the image's menu or select the word nearest to
	// it, on the list itself once the lifted item lets the press through; a
	// mouse drag would drag the image itself.
	const whileHeld = (e: Event) => {
		if (held) e.preventDefault();
	};
	const dragstart = (e: Event) => e.preventDefault();

	node.addEventListener('pointerdown', down);
	node.addEventListener('keydown', keydown);
	node.addEventListener('touchmove', touchmove, { passive: false });
	node.addEventListener('contextmenu', whileHeld);
	node.addEventListener('selectstart', whileHeld);
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
			node.removeEventListener('contextmenu', whileHeld);
			node.removeEventListener('selectstart', whileHeld);
			node.removeEventListener('dragstart', dragstart);
		}
	};
};
