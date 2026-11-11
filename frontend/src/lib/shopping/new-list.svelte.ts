import { goto } from '$app/navigation';
import { resolve } from '$app/paths';
import { page } from '$app/state';
import { holdKeyboard } from '#lib/keyboard.js';

/** Whether the import of a new list is open; set from the top bar on the
 * lists and the phone's New sheet there, as `importDialog` is for recipes. */
export const listImport = $state({ open: false });

/** Whether the draft of a new list is the page already. */
export const onDraft = () => page.url.pathname === resolve('/lists/new');

/** Opens the draft of a new list, a page with its name field at the top.
 * The keyboard is raised inside the tap, which iOS insists on. */
export function startList(): void {
	holdKeyboard();
	void goto(resolve('/lists/new'));
}
