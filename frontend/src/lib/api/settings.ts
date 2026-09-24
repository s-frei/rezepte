import { api } from './client';

/** The household-wide settings the instance owner controls. */
export type Settings = { recipesLockedByDefault: boolean };

export function getSettings(): Promise<Settings> {
	return api<Settings>('/settings');
}

/** Owner only; everybody else gets a 403. */
export function setRecipesLockedByDefault(on: boolean): Promise<Settings> {
	return api<Settings>('/settings', {
		method: 'PATCH',
		body: JSON.stringify({ recipesLockedByDefault: on })
	});
}
