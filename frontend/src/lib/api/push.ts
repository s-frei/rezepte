import { api } from './client';

/** The VAPID public key, base64url. */
export async function getPushKey(): Promise<string> {
	return (await api<{ key: string }>('/push/key')).key;
}

export async function putSubscription(s: {
	endpoint: string;
	p256dh: string;
	auth: string;
}): Promise<void> {
	await api('/push/subscription', { method: 'PUT', body: JSON.stringify(s) });
}

export async function deleteSubscription(endpoint: string): Promise<void> {
	await api('/push/subscription', { method: 'DELETE', body: JSON.stringify({ endpoint }) });
}

/** Schedules a push; resolves with the alarm's id, which the push carries as its tag. */
export async function postAlarm(a: {
	endpoint: string;
	fireAt: string;
	title: string;
	body: string;
	url: string;
}): Promise<string> {
	return (await api<{ id: string }>('/push/alarms', { method: 'POST', body: JSON.stringify(a) }))
		.id;
}

export async function deleteAlarm(id: string): Promise<void> {
	await api(`/push/alarms/${encodeURIComponent(id)}`, { method: 'DELETE' });
}
