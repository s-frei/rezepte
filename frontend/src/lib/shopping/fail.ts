import { toast } from 'svelte-sonner';
import { goto } from '$app/navigation';
import { resolve } from '$app/paths';
import { ApiError, isSignedOut } from '#lib/api/client.js';
import { m } from '#lib/paraglide/messages.js';

/**
 * A failed change on a list page: a toast (`message`, or the generic one),
 * and back to /lists when the list is gone (deleted, or the owner removed me:
 * the server answers 404).
 */
export function failList(error: unknown, message = m.list_action_error()) {
	if (isSignedOut(error)) return;
	if (error instanceof ApiError && error.status === 404) {
		toast.error(m.list_gone());
		void goto(resolve('/lists'));
		return;
	}
	toast.error(message);
}
