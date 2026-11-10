/**
 * Keeps the on-screen keyboard up across a page change. iOS raises it only
 * for a focus inside the tap itself, and a field on the next page is focused
 * after the navigation, outside it. So the tap focuses a hidden stand-in
 * (`holdKeyboard`), and the next page moves the focus on to its own field
 * (`handKeyboardTo`); a focus that moves between fields keeps the keyboard.
 */
let held: HTMLInputElement | null = null;

export function holdKeyboard() {
	held?.remove();
	held = document.createElement('input');
	held.tabIndex = -1;
	held.setAttribute('aria-hidden', 'true');
	// 16px: a smaller field makes iOS zoom the page in.
	held.style.cssText =
		'position:fixed;top:0;left:0;width:1px;height:1px;opacity:0;font-size:16px;pointer-events:none';
	document.body.append(held);
	held.focus({ preventScroll: true });
}

export function handKeyboardTo(field: HTMLElement) {
	field.focus();
	held?.remove();
	held = null;
}
