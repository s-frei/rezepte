import { api } from './client';

export type MailSecurity = 'starttls' | 'tls' | 'none';

/** The mail configuration as the owner sees it; the password itself never comes back. */
export type MailSettings = {
	/** env: REZEPTE_SMTP_* pin it, read-only here; settings: configured here; none: off. */
	source: 'env' | 'settings' | 'none';
	host: string;
	port: number;
	security: MailSecurity;
	username: string;
	passwordSet: boolean;
	from: string;
	fromName: string;
	publicUrlMissing: boolean;
};

/** What the edit dialog sends; an omitted password keeps the stored one. */
export type MailConfigInput = {
	host: string;
	port: number;
	security: MailSecurity;
	username?: string;
	password?: string;
	from: string;
	fromName?: string;
};

export function getMailSettings(): Promise<MailSettings> {
	return api<MailSettings>('/settings/mail');
}

export function saveMailSettings(input: MailConfigInput): Promise<MailSettings> {
	return api<MailSettings>('/settings/mail', { method: 'PUT', body: JSON.stringify(input) });
}

export function clearMailSettings(): Promise<void> {
	return api<void>('/settings/mail', { method: 'DELETE' });
}

/** Tests `config` when given (unsaved dialog values), else the configuration in force. A 502 carries the server's reply as its detail. */
export function sendTestMail(to: string, config?: MailConfigInput): Promise<void> {
	return api<void>('/settings/mail/test', { method: 'POST', body: JSON.stringify({ to, config }) });
}

/** One mail in the owner's outbox: kind, recipient, subject and how it went, never the content. */
export type SentMail = {
	id: number;
	kind: string;
	to: string;
	subject: string;
	/** queued: waiting for the mail server or for a retry. */
	status: 'queued' | 'sent' | 'failed';
	attempts: number;
	/** The last failed attempt's reason, as the mail server said it; also on a mail a retry got through. */
	error?: string;
	createdAt: string;
	finishedAt?: string;
	/** When the next attempt is due, while a retry waits. */
	nextAttemptAt?: string;
	/** The accounts that hold the address now: none for a stranger's, several when accounts share it. */
	people: MailPerson[];
};

/** An account a mail's address belongs to. */
export type MailPerson = {
	id: string;
	username: string;
	displayName: string;
	color: string;
	avatarId?: string;
};

/** One page of the outbox, newest first, and the counts over all of it. */
export type Outbox = {
	items: SentMail[];
	more: boolean;
	total: number;
	failed: number;
	queued: number;
};

/** The page of mails older than `before` (absent: the newest), only those in `status` when given. */
export function getOutbox(
	options: { before?: number; status?: SentMail['status'] } = {}
): Promise<Outbox> {
	const query = new URLSearchParams();
	if (options.before) query.set('before', String(options.before));
	if (options.status) query.set('status', options.status);
	const qs = query.size ? `?${query}` : '';
	return api<Outbox>(`/settings/mail/history${qs}`);
}
