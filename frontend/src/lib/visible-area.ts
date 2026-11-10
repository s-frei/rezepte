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
