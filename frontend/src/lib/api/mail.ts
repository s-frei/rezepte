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
