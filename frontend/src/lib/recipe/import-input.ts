import type { ReadonlyURLSearchParams } from '$app/state';

export type ImportInput =
	{ kind: 'empty' } | { kind: 'link'; url: string; host: string } | { kind: 'text'; text: string };

/** The address when `text` parses as one with http: or https:, else null. */
function httpUrl(text: string): URL | null {
	try {
		const url = new URL(text);
		return url.protocol === 'http:' || url.protocol === 'https:' ? url : null;
	} catch {
		return null;
	}
}

/** Link or text: one http(s) token is a link, anything else is recipe text. */
export function detectInput(raw: string): ImportInput {
	const text = raw.trim();
	if (text === '') return { kind: 'empty' };
	if (!/\s/.test(text)) {
		const url = httpUrl(text);
		if (url) return { kind: 'link', url: url.href, host: url.hostname.replace(/^www\./, '') };
	}
	return { kind: 'text', text };
}

/** What the Web Share Target handed over, as the import field's content:
 * a shared link, else a link inside the text, else the text. The title is
 * only a label and never a recipe. */
export function sharedInput(params: ReadonlyURLSearchParams): string {
	const url = httpUrl((params.get('url') ?? '').trim());
	if (url) return url.href;
	const text = (params.get('text') ?? '').trim();
	const inText = text.match(/https?:\/\/\S+/)?.[0]?.replace(/[.,;:!?)\]}>'"»]+$/, '');
	return (inText && httpUrl(inText)?.href) || text;
}
