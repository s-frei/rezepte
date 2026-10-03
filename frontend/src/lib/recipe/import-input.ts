export type ImportInput =
	{ kind: 'empty' } | { kind: 'link'; url: string; host: string } | { kind: 'text'; text: string };

/** Link or text: one http(s) token is a link, anything else is recipe text. */
export function detectInput(raw: string): ImportInput {
	const text = raw.trim();
	if (text === '') return { kind: 'empty' };
	if (!/\s/.test(text)) {
		try {
			const url = new URL(text);
			if (url.protocol === 'http:' || url.protocol === 'https:') {
				return { kind: 'link', url: url.href, host: url.hostname.replace(/^www\./, '') };
			}
		} catch {
			// not a URL: falls through to text
		}
	}
	return { kind: 'text', text };
}
