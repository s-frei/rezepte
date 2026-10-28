import { api } from './client';

/** A kind of notification mail; security notices are none and always go out. */
export type Kind = 'comment' | 'recipe' | 'invite_accepted';
export type Group = 'household' | 'admin';

/** Which group a kind belongs to, as the server groups them. */
export const KIND_GROUP: Record<Kind, Group> = {
	comment: 'household',
	recipe: 'household',
	invite_accepted: 'admin'
};

/** What an unsubscribe link reaches, and where it stands. */
export type Unsubscribe = {
	username: string;
	email: string;
	kind: Kind;
	group: Group;
	kindSubscribed: boolean;
	groupSubscribed: boolean;
	/** Every kind of the group the account may have, true when it gets that mail. */
	groupKinds: Partial<Record<Kind, boolean>>;
};

/** The link's signed parameters. */
export type UnsubscribeLink = { u: string; k: string; s: string };

/** The kinds the account may have, each on or off. */
export async function getNotificationPrefs(): Promise<Partial<Record<Kind, boolean>>> {
	return (await api<{ kinds: Partial<Record<Kind, boolean>> }>('/auth/me/notifications')).kinds;
}

/** Switches the kinds named; answers with all of them. */
export async function setNotificationPrefs(
	kinds: Partial<Record<Kind, boolean>>
): Promise<Partial<Record<Kind, boolean>>> {
	const body = JSON.stringify({ kinds });
	return (
		await api<{ kinds: Partial<Record<Kind, boolean>> }>('/auth/me/notifications', {
			method: 'PUT',
			body
		})
	).kinds;
}

/** Reads an unsubscribe link without changing anything; 410 when it no longer works. */
export function getUnsubscribe(link: UnsubscribeLink): Promise<Unsubscribe> {
	return api<Unsubscribe>(`/unsubscribe?${new URLSearchParams(link)}`);
}

/**
 * Unsubscribes the link's kind or its whole group; `subscribed: true` undoes it.
 * With the group scope, `kinds` limits it to those kinds of the group.
 */
export function postUnsubscribe(
	body: UnsubscribeLink & { scope: 'kind' | 'group'; subscribed: boolean; kinds?: Kind[] }
): Promise<Unsubscribe> {
	return api<Unsubscribe>('/unsubscribe', { method: 'POST', body: JSON.stringify(body) });
}
