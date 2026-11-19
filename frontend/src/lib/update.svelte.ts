import { onSignIn, session } from '#lib/auth.svelte.js';
import { getUpdateStatus, type UpdateStatus } from './api/settings';
import { load, save } from './storage';
import { UPDATE_DISMISSED_KEY } from './update';

/**
 * The owner's update status and the version whose note they hid. `dismissed`
 * is kept here as well as in storage, so hiding works where storage throws.
 * A failed fetch leaves `status` undefined, rendered like `unknown`.
 */
export const update = $state({
	status: undefined as UpdateStatus | undefined,
	dismissed: load(UPDATE_DISMISSED_KEY)
});

let loaded = false;

export async function loadUpdateStatus(): Promise<void> {
	if (loaded || session.user?.role !== 'superadmin') return;
	loaded = true;
	const id = session.user.id;
	try {
		const status = await getUpdateStatus();
		// A switch while this was on its way loads for the new account.
		if (session.user?.id === id) update.status = status;
	} catch {
		// After a switch, the new account's load owns the flag.
		if (session.user?.id === id) loaded = false;
	}
}

// Another account on this page: an owner loads afresh, a member never asks.
onSignIn(() => {
	update.status = undefined;
	loaded = false;
	void loadUpdateStatus();
});

export function dismissUpdate(): void {
	const version = update.status?.latest?.version;
	if (!version) return;
	update.dismissed = version;
	save(UPDATE_DISMISSED_KEY, version);
}
