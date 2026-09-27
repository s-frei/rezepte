import { api } from './client';

/** How many minutes a link preview lasts: 15 minutes, an hour or a day. */
export type PreviewLifetime = 15 | 60 | 1440;

/** The household-wide settings the instance owner controls. */
export type Settings = {
	recipesLockedByDefault: boolean;
	/** Whether a recipe link shared from the app shows the recipe in chat previews. */
	linkPreviews: boolean;
	/** How long such a link shows the recipe. */
	linkPreviewMinutes: PreviewLifetime;
};

export function getSettings(): Promise<Settings> {
	return api<Settings>('/settings');
}

/** Owner only; everybody else gets a 403. Changes only the fields named. */
export function updateSettings(patch: Partial<Settings>): Promise<Settings> {
	return api<Settings>('/settings', {
		method: 'PATCH',
		body: JSON.stringify(patch)
	});
}

/** Owner only; everybody else gets a 403. */
export function setRecipesLockedByDefault(on: boolean): Promise<Settings> {
	return updateSettings({ recipesLockedByDefault: on });
}
