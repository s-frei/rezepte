import { MediaQuery } from 'svelte/reactivity';

/** Px insets of the visible area (above the software keyboard) inside the fixed box. */
export type VisibleArea = { top: number; bottom: number };

/**
 * No software keyboard without a focused text field, so `!typing` reads as the
 * full box; this also guards the stuck visual viewport WebKit leaves behind
 * after a keyboard closes (bugs 297779, 300523, 218983).
 */
export function measureVisibleArea(
	probe: { top: number; bottom: number },
	viewport: { offsetTop: number; height: number },
	typing: boolean
): VisibleArea {
	if (!typing) return { top: 0, bottom: 0 };
	return {
		top: Math.max(0, Math.round(viewport.offsetTop - probe.top)),
		bottom: Math.max(0, Math.round(probe.bottom - viewport.offsetTop - viewport.height))
	};
}

const NOT_TYPED = new Set([
	'button',
	'checkbox',
	'color',
	'file',
	'image',
	'radio',
	'range',
	'reset',
	'submit'
]);

// Unlike the shell's `isField`, a field inside a dialog counts: the palette
// and the login sheet are dialogs.
export function isTextField(el: Element | null): el is HTMLElement {
	if (el instanceof HTMLTextAreaElement) return true;
	if (el instanceof HTMLInputElement) return !NOT_TYPED.has(el.type);
	return el instanceof HTMLElement && el.isContentEditable;
}

/**
 * Reports the visible area now and whenever it moves, so a `fixed` box can
 * follow the keyboard where it shrinks the visual viewport, not the layout one
 * the box measures from: iOS. Chromium shrinks the layout itself
 * (`interactive-widget=resizes-content` in app.html), and the insets read 0.
 * Where the box's edges are is measured, not assumed, with `probe` (a
 * zero-width fixed line from the box's top to its bottom, no transition):
 * Chrome for Android anchors fixed elements to a box that differs from
 * `innerHeight`. Returns the cleanup.
 */
export function followVisibleArea(
	probe: HTMLElement,
	set: (area: VisibleArea) => void
): () => void {
	const viewport = window.visualViewport;
	if (!viewport) return () => {};
	const measure = () =>
		set(
			measureVisibleArea(
				probe.getBoundingClientRect(),
				viewport,
				isTextField(document.activeElement)
			)
		);
	let timer: ReturnType<typeof setTimeout> | undefined;
	// The viewport settles after the keyboard closes, not at the blur.
	const afterFocusOut = () => {
		measure();
		clearTimeout(timer);
		timer = setTimeout(measure, 150);
	};
	measure();
	viewport.addEventListener('resize', measure);
	viewport.addEventListener('scroll', measure);
	// The toolbar moving resizes the window without always resizing the visual viewport.
	window.addEventListener('resize', measure);
	document.addEventListener('focusin', measure);
	document.addEventListener('focusout', afterFocusOut);
	return () => {
		viewport.removeEventListener('resize', measure);
		viewport.removeEventListener('scroll', measure);
		window.removeEventListener('resize', measure);
		document.removeEventListener('focusin', measure);
		document.removeEventListener('focusout', afterFocusOut);
		clearTimeout(timer);
	};
}

/** Rows of a scrolling list, measured from the top of its content. */
export type ListRow = { top: number; height: number };

/**
 * Max height for a list that must end within `room` px: all of `room` when
 * everything fits; otherwise the last row that can show 48% of itself (past
 * the x-height, 43% of a 44px option, by about 2px) is cut at no more than
 * 55% (above the baseline, 59%): its letters show with their feet missing,
 * so the list reads as scrollable and never ends on a heading. Never under
 * `min` (two rows).
 */
export function fitListHeight(room: number, rows: ListRow[], min = 88): number {
	const last = rows.at(-1);
	if (!last || last.top + last.height <= room) return Math.max(min, Math.round(room));
	const row = rows.filter((r) => r.top + 0.48 * r.height <= room).at(-1);
	return Math.max(min, Math.round(row ? Math.min(room, row.top + 0.55 * row.height) : min));
}

/**
 * Keeps `node`, a scrolling list below a text field, 12px above the visible
 * area's bottom: the keyboard on iOS (visual viewport), the shrunk layout on
 * Android (`interactive-widget=resizes-content`). Without a keyboard (put
 * away, focus kept) that is 12px above the screen's bottom, which is free:
 * the dock hides while typing. Pass `cap` (px) where a list should stay
 * small anyway (TagInput: 12rem). Returns the cleanup, which also drops the
 * max-height it set.
 */
export function fitAboveKeyboard(node: HTMLElement, cap = Infinity): () => void {
	const viewport = window.visualViewport;
	if (!viewport) return () => {};
	const fit = () => {
		const box = node.getBoundingClientRect();
		// 21px: the frame's 8px bottom padding, its 1px border and the 12px gap.
		const room = Math.min(cap, viewport.offsetTop + viewport.height - box.top - 21);
		const top = box.top - node.scrollTop;
		const rows = [...node.querySelectorAll('[role=option]')].map((option) => {
			const r = option.getBoundingClientRect();
			return { top: r.top - top, height: r.height };
		});
		node.style.maxHeight = `${fitListHeight(room, rows)}px`;
	};
	fit();
	viewport.addEventListener('resize', fit);
	viewport.addEventListener('scroll', fit);
	window.addEventListener('scroll', fit);
	return () => {
		viewport.removeEventListener('resize', fit);
		viewport.removeEventListener('scroll', fit);
		window.removeEventListener('scroll', fit);
		// Past a phone the stylesheet's max-height applies again.
		node.style.maxHeight = '';
	};
}

let keyboardQuery: MediaQuery | undefined;

/**
 * Whether focusing a text field likely brings up a software keyboard that
 * covers the page: a phone's width, or a coarse pointer at any width. A
 * landscape phone is wider than `md`, and an iPad's keyboard covers half
 * its screen, so width alone sent both down the desktop path, a list
 * running on under the keyboard. A desktop with a mouse is left alone.
 * Created on first read, so importing it outside a browser costs nothing.
 */
export const softKeyboard = {
	get current(): boolean {
		keyboardQuery ??= new MediaQuery('(max-width: 767.98px), (pointer: coarse)');
		return keyboardQuery.current;
	}
};
