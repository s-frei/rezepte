import type { UpdateStatus } from './api/settings';

/** Holds the version whose note the owner hid; a newer release shows it again. */
export const UPDATE_DISMISSED_KEY = 'rezepte-update-dismissed';

export function noteVisible(status: UpdateStatus | undefined, dismissed: string | null): boolean {
	return status?.state === 'available' && status.latest?.version !== dismissed;
}
