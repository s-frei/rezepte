const NOT_TYPED = new Set([
	'button',
	'checkbox',
	'color',
	'file',
	'radio',
	'range',
	'reset',
	'submit'
]);

/**
 * A field outside a dialog that brings up a phone's software keyboard, which
 * the shell's bottom chrome (the nav, the timer player) would otherwise sit
 * on top of or cover the field with.
 */
export function isField(target: EventTarget | null): boolean {
	if (!(target instanceof HTMLElement) || target.closest('[role="dialog"]')) return false;
	if (target instanceof HTMLTextAreaElement || target.isContentEditable) return true;
	return target instanceof HTMLInputElement && !NOT_TYPED.has(target.type);
}
