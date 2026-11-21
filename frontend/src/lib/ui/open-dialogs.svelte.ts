import { untrack } from 'svelte';

/**
 * How many of the app's dialogs and sheets (`BaseDialog`, `BottomSheet`,
 * the filter panel) are open. The layout reads it to show toasts at the top
 * of a phone while one is, so a toast never covers a sheet's actions.
 */
let count = $state(0);

export const openDialogs = {
	get any(): boolean {
		return count > 0;
	}
};

/** Counts the calling dialog while `isOpen()` holds; call it during init. */
export function countWhileOpen(isOpen: () => boolean) {
	$effect(() => {
		if (!isOpen()) return;
		// Untracked: reading `count` here would rerun the effect on its own write.
		untrack(() => count++);
		return () => untrack(() => count--);
	});
}

/**
 * A toast's action (Undo) acts on what the dialog shows: an outside press
 * that lands in the toaster keeps the dialog open. Wherever the toasts sit,
 * over the sheet or at the top, the toaster is outside it.
 */
export function keepForToasts(event: Event) {
	if (event.target instanceof Element && event.target.closest('[data-sonner-toaster]'))
		event.preventDefault();
}
