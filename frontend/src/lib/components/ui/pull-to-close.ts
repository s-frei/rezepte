import type { Attachment } from 'svelte/attachments';
import { prefersReducedMotion } from 'svelte/motion';

/** Pixels a finger may wander before a press counts as a pull, not a tap. */
const SLOP = 4;
/** Downward speed, in px/ms, that closes the sheet however short the pull. */
const FLICK = 0.5;

/**
 * A bottom sheet's grip: pulling it down drags the sheet (the grip's
 * `[role="dialog"]` ancestor) with the finger, and `onclose` runs when it is
 * let go a quarter of the sheet's height down or flicked; short of that the
 * sheet springs back. The click that ends a pull is swallowed, so only a tap
 * reaches the grip's own click. The sheet keeps the transform it was let go
 * at, for its outro to carry on from.
 */
export function pullToClose(onclose: () => void): Attachment<HTMLElement> {
	return (handle) => {
		let sheet: HTMLElement | null = null;
		let startY = 0;
		let lastY = 0;
		let lastTime = 0;
		let speed = 0;
		let pulled = false;

		function down(event: PointerEvent) {
			if (event.button !== 0) return;
			sheet = handle.closest('[role="dialog"]');
			if (!sheet) return;
			handle.setPointerCapture(event.pointerId);
			startY = lastY = event.clientY;
			lastTime = event.timeStamp;
			speed = 0;
			pulled = false;
			sheet.style.transition = 'none';
		}

		function move(event: PointerEvent) {
			if (!sheet) return;
			const dy = Math.max(0, event.clientY - startY);
			if (dy > SLOP) pulled = true;
			const dt = event.timeStamp - lastTime;
			if (dt > 0) speed = (event.clientY - lastY) / dt;
			lastY = event.clientY;
			lastTime = event.timeStamp;
			sheet.style.transform = `translateY(${dy}px)`;
		}

		function up() {
			if (!sheet) return;
			const node = sheet;
			sheet = null;
			// The click that follows a pull comes before this; a later one is a tap.
			setTimeout(() => (pulled = false));
			const dy = Math.max(0, lastY - startY);
			if (pulled && (dy > node.offsetHeight / 4 || speed > FLICK)) {
				node.style.transition = '';
				onclose();
				return;
			}
			node.style.transition = prefersReducedMotion.current ? '' : 'transform 200ms ease-out';
			node.style.transform = '';
			node.addEventListener('transitionend', () => (node.style.transition = ''), { once: true });
		}

		// A pull ends in a click on the grip; only a tap may close through it.
		function click(event: MouseEvent) {
			if (!pulled) return;
			event.preventDefault();
			event.stopImmediatePropagation();
		}

		handle.addEventListener('pointerdown', down);
		handle.addEventListener('pointermove', move);
		handle.addEventListener('pointerup', up);
		handle.addEventListener('pointercancel', up);
		handle.addEventListener('click', click, { capture: true });
		return () => {
			handle.removeEventListener('pointerdown', down);
			handle.removeEventListener('pointermove', move);
			handle.removeEventListener('pointerup', up);
			handle.removeEventListener('pointercancel', up);
			handle.removeEventListener('click', click, { capture: true });
		};
	};
}
