import { api } from './client';

/** A kind of notification; security notices are none and always go out by mail. */
export type Kind = 'comment' | 'recipe' | 'tasty' | 'invite_accepted' | 'mail_failing';
export type Group = 'household' | 'admin';
/** A way a notification reaches the account. */
export type Channel = 'mail' | 'push';

/** Which group a kind belongs to, as the server groups them. */
export const KIND_GROUP: Record<Kind, Group> = {
	comment: 'household',
	recipe: 'household',
	tasty: 'household',
	invite_accepted: 'admin',
	mail_failing: 'admin'
};

/**
 * The account's preferences: each kind's own choice and each group's master
 * switch, per channel. A kind goes out on a channel only while both are on;
 * a pair the account may not have is absent.
 */
export type Prefs = {
	kinds: Partial<Record<Kind, Partial<Record<Channel, boolean>>>>;
	groups: Partial<Record<Group, Partial<Record<Channel, boolean>>>>;
};

/** What an unsubscribe link reaches, and where it stands. */
export type Unsubscribe = {
	username: string;
	email: string;
	kind: Exclude<Kind, 'mail_failing'>;
	group: Group;
	/** Whether the account gets the link's kind by mail. */
	kindSubscribed: boolean;
	/** Whether the group's mail switch is on. */
	groupSubscribed: boolean;
	/** Every kind of the group the account may have by mail, true when it gets that mail. */
	groupKinds: Partial<Record<Kind, boolean>>;
};

/** The link's signed parameters. */
export type UnsubscribeLink = { u: string; k: string; s: string };

/** Every kind and group the account may have, per channel. */
export function getNotificationPrefs(): Promise<Prefs> {
	return api<Prefs>('/auth/me/notifications');
}

/** Switches the pairs named; answers with all of them. */
export function setNotificationPrefs(patch: Prefs): Promise<Prefs> {
	return api<Prefs>('/auth/me/notifications', { method: 'PUT', body: JSON.stringify(patch) });
}

/** Reads an unsubscribe link without changing anything; 410 when it no longer works. */
export function getUnsubscribe(link: UnsubscribeLink): Promise<Unsubscribe> {
	return api<Unsubscribe>(`/unsubscribe?${new URLSearchParams(link)}`);
}

/**
 * Switches the mail of the link's kind, or its group's mail switch, off;
 * `subscribed: true` undoes exactly that.
 */
export function postUnsubscribe(
	body: UnsubscribeLink & { scope: 'kind' | 'group'; subscribed: boolean }
): Promise<Unsubscribe> {
	return api<Unsubscribe>('/unsubscribe', { method: 'POST', body: JSON.stringify(body) });
}
