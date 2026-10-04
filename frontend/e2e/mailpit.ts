import { expect, type APIRequestContext } from '@playwright/test';

export const mailpitUrl = process.env.MAILPIT_URL ?? '';
export const mailpitSmtpPort = Number(process.env.MAILPIT_SMTP_PORT ?? 0);

type Found = { messages: { ID: string; Subject: string }[] };

/** Waits for the newest mail to address and returns its parts. */
export async function latestMailTo(request: APIRequestContext, address: string) {
	let id = '';
	let subject = '';
	await expect
		.poll(
			async () => {
				const res = await request.get(`${mailpitUrl}/api/v1/search`, {
					params: { query: `to:"${address}"` }
				});
				const found = (await res.json()) as Found;
				id = found.messages[0]?.ID ?? '';
				subject = found.messages[0]?.Subject ?? '';
				return id;
			},
			{ timeout: 10_000 }
		)
		.not.toBe('');
	const msg = await (await request.get(`${mailpitUrl}/api/v1/message/${id}`)).json();
	return { html: msg.HTML as string, text: msg.Text as string, subject };
}

/** How many mails Mailpit holds for address, right now. */
export async function mailCountTo(request: APIRequestContext, address: string) {
	const res = await request.get(`${mailpitUrl}/api/v1/search`, {
		params: { query: `to:"${address}"` }
	});
	return ((await res.json()) as Found).messages.length;
}

/** The link to path (with its #token) in a mail's plain-text part. */
export function linkIn(text: string, path: string): string {
	const match = text.match(new RegExp(`https?://\\S+${path}#\\S+`));
	if (!match) throw new Error(`no ${path} link in: ${text}`);
	return match[0];
}

/** The setup link in a mail's plain-text part. */
export function setupLinkIn(text: string): string {
	return linkIn(text, '/welcome');
}
