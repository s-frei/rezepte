// The four sections of the docs, as the search dialog's filter tags. A page's
// section follows from its address; the content folders (guide), (hosting),
// api and (releases) are the same split, but leave no trace in the URL.
export const SECTIONS = [
	{ value: 'guide', name: 'Guide', prefixes: ['/overview', '/guide', '/contributing'] },
	{ value: 'hosting', name: 'Hosting', prefixes: ['/getting-started', '/operations'] },
	{ value: 'api', name: 'API', prefixes: ['/api'] },
	{ value: 'changelog', name: 'Changelog', prefixes: ['/changelog', '/roadmap'] }
] as const;

/** The section a page belongs to, or undefined outside the docs (the landing page). */
export function sectionOf(url: string): string | undefined {
	return SECTIONS.find((s) => s.prefixes.some((p) => url === p || url.startsWith(`${p}/`)))?.value;
}
